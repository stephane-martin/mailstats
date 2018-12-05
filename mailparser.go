package main

import (
	"bytes"
	"compress/bzip2"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/h2non/filetype/matchers"
	"github.com/oklog/ulid"
	"github.com/xi2/xz"
	"golang.org/x/sync/errgroup"
	"io"
	"io/ioutil"
	"mime"
	"mime/multipart"
	"net/mail"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/abadojack/whatlanggo"
	"github.com/inconshreveable/log15"
	"github.com/jdkato/prose"
	"github.com/kljensen/snowball"
	"github.com/mvdan/xurls"
	"golang.org/x/net/html"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/htmlindex"
	"golang.org/x/text/encoding/unicode"
)

type FeaturesMail struct {
	BaseInfos
	UID          string              `json:"uid,omitempty"`
	Size         int                 `json:"size"`
	ContentType  string              `json:"content_type,omitempty"`
	TimeReported string              `json:"timereported,omitempty"`
	Headers      map[string][]string `json:"headers,omitempty"`
	Attachments  []*Attachment       `json:"attachments,omitempty"`
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

type Attachment struct {
	Name          string                 `json:"name,omitempty"`
	InferredType  string                 `json:"inferred_type,omitempty"`
	ReportedType  string                 `json:"reported_type,omitempty"`
	Size          uint64                 `json:"size"`
	Hash          string                 `json:"hash,omitempty"`
	PDFMetadata   *PDFMeta               `json:"pdf_metadata,omitempty"`
	ImageMetadata map[string]interface{} `json:"image_metadata,omitempty"`
	Archives      map[string]*Archive    `json:"archive_content,omitempty"`
	SubAttachment *Attachment            `json:"sub_attachment,omitempty"`
}

type Address struct {
	Name    string `json:"name,omitempty"`
	Address string `json:"address,omitempty"`
}

var stopWordsEnglish map[string]struct{}
var stopWordsFrench map[string]struct{}

func init() {
	enB, err := Asset("data/stopwords-en.txt")
	if err != nil {
		panic(err)
	}
	enF, err := Asset("data/stopwords-fr.txt")
	if err != nil {
		panic(err)
	}
	stopWordsEnglish = make(map[string]struct{})
	stopWordsFrench = make(map[string]struct{})
	for _, word := range strings.Split(string(enB), "\n") {
		word = strings.ToLower(strings.TrimSpace(word))
		if len(word) > 0 {
			stopWordsEnglish[word] = struct{}{}
		}
	}
	for _, word := range strings.Split(string(enF), "\n") {
		word = strings.ToLower(strings.TrimSpace(word))
		if len(word) > 0 {
			stopWordsFrench[normalize(word)] = struct{}{}
		}
	}
}

type Parser interface {
	Parse(i *IncomingMail) (*FeaturesMail, error)
	Close() error
}

type parserImpl struct {
	logger log15.Logger
	tool   *ExifToolWrapper
}

func NewParser(logger log15.Logger) *parserImpl {
	tool, err := NewExifToolWrapper()
	if err != nil {
		logger.Info("Failed to locate exiftool", "error", err)
		return &parserImpl{logger: logger}
	}
	return &parserImpl{logger: logger, tool: tool}
}

func (p *parserImpl) Close() error {
	if p.tool != nil {
		return p.tool.Close()
	}
	return nil
}

func (p *parserImpl) Parse(i *IncomingMail) (features *FeaturesMail, err error) {
	m, err := mail.ReadMessage(bytes.NewReader(i.Data))
	if err != nil {
		return nil, err
	}
	features = new(FeaturesMail)
	features.BaseInfos = i.BaseInfos
	uid := ulid.ULID(i.BaseInfos.UID).String()
	features.UID = uid
	features.TimeReported = i.TimeReported.Format(time.RFC3339)
	features.Headers = make(map[string][]string)

	for k, vl := range m.Header {
		k = strings.ToLower(k)
		features.Headers[k] = make([]string, 0)
		for _, v := range vl {
			decv, err := StringDecode(v)
			if err != nil {
				p.logger.Debug("Error decoding header value", "key", k, "value", v, "error", err)
				continue
			}
			features.Headers[k] = append(features.Headers[k], decv)
		}
	}
	features.Size = len(i.Data)

	contentType, plain, htmls, attachments := ParsePart(bytes.NewReader(i.Data), p.tool, p.logger)
	features.ContentType = contentType
	features.Attachments = attachments
	plain = filterPlain(plain)
	urls := make([]string, 0)
	images := make([]string, 0)

	var b strings.Builder
	for _, h := range htmls {
		t, eurls, eimages := html2text(h)
		if len(t) > 0 {
			b.WriteString(t)
			b.WriteString(".\n")
		}
		urls = append(urls, eurls...)
		images = append(images, eimages...)
	}
	ahtml := b.String()
	if len(plain) == 0 {
		plain = ahtml
	}
	// unicode normalization
	plain = normalize(plain)

	urls = append(urls, xurls.Relaxed().FindAllString(plain, -1)...)
	urls = append(urls, xurls.Relaxed().FindAllString(ahtml, -1)...)
	features.Urls = distinct(urls)

	emails := emailRE.FindAllString(plain, -1)
	emails = append(emails, emailRE.FindAllString(ahtml, -1)...)
	features.Emails = distinct(emails)

	features.Images = distinct(images)

	if len(plain) > 0 {
		features.BagOfWords = make(map[string]int)
		bagOfWords(plain, features.BagOfWords)
		langInfo := whatlanggo.Detect(plain)
		if langInfo.Script != nil && langInfo.Lang != -1 {
			features.Language = strings.ToLower(whatlanggo.Langs[langInfo.Lang])
		}
	}

	if len(features.Language) > 0 {
		switch features.Language {
		case "english":
			for word := range features.BagOfWords {
				if _, ok := stopWordsEnglish[word]; ok {
					delete(features.BagOfWords, word)
				}
			}
		case "french":
			for word := range features.BagOfWords {
				if _, ok := stopWordsFrench[word]; ok {
					delete(features.BagOfWords, word)
				}
			}
		default:
		}
		features.BagOfStems = make(map[string]int)
		for word := range features.BagOfWords {
			stem, err := snowball.Stem(word, features.Language, true)
			if err == nil {
				features.BagOfStems[stem] = features.BagOfStems[stem] + features.BagOfWords[word]
			}
		}
	}

	if len(features.Headers["date"]) > 0 {
		d, err := mail.ParseDate(features.Headers["date"][0])
		if err == nil {
			features.TimeHeader = d.Format(time.RFC3339)
		}
		delete(features.Headers, "date")
	}

	if len(features.Headers["from"]) > 0 {
		addr := features.Headers["from"][0]
		paddr, err := mail.ParseAddress(addr)
		if err == nil {
			features.From = Address(*paddr)
		}
		delete(features.Headers, "from")
	}

	if len(features.Headers["to"]) > 0 {
		paddrs, err := mail.ParseAddressList(features.Headers["to"][0])
		if err == nil {
			for _, paddr := range paddrs {
				features.To = append(features.To, Address(*paddr))
			}
		}
		delete(features.Headers, "to")
	}

	if len(features.Headers["subject"]) > 0 {
		features.Title = features.Headers["subject"][0]
		delete(features.Headers, "subject")
	}

	return features, nil
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

var abbrs = []string{"t'", "s'", "d'", "j'", "l'", "m'", "c'", "n'", "qu'", "«"}

func filterWord(w string) string {
	w = strings.ToLower(w)
	for _, abbr := range abbrs {
		for {
			w2 := strings.TrimPrefix(w, abbr)
			if w2 == w {
				break
			}
			w = w2
		}
	}
	w = strings.TrimSuffix(w, "»")
	if utf8.RuneCountInString(w) <= 3 {
		return ""
	}
	if strings.ContainsAny(w, "«<>0123456789_-~#{}[]()|`^=+°&$£µ%/:;,?§!.@") {
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
		encoder, err := htmlindex.Get(charset)
		if err == nil {
			r := encoder.NewDecoder().Reader(body)
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

func ParsePart(part io.Reader, tool *ExifToolWrapper, logger log15.Logger) (string, string, []string, []*Attachment) {
	plain := ""
	var htmls []string
	var attachments []*Attachment

	msg, err := mail.ReadMessage(part)
	if err != nil {
		logger.Info("ReadMessage", "error", err)
		return "", "", nil, nil
	}
	ctHeader := strings.TrimSpace(msg.Header.Get("Content-Type"))
	if len(ctHeader) == 0 {
		logger.Info("No Content-Type")
		return "", "", nil, nil
	}
	contentType, params, err := mime.ParseMediaType(ctHeader)
	if err != nil {
		logger.Info("Content-Type parsing error", "error", err)
		return "", "", nil, nil
	}

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
			logger.Info("NextPart", "error", err)
			if strings.Contains(err.Error(), "EOF") {
				break
			}
			continue
		}
		subctHeader := strings.TrimSpace(subpart.Header.Get("Content-Type"))
		if len(subctHeader) == 0 {
			logger.Info("NextPart: no Content-Type")
			continue
		}
		subContentType, subParams, err := mime.ParseMediaType(subctHeader)
		if err != nil {
			logger.Info("NextPart Content-Type parsing", "error", err)
			continue
		}

		if strings.HasPrefix(subContentType, "message/") {
			_, subplain, subhtmls, subAttachments := ParsePart(subpart, tool, logger)
			plain = plain + subplain + "\n"
			htmls = append(htmls, subhtmls...)
			attachments = append(attachments, subAttachments...)
			continue
		}

		if strings.HasPrefix(subContentType, "multipart/") {
			_, subplain, subhtmls, subAttachments := ParsePart(
				io.MultiReader(
					strings.NewReader(fmt.Sprintf("Content-Type: %s\n\n", subctHeader)),
					subpart,
				),
				tool,
				logger,
			)
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
		var subPartReader io.Reader = subpart
		if subpart.Header.Get("Content-Transfer-Encoding") == "base64" {
			subPartReader = base64.NewDecoder(base64.StdEncoding, subPartReader)
		}
		attachment, err := AnalyseAttachment(filename, subPartReader, tool, logger)
		if err == nil {
			attachment.ReportedType = subContentType
			attachments = append(attachments, attachment)
		}
	}
	return contentType, plain, htmls, attachments
}

func AnalyseAttachment(filename string, reader io.Reader, tool *ExifToolWrapper, logger log15.Logger) (*Attachment, error) {
	content, err := ioutil.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	attachment := &Attachment{Name: filename, Size: uint64(len(content))}
	h := sha256.Sum256(content)
	attachment.Hash = hex.EncodeToString(h[:])

	typ, err := Guess(content)
	if err != nil {
		return nil, err
	}
	attachment.InferredType = typ.MIME.Value
	logger.Debug("Attachment", "value", typ.MIME.Value, "type", typ.MIME.Type, "subtype", typ.MIME.Subtype)

	switch typ {
	case matchers.TypePdf:
		m, err := PDFBytesInfo(content)
		if err != nil {
			logger.Warn("Error extracting metadata from PDF", "error", err)
		} else {
			attachment.PDFMetadata = m
		}
	case matchers.TypePng, matchers.TypeJpeg, matchers.TypeWebp, matchers.TypeGif:
		if tool != nil {
			meta, err := tool.Extract(content)
			if err != nil {
				logger.Warn("Failed to extract metadata with exiftool", "error", err)
			} else {
				attachment.ImageMetadata = meta
			}
		}
	case matchers.TypeZip, matchers.TypeTar, matchers.TypeRar:
		archive, err := AnalyzeArchive(typ, bytes.NewReader(content), attachment.Size, logger)
		if err != nil {
			logger.Warn("Error analyzing archive", "error", err)
		} else {
			if attachment.Archives == nil {
				attachment.Archives = make(map[string]*Archive)
			}
			attachment.Archives[filename] = archive
		}

	case matchers.TypeGz:
		gzReader, err := gzip.NewReader(bytes.NewReader(content))
		if err != nil {
			logger.Warn("Failed to decompress gzip attachement", "error", err)
		} else {
			if strings.HasSuffix(filename, ".gz") {
				filename = filename[:len(filename)-3]
			}
			subAttachment, err := AnalyseAttachment(filename, gzReader, tool, logger)
			if err != nil {
				logger.Warn("Error analyzing subattachement", "error", err)
			} else {
				attachment.SubAttachment = subAttachment
			}
		}

	case matchers.TypeBz2:
		bz2Reader := bzip2.NewReader(bytes.NewReader(content))
		if strings.HasSuffix(filename, ".bz2") {
			filename = filename[:len(filename)-4]
		}
		subAttachment, err := AnalyseAttachment(filename, bz2Reader, tool, logger)
		if err != nil {
			logger.Warn("Error analyzing sub-attachement", "error", err)
		} else {
			attachment.SubAttachment = subAttachment
		}

	case matchers.TypeXz:
		xzReader, err := xz.NewReader(bytes.NewReader(content), 0)
		if err != nil {
			logger.Warn("Failed to decompress xz attachement", "error", err)
		} else {
			if strings.HasSuffix(filename, ".xz") {
				filename = filename[:len(filename)-3]
			}
			subAttachment, err := AnalyseAttachment(filename, xzReader, tool, logger)
			if err != nil {
				logger.Warn("Error analyzing subattachement", "error", err)
			} else {
				attachment.SubAttachment = subAttachment
			}
		}

	}



	return attachment, nil
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
		case "windows-1252":
			dec = charmap.Windows1252.NewDecoder()
		case "windows-1258":
			dec = charmap.Windows1258.NewDecoder()
		case "iso-8859-15":
			dec = charmap.ISO8859_15.NewDecoder()
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

func ParseMails(ctx context.Context, collector Collector, parser Parser, consumer Consumer, forwarder Forwarder, nbParsers int, logger log15.Logger) error {
	g, lctx := errgroup.WithContext(ctx)

	if nbParsers == 0 {
		<-ctx.Done()
		return nil
	}

	for i := 0; i < nbParsers; i++ {
		g.Go(func() error {
		L:
			for {
				incoming, err := collector.PullCtx(lctx)
				if err == context.Canceled {
					return err
				}
				if err != nil {
					logger.Warn("Error fetching mail from collector", "error", err)
					continue L
				}
				if incoming == nil {
					return errors.New("incoming mail is nil :(")
				}
				forwarder.Push(*incoming)
				features, err := parser.Parse(incoming)
				if err != nil {
					logger.Info("Failed to parse message", "error", err)
					continue L
				}
				// TODO: in own single goroutine ?
				go func() {
					err := consumer.Consume(features)
					if err != nil {
						logger.Warn("Failed to consume parsing results", "error", err)
					} else {
						logger.Info("Parsing results sent to consumer")
					}
				}()

			}
		})
	}
	return g.Wait()
}
