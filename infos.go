package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"mime"
	"mime/multipart"
	"net/mail"
	"regexp"
	"strings"
	"time"

	"github.com/abadojack/whatlanggo"
	"github.com/h2non/filetype"
	"github.com/jdkato/prose"
	"github.com/kljensen/snowball"
	"github.com/mvdan/xurls"
	"golang.org/x/net/html"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/htmlindex"
	"golang.org/x/text/encoding/unicode"
)

type BaseInfos struct {
	MailFrom string   `json:"mail_from,omitempty"`
	RcptTo   []string `json:"rcpt_to,omitempty"`
	Host     string   `json:"host,omitempty"`
	Family   string   `json:"family,omitempty"`
	Port     int      `json:"port"`
	Addr     string   `json:"addr,omitempty"`
	Helo     string   `json:"helo,omitempty"`
}

type Infos struct {
	BaseInfos
	TimeReported time.Time `json:"timereported"`
	Data         string    `json:"data,omitempty"`
}

type Attachment struct {
	Name         string `json:"name,omitempty"`
	InferedType  string `json:"infered_type,omitempty"`
	ReportedType string `json:"reported_type,omitempty"`
	Size         int    `json:"size"`
	Hash         string `json:"hash,omitempty"`
}

type Address struct {
	Name    string `json:"name,omitempty"`
	Address string `json:"address,omitempty"`
}

type ParsedInfos struct {
	BaseInfos
	Size         int                 `json:"size"`
	ContentType  string              `json:"content_type,omitempty"`
	TimeReported string              `json:"timereported"`
	Headers      map[string][]string `json:"headers,omitempty"`
	Attachments  []Attachment        `json:"attachments,omitempty"`
	BagOfWords   map[string]int      `json:"bag_of_words,omitempty"`
	BagOfStems   map[string]int      `json:"bag_of_stems,omitempty"`
	Language     string              `json:"language,omitempty"`
	TimeHeader   string              `json:"time_header,omitempty"`
	From         Address             `json:"from,omitempty"`
	To           []Address           `json:"to,omitempty"`
	Title        string              `json:"title,omitempty"`
	Emails       []string            `json:"emails,omitempty"`
	Urls         []string            `json:"urls,omitempty"`
	Images       []string            `json:"images,omitempty"`
}

