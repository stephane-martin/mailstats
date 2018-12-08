package main

import (
	"bytes"
	"compress/bzip2"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"github.com/h2non/filetype/matchers"
	"github.com/oklog/ulid"
	"github.com/stephane-martin/mailstats/collectors"
	"github.com/stephane-martin/mailstats/consumers"
	"github.com/stephane-martin/mailstats/extractors"
	"github.com/stephane-martin/mailstats/forwarders"
	"github.com/stephane-martin/mailstats/models"
	"github.com/stephane-martin/mailstats/utils"
	"github.com/xi2/xz"
	"golang.org/x/sync/errgroup"
	"io"
	"io/ioutil"
	"mime"
	"mime/multipart"
	"net/mail"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/abadojack/whatlanggo"
	"github.com/inconshreveable/log15"
	"github.com/mvdan/xurls"
	"golang.org/x/net/html"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/htmlindex"
	"golang.org/x/text/encoding/unicode"
)


type Parser interface {
	Parse(i *models.IncomingMail) (*models.FeaturesMail, error)
	Close() error
}

type parserImpl struct {
	logger log15.Logger
	tool   *extractors.ExifToolWrapper
}

func NewParser(logger log15.Logger) *parserImpl {
	tool, err := extractors.NewExifToolWrapper()
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

func (p *parserImpl) Parse(i *models.IncomingMail) (features *models.FeaturesMail, err error) {
	m, err := mail.ReadMessage(bytes.NewReader(i.Data))
	if err != nil {
		return nil, err
	}
	features = new(models.FeaturesMail)
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
	ahtml := strings.TrimSpace(b.String())
	if len(ahtml) > 0 {
		plain = ahtml
	}
	// unicode normalization
	plain = utils.Normalize(plain)

	moreurls := xurls.Relaxed().FindAllString(plain, -1)
	moreurls = append(moreurls, xurls.Relaxed().FindAllString(ahtml, -1)...)
	for _, u := range moreurls {
		v, err := url.PathUnescape(u)
		if err == nil {
			features.Urls = append(features.Urls, v)
		}
	}
	features.Urls = distinct(urls)

	emails := emailRE.FindAllString(plain, -1)
	emails = append(emails, emailRE.FindAllString(ahtml, -1)...)
	features.Emails = distinct(emails)

	features.Images = distinct(images)

	if len(plain) > 0 {
		langInfo := whatlanggo.Detect(plain)
		if langInfo.Script != nil && langInfo.Lang != -1 {
			features.Language = strings.ToLower(whatlanggo.Langs[langInfo.Lang])
		}
		features.BagOfWords = extractors.BagOfWords(plain, features.Language)
	}

	if len(features.Language) > 0 {
		stems := extractors.Stems(features.BagOfWords, features.Language)
		features.BagOfStems = make(map[string]int)
		for word := range features.BagOfWords {
			features.BagOfStems[stems[word]] = features.BagOfStems[stems[word]] + features.BagOfWords[word]
		}
		features.Keywords = extractors.Keywords(plain, stems, features.Language)
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
			features.From = models.Address(*paddr)
		}
		delete(features.Headers, "from")
	}

	if len(features.Headers["to"]) > 0 {
		paddrs, err := mail.ParseAddressList(features.Headers["to"][0])
		if err == nil {
			for _, paddr := range paddrs {
				features.To = append(features.To, models.Address(*paddr))
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



func html2text(h string) (text string, links []string, images []string) {
	h = strings.TrimSpace(h)
	if len(h) == 0 {
		return "", nil, nil
	}
	texts := make([]string, 0)
	z := html.NewTokenizer(strings.NewReader(h))
	ignoreScript := false
	ignoreNoScript := false
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
			} else if tagName == "noscript" {
				ignoreNoScript = true
			} else if tagName == "style" {
				ignoreStyle = true
			} else if tagName == "a" {
				for _, attr := range tok.Attr {
					if strings.ToLower(attr.Key) == "href" {
						v, err := url.PathUnescape(attr.Val)
						if err == nil {
							links = append(links, v)
						}
						break
					}
				}
			} else if tagName == "img" {
				for _, attr := range tok.Attr {
					if strings.ToLower(attr.Key) == "src" {
						v, err := url.PathUnescape(attr.Val)
						if err == nil {
							images = append(images, v)
						}
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
			} else if tagName == "noscript" {
				ignoreNoScript = false
			} else if tagName == "style" {
				ignoreStyle = false
			} else if tagName == "title" || tagName == "h1" || tagName == "h2" || tagName == "h3" || tagName == "h4" || tagName == "h5" || tagName == "h6" {
				texts = append(texts, ".")
			}
		} else if t == html.TextToken && !ignoreScript && !ignoreStyle && !ignoreNoScript {
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

func ParsePart(part io.Reader, tool *extractors.ExifToolWrapper, logger log15.Logger) (string, string, []string, []*models.Attachment) {
	plain := ""
	var htmls []string
	var attachments []*models.Attachment

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

func AnalyseAttachment(filename string, reader io.Reader, tool *extractors.ExifToolWrapper, logger log15.Logger) (*models.Attachment, error) {
	content, err := ioutil.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	attachment := &models.Attachment{Name: filename, Size: uint64(len(content))}
	h := sha256.Sum256(content)
	attachment.Hash = hex.EncodeToString(h[:])

	typ, err := utils.Guess(content)
	if err != nil {
		return nil, err
	}
	attachment.InferredType = typ.MIME.Value
	logger.Debug("Attachment", "value", typ.MIME.Value, "type", typ.MIME.Type, "subtype", typ.MIME.Subtype)

	switch typ {
	case matchers.TypePdf:
		m, err := extractors.PDFBytesInfo(content)
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
				attachment.Archives = make(map[string]*models.Archive)
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

func ParseMails(ctx context.Context, collector collectors.Collector, parser Parser, consumer consumers.Consumer, forwarder forwarders.Forwarder, nbParsers int, logger log15.Logger) error {
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
					continue L
				}
				forwarder.Push(*incoming)
				features, err := parser.Parse(incoming)
				if err != nil {
					logger.Info("Failed to parse message", "error", err)
					continue L
				}
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