func (i *Infos) Parse() (*ParsedInfos, error) {
	m, err := mail.ReadMessage(strings.NewReader(i.Data))
	if err != nil {
		return nil, err
	}
	parsed := new(ParsedInfos)
	parsed.BaseInfos = i.BaseInfos
	parsed.TimeReported = i.TimeReported.Format(time.RFC3339)
	parsed.Headers = make(map[string][]string)

	for k, vl := range m.Header {
		k = strings.ToLower(k)
		parsed.Headers[k] = make([]string, 0)
		for _, v := range vl {
			decv, err := StringDecode(v)
			if err != nil {
				continue
			}
			parsed.Headers[k] = append(parsed.Headers[k], decv)
		}
	}
	parsed.Size = len(i.Data)

	contentType, plain, htmls, attachments := ParsePart(strings.NewReader(i.Data), "")
	parsed.ContentType = contentType
	parsed.Attachments = attachments
	plain = filterPlain(plain)
	urls := make([]string, 0)
	images := make([]string, 0)

	var b strings.Builder
	for _, h := range htmls {
		t, eurls, eimages := html2text(h)
		if len(t) > 0 {
			b.WriteString(t)
			b.WriteByte('\n')
		}
		urls = append(urls, eurls...)
		images = append(images, eimages...)
	}
	ahtml := b.String()
	if len(plain) == 0 {
		plain = ahtml
	}

	urls = append(urls, xurls.Relaxed().FindAllString(plain, -1)...)
	urls = append(urls, xurls.Relaxed().FindAllString(ahtml, -1)...)
	parsed.Urls = distinct(urls)

	emails := emailRE.FindAllString(plain, -1)
	emails = append(emails, emailRE.FindAllString(ahtml, -1)...)
	parsed.Emails = distinct(emails)

	parsed.Images = distinct(images)

	if len(plain) > 0 {
		parsed.BagOfWords = make(map[string]int)
		bagOfWords(plain, parsed.BagOfWords)
		langinfo := whatlanggo.Detect(plain)
		if langinfo.Script != nil && langinfo.Lang != -1 {
			parsed.Language = strings.ToLower(whatlanggo.Langs[langinfo.Lang])
		}
	}

	if len(parsed.Language) > 0 {
		parsed.BagOfStems = make(map[string]int)
		for word := range parsed.BagOfWords {
			stem, err := snowball.Stem(word, parsed.Language, true)
			if err == nil {
				parsed.BagOfStems[stem] = parsed.BagOfStems[stem] + parsed.BagOfWords[word]
			}
		}
	}

	if len(parsed.Headers["date"]) > 0 {
		d, err := mail.ParseDate(parsed.Headers["date"][0])
		if err == nil {
			parsed.TimeHeader = d.Format(time.RFC3339)
		}
		delete(parsed.Headers, "date")
	}

	if len(parsed.Headers["from"]) > 0 {
		addr := parsed.Headers["from"][0]
		paddr, err := mail.ParseAddress(addr)
		if err == nil {
			parsed.From = Address(*paddr)
		}
		delete(parsed.Headers, "from")
	}

	if len(parsed.Headers["to"]) > 0 {
		paddrs, err := mail.ParseAddressList(parsed.Headers["to"][0])
		if err == nil {
			for _, paddr := range paddrs {
				parsed.To = append(parsed.To, Address(*paddr))
			}
		}
		delete(parsed.Headers, "to")
	}

	if len(parsed.Headers["subject"]) > 0 {
		parsed.Title = parsed.Headers["subject"][0]
		delete(parsed.Headers, "subject")
	}

	return parsed, nil
}

var emailRE = regexp.MustCompile(`(?i)\b[A-Z0-9._%+-]+@[A-Z0-9.-]+\.[A-Z]{2,}\b`)

func distinct(set []string) []string {
	m := make(map[string]bool)
	for _, s := range set {
		m[s] = true
	}
	set = []string{}
	for s := range m {
		set = append(set, s)
	}
	return set
}

func bagOfWords(text string, bag map[string]int) {
	words := extractWords(text)
	if len(words) == 0 {
		return
	}
	for _, word := range words {
		bag[word] = bag[word] + 1
	}
	return
}

func filterPlain(plain string) string {
	plain = strings.TrimSpace(plain)
	if len(plain) == 0 {
		return ""
	}
	lines := make([]string, 0)
	// work around some forwarding bugs in evolution
	for _, line := range strings.FieldsFunc(plain, func(r rune) bool {
		return r == '\n' || r == '\r'
	}) {
		line = strings.TrimLeft(strings.TrimRight(line, "\r\n "), ">\r\n ")
		if len(line) > 0 {
			lines = append(lines, line)
		}
	}
	plain = strings.Join(lines, "\n")
	plain = regexp.MustCompile("(?s)<!--.*-->").ReplaceAllLiteralString(plain, "")
	return strings.TrimSpace(plain)
}

func extractWords(text string) (words []string) {
	text = strings.TrimSpace(text)
	if len(text) == 0 {
		return nil
	}
	doc, err := prose.NewDocument(text, prose.WithExtraction(false), prose.WithSegmentation(false), prose.WithTagging(false))
	if err != nil {
		return nil
	}
	for _, tok := range doc.Tokens() {
		word := filterWord(tok.Text)
		if len(word) > 0 {
			words = append(words, word)
		}
	}
	return words
}

var abbrs []string = []string{"t'", "s'", "d'", "j'", "l'", "m'", "c'", "n'"}

func filterWord(w string) string {
	w = strings.ToLower(w)
	for _, abbr := range abbrs {
		w = strings.TrimPrefix(w, abbr)
	}
	if len(w) <= 3 {
		return ""
	}
	if strings.ContainsAny(w, "<>0123456789_-~#{}[]()|`^=+°&$£µ%/:;,?§!.@") {
		return ""
	}
	return w
}

func html2text(h string) (text string, links []string, images []string) {
	h = strings.TrimSpace(h)
	if len(h) == 0 {
		return "", nil, nil
	}
	texts := make([]string, 0)
	z := html.NewTokenizer(strings.NewReader(h))
	ignoreScript := false
	ignoreStyle := false
	for {
		t := z.Next()
		if t == html.ErrorToken {
			break
		} else if t == html.StartTagToken {
			tok := z.Token()
			tagName := strings.ToLower(tok.Data)
			if tagName == "script" {
				ignoreScript = true
			} else if tagName == "style" {
				ignoreStyle = true
			} else if tagName == "a" {
				for _, attr := range tok.Attr {
					if strings.ToLower(attr.Key) == "href" {
						links = append(links, attr.Val)
						break
					}
				}
			} else if tagName == "img" {
				for _, attr := range tok.Attr {
					if strings.ToLower(attr.Key) == "src" {
						images = append(images, attr.Val)
						break
					}
				}
			} else {
			}
		} else if t == html.EndTagToken {
			tok := z.Token()
			tagName := strings.ToLower(tok.Data)
			if tagName == "script" {
				ignoreScript = false
			} else if tagName == "style" {
				ignoreStyle = false
			}
		} else if t == html.TextToken && !ignoreScript && !ignoreStyle {
			tok := z.Token()
			textData := strings.TrimSpace(tok.Data)
			if len(textData) > 0 {
				texts = append(texts, textData)
			}
		}
	}
	return strings.TrimSpace(strings.Join(texts, " ")), links, images
}

func decodeBody(body io.Reader, charset string) string {
	if charset != "" {
		encoding, err := htmlindex.Get(charset)
		if err == nil {
			r := encoding.NewDecoder().Reader(body)
			res, err := ioutil.ReadAll(r)
			if err == nil {
				return string(res)
			}
		}
	}
	res, err := decodeUtf8(body)
	if err == nil {
		return res
	}
	res, err = decodeLatin1(body)
	if err == nil {
		return res
	}
	// damn, nothing works
	return ""
}

func decodeLatin1(body io.Reader) (string, error) {
	r := charmap.ISO8859_15.NewDecoder().Reader(body)
	res, err := ioutil.ReadAll(r)
	if err != nil {
		return "", err
	}
	return string(res), nil
}

func decodeUtf8(body io.Reader) (string, error) {
	r := unicode.UTF8.NewDecoder().Reader(body)
	res, err := ioutil.ReadAll(r)
	if err != nil {
		return "", err
	}
	return string(res), nil
}

func ParsePart(part io.Reader, contentTypeHeader string) (string, string, []string, []Attachment) {
	plain := ""
	var htmls []string
	var attachments []Attachment

	msg, err := mail.ReadMessage(part)
	if err != nil {
		return "", "", nil, nil
	}
	if len(contentTypeHeader) == 0 {
		contentTypeHeader = strings.TrimSpace(msg.Header.Get("Content-Type"))
	}
	if len(contentTypeHeader) == 0 {
		return "", "", nil, nil
	}
	contentType, params, err := mime.ParseMediaType(contentTypeHeader)
	if err != nil {
		return "", "", nil, nil
	}
	//fmt.Fprintln(os.Stderr, "ParsePart "+contentType)
	charset := strings.TrimSpace(params["charset"])
	if contentType == "text/plain" {
		b := decodeBody(msg.Body, charset)
		return contentType, b, nil, nil
	}
	if contentType == "text/html" {
		return contentType, "", []string{decodeBody(msg.Body, charset)}, nil
	}
	if !strings.HasPrefix(contentType, "multipart/") {
		return contentType, "", nil, nil
	}
	boundary := strings.TrimSpace(params["boundary"])
	if len(boundary) == 0 {
		return contentType, "", nil, nil
	}
	mr := multipart.NewReader(msg.Body, boundary)
	for {
		subpart, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}
		subct := strings.TrimSpace(subpart.Header.Get("Content-Type"))
		if len(subct) == 0 {
			continue
		}
		subContentType, subParams, err := mime.ParseMediaType(subct)
		if err != nil {
			continue
		}
		//fmt.Fprintln(os.Stderr, "NextPart "+subContentType)

		if strings.HasPrefix(subContentType, "multipart/") {
			_, subplain, subhtmls, subAttachments := ParsePart(subpart, subct)
			plain = plain + subplain + "\n"
			htmls = append(htmls, subhtmls...)
			attachments = append(attachments, subAttachments...)
			continue
		}

		if strings.HasPrefix(subContentType, "message/") {
			_, subplain, subhtmls, subAttachments := ParsePart(subpart, "")
			plain = plain + subplain + "\n"
			htmls = append(htmls, subhtmls...)
			attachments = append(attachments, subAttachments...)
			continue
		}

		fn := strings.TrimSpace(subpart.FileName())
		if len(fn) == 0 {
			if subContentType == "text/plain" {
				subCharset := strings.TrimSpace(subParams["charset"])
				b := decodeBody(subpart, subCharset)
				plain = plain + b + "\n"
			}
			if subContentType == "text/html" {
				subCharset := strings.TrimSpace(subParams["charset"])
				htmls = append(htmls, decodeBody(subpart, subCharset))
			}
			continue
		}
		filename, err := StringDecode(fn)
		if err != nil {
			continue
		}
		partBody, err := ioutil.ReadAll(subpart)
		if err != nil {
			continue
		}
		if subpart.Header.Get("Content-Transfer-Encoding") == "base64" {
			decoded, err := base64.StdEncoding.DecodeString(string(partBody))
			if err == nil {
				partBody = decoded
			}
		}
		inferedType := ""
		//fmt.Fprintln(os.Stderr, string(partBody))
		typ, err := filetype.Match(partBody)
		if err == nil {
			inferedType = typ.MIME.Value
		} else {
			//fmt.Fprintln(os.Stderr, "FILETYPE ERROR: "+err.Error())
		}
		h := sha256.Sum256(partBody)
		attachments = append(attachments, Attachment{
			Name:         filename,
			InferedType:  inferedType,
			ReportedType: subContentType,
			Size:         len(partBody),
			Hash:         hex.EncodeToString(h[:]),
		})
	}
	return contentType, plain, htmls, attachments
}

func NewDecoder() *mime.WordDecoder {
	decoder := new(mime.WordDecoder)
	// attach custom charset decoder
	decoder.CharsetReader = func(charset string, input io.Reader) (io.Reader, error) {
		var dec *encoding.Decoder
		// get proper charset decoder
		switch charset {
		case "koi8-r":
			dec = charmap.KOI8R.NewDecoder()
		case "windows-1251":
			dec = charmap.Windows1251.NewDecoder()
		default:
			return nil, fmt.Errorf("unhandled charset %q", charset)
		}
		// read input data
		content, err := ioutil.ReadAll(input)
		if err != nil {
			return nil, err
		}
		// decode
		data, err := dec.Bytes(content)
		if err != nil {
			return nil, err
		}
		return bytes.NewReader(data), nil
	}
	return decoder
}

// WordDecode automatically decodes word if necessary and returns decoded data
func WordDecode(word string) (string, error) {
	// return word unchanged if not RFC 2047 encoded
	if !strings.HasPrefix(word, "=?") || !strings.HasSuffix(word, "?=") || strings.Count(word, "?") != 4 {
		return word, nil
	}
	return NewDecoder().Decode(word)
}

// StringDecode splits text to list of words, decodes
// every word and assembles it back into a decoded string
func StringDecode(text string) (string, error) {
	words := strings.Split(text, " ")
	var err error
	for idx := range words {
		words[idx], err = WordDecode(words[idx])
		if err != nil {
			return "", err
		}
	}
	return strings.Join(words, " "), nil
}
