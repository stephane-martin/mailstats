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
	"github.com/emersion/go-message/charset"
	"github.com/stephane-martin/mailstats/metrics"
	"io"
	"io/ioutil"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/h2non/filetype/matchers"
	"github.com/jordic/goics"
	"github.com/oklog/ulid"
	"github.com/stephane-martin/mailstats/collectors"
	"github.com/stephane-martin/mailstats/consumers"
	"github.com/stephane-martin/mailstats/extractors"
	"github.com/stephane-martin/mailstats/forwarders"
	"github.com/stephane-martin/mailstats/models"
	"github.com/stephane-martin/mailstats/utils"
	"github.com/xi2/xz"
	"golang.org/x/sync/errgroup"

	"github.com/inconshreveable/log15"
	"github.com/mvdan/xurls"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/htmlindex"
	"golang.org/x/text/encoding/unicode"
)


type Parser interface {
	Parse(i *models.IncomingMail) (*models.FeaturesMail, error)
	Close() error
}

type ParserImpl struct {
	logger log15.Logger
	tool   *extractors.ExifToolWrapper
}

func NewParser(logger log15.Logger) *ParserImpl {
	tool, err := extractors.NewExifToolWrapper()
	if err != nil {
		logger.Info("Failed to locate exiftool", "error", err)
		return &ParserImpl{logger: logger}
	}
	return &ParserImpl{logger: logger, tool: tool}
}

func (p *ParserImpl) Close() error {
	if p.tool != nil {
		return p.tool.Close()
	}
	return nil
}

func (p *ParserImpl) Parse(i *models.IncomingMail) (features *models.FeaturesMail, err error) {
	now := time.Now()
	defer func() {
		if err == nil {
			metrics.M().ParsingDuration.Observe(time.Now().Sub(now).Seconds())
		} else {
			metrics.M().ParsingErrors.WithLabelValues(i.Family).Inc()
		}
	}()
	metrics.M().MessageSize.Observe(float64(len(i.Data)))

	m, err := mail.ReadMessage(bytes.NewReader(i.Data))
	if err != nil {
		metrics.M().ParsingErrors.WithLabelValues(i.Family)
		return nil, err
	}
	features = new(models.FeaturesMail)
	features.BaseInfos = i.BaseInfos
	uid := ulid.ULID(i.BaseInfos.UID).String()
	features.UID = uid
	features.Reported = i.TimeReported.Format(time.RFC3339)
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
		t, eURLs, eImages := extractors.HTML2Text(h)
		if len(t) > 0 {
			b.WriteString(t)
			b.WriteString(".\n")
		}
		urls = append(urls, eURLs...)
		images = append(images, eImages...)
	}
	ahtml := strings.TrimSpace(b.String())
	if len(ahtml) > 0 && (len(ahtml) >= (len(plain)/2)) {
		plain = ahtml
	}
	// unicode normalization
	plain = utils.Normalize(plain)

	moreURLs := xurls.Relaxed().FindAllString(plain, -1)
	moreURLs = append(moreURLs, xurls.Relaxed().FindAllString(ahtml, -1)...)
	for _, u := range moreURLs {
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
		features.Language = extractors.Language(plain)
	}

	if len(features.Language) > 0 {
		bagOfWords := extractors.BagOfWords(plain, features.Language)
		stems := extractors.Stems(bagOfWords, features.Language)
		features.BagOfWords = make(map[string]int)
		for word := range bagOfWords {
			features.BagOfWords[stems[word]] += bagOfWords[word]
		}
		features.Keywords, features.Phrases = extractors.Keywords(plain, stems, features.Language)
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
		pAddrs, err := mail.ParseAddressList(features.Headers["to"][0])
		if err == nil {
			for _, pAddr := range pAddrs {
				features.To = append(features.To, models.Address(*pAddr))
			}
		}
		delete(features.Headers, "to")
	}

	if len(features.Headers["subject"]) > 0 {
		features.Title = features.Headers["subject"][0]
		delete(features.Headers, "subject")
	}

	if len(features.Headers["received"]) > 0 {
		features.Received = make([]models.ReceivedElement, 0)
		for _, h := range features.Headers["received"] {
			features.Received = append(features.Received, extractors.ParseReceivedHeader(h, p.logger))
		}
		delete(features.Headers, "received")
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

func decodeBody(body io.Reader, charset string, qp string) string {
	if qp == "quoted-printable" {
		body = quotedprintable.NewReader(body)
	} else if qp == "base64" {
		body = base64.NewDecoder(base64.StdEncoding, body)
	}
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
	res, err := decodeUTF8(body)
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

func decodeUTF8(body io.Reader) (string, error) {
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

	charSet := strings.TrimSpace(params["charset"])
	transferEncoding := strings.ToLower(strings.TrimSpace(msg.Header.Get("Content-Transfer-Encoding")))
	if contentType == "text/plain" {
		b := decodeBody(msg.Body, charSet, transferEncoding)
		return contentType, b, nil, nil
	}
	if contentType == "text/html" {
		return contentType, "", []string{decodeBody(msg.Body, charSet, transferEncoding)}, nil
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
		subPart, err := mr.NextPart()
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
		subContentTypeHeader := strings.TrimSpace(subPart.Header.Get("Content-Type"))
		subTransferHeader := strings.TrimSpace(subPart.Header.Get("Content-Transfer-Encoding"))

		if len(subContentTypeHeader) == 0 {
			logger.Info("NextPart: no Content-Type")
			continue
		}

		subContentType, subParams, err := mime.ParseMediaType(subContentTypeHeader)
		if err != nil {
			logger.Info("NextPart Content-Type parsing", "error", err)
			continue
		}
		subCharset := strings.TrimSpace(subParams["charset"])

		if strings.HasPrefix(subContentType, "message/") {
			_, subplain, subhtmls, subAttachments := ParsePart(subPart, tool, logger)
			plain = plain + subplain + "\n"
			htmls = append(htmls, subhtmls...)
			attachments = append(attachments, subAttachments...)
			continue
		}

		if strings.HasPrefix(subContentType, "multipart/") {
			h := fmt.Sprintf("Content-Type: %s\n", subContentTypeHeader)
			if subTransferHeader != "" {
				h += fmt.Sprintf("Content-Transfer-Encoding: %s\n", subTransferHeader)
			}
			h += "\n"

			_, subplain, subhtmls, subAttachments := ParsePart(
				io.MultiReader(
					strings.NewReader(h),
					subPart,
				),
				tool,
				logger,
			)
			plain = plain + subplain + "\n"
			htmls = append(htmls, subhtmls...)
			attachments = append(attachments, subAttachments...)
			continue
		}

		fn := strings.TrimSpace(subPart.FileName())
		if len(fn) == 0 {
			if subContentType == "text/plain" {
				b := decodeBody(subPart, subCharset, subTransferHeader)
				plain = plain + b + "\n"
			}
			if subContentType == "text/html" {
				htmls = append(htmls, decodeBody(subPart, subCharset, subTransferHeader))
			}
			continue
		}
		filename, err := StringDecode(fn)
		if err != nil {
			continue
		}
		var subPartReader io.Reader = subPart
		if subTransferHeader == "base64" {
			subPartReader = base64.NewDecoder(base64.StdEncoding, subPartReader)
		}
		attachment, err := AnalyseAttachment(filename, subContentType, subPartReader, tool, logger)
		if err == nil {
			attachment.ReportedType = subContentType
			attachments = append(attachments, attachment)
		}
	}
	return contentType, plain, htmls, attachments
}

func AnalyseAttachment(filename string, ct string, r io.Reader, t *extractors.ExifToolWrapper, l log15.Logger) (*models.Attachment, error) {
	content, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	attachment := &models.Attachment{
		Name: filename,
		Size: uint64(len(content)),
		ReportedType: ct,
	}

	h := sha256.Sum256(content)
	attachment.Hash = hex.EncodeToString(h[:])

	typ, err := utils.Guess(filename, content)
	if err != nil {
		return nil, err
	}
	attachment.InferredType = typ.MIME.Value
	attachment.Executable = extractors.IsExecutable(attachment.InferredType)
	l.Debug("Attachment", "value", typ.MIME.Value, "filename", filename)

	if attachment.Executable {
		meta, err := t.Extract(content, "-EXE:All")
		if err != nil {
			l.Warn("Failed to extract metadata with exiftool", "error", err)
		} else {
			attachment.ExeMetadata = meta
		}
		return attachment, nil
	}

	switch typ {
	case matchers.TypePdf:
		m, err := extractors.PDFBytesInfo(content)
		if err != nil {
			l.Warn("Error extracting metadata from PDF", "error", err)
		} else {
			attachment.PDFMetadata = m
		}
	case matchers.TypeDocx:
		text, props, hasMacro, err := extractors.ConvertBytesDocx(content)
		if err != nil {
			l.Warn("Error extracting metadata from DOCX", "error", err)
		} else {
			meta := &models.DocMeta{
				Properties: props,
				HasMacro:   hasMacro,
				Language:   extractors.Language(text),
			}
			if meta.Language != "" {
				meta.Keywords, meta.Phrases = extractors.Keywords(text, nil, meta.Language)
			}
			attachment.DocMetadata = meta
		}
	case utils.OdtType:
		text, props, err := extractors.ConvertBytesODT(content)
		if err != nil {
			l.Warn("Error extracting metadata from ODT", "error", err)
		} else {
			meta := &models.DocMeta{
				Properties: props,
				Language:   extractors.Language(text),
			}
			if meta.Language != "" {
				meta.Keywords, meta.Phrases = extractors.Keywords(text, nil, meta.Language)
			}
			attachment.DocMetadata = meta
		}

	case matchers.TypeDoc:
		text, err := extractors.ConvertBytesDoc(content)
		if err != nil {
			l.Warn("Error extracting text from DOC", "error", err)
		} else {
			meta := &models.DocMeta{
				Language: extractors.Language(text),
			}
			p, err := t.Extract(content, "-FlashPix:All")
			if err != nil {
				l.Warn("Error extracting metadata from DOC", "error", err)
			} else {
				meta.Properties = p
			}
			if meta.Language != "" {
				meta.Keywords, meta.Phrases = extractors.Keywords(text, nil, meta.Language)
			}
			attachment.DocMetadata = meta
		}

	case matchers.TypePng:
		if t != nil {
			meta, err := t.Extract(content, "-EXIF:All", "-PNG:All")
			if err != nil {
				l.Warn("Failed to extract metadata with exiftool", "error", err)
			} else {
				attachment.ImageMetadata = meta
			}
		}

	case matchers.TypeJpeg, matchers.TypeWebp, matchers.TypeGif:
		if t != nil {
			meta, err := t.Extract(content, "-EXIF:All")
			if err != nil {
				l.Warn("Failed to extract metadata with exiftool", "error", err)
			} else {
				attachment.ImageMetadata = meta
			}
		}
	case matchers.TypeZip, matchers.TypeTar, matchers.TypeRar:
		archive, err := AnalyzeArchive(typ, bytes.NewReader(content), attachment.Size, l)
		if err != nil {
			l.Warn("Error analyzing archive", "error", err)
		} else {
			if attachment.Archives == nil {
				attachment.Archives = make(map[string]*models.Archive)
			}
			attachment.Archives[filename] = archive
		}

	case matchers.TypeGz:
		gzReader, err := gzip.NewReader(bytes.NewReader(content))
		if err != nil {
			l.Warn("Failed to decompress gzip attachement", "error", err)
		} else {
			if strings.HasSuffix(filename, ".gz") {
				filename = filename[:len(filename)-3]
			}
			subAttachment, err := AnalyseAttachment(filename, "", gzReader, t, l)
			if err != nil {
				l.Warn("Error analyzing subattachement", "error", err)
			} else {
				attachment.SubAttachment = subAttachment
				attachment.Executable = subAttachment.Executable
			}
		}

	case matchers.TypeBz2:
		bz2Reader := bzip2.NewReader(bytes.NewReader(content))
		if strings.HasSuffix(filename, ".bz2") {
			filename = filename[:len(filename)-4]
		}
		subAttachment, err := AnalyseAttachment(filename, "", bz2Reader, t, l)
		if err != nil {
			l.Warn("Error analyzing sub-attachement", "error", err)
		} else {
			attachment.SubAttachment = subAttachment
			attachment.Executable = subAttachment.Executable
		}

	case matchers.TypeXz:
		xzReader, err := xz.NewReader(bytes.NewReader(content), 0)
		if err != nil {
			l.Warn("Failed to decompress xz attachement", "error", err)
		} else {
			if strings.HasSuffix(filename, ".xz") {
				filename = filename[:len(filename)-3]
			}
			subAttachment, err := AnalyseAttachment(filename, "", xzReader, t, l)
			if err != nil {
				l.Warn("Error analyzing subattachement", "error", err)
			} else {
				attachment.SubAttachment = subAttachment
				attachment.Executable = subAttachment.Executable
			}
		}

	default:
		if typ.MIME.Value == "text/plain" && strings.HasSuffix(filename, ".ics") {
			dec := goics.NewDecoder(bytes.NewReader(content))
			c := new(extractors.IcalConsumer)
			err := dec.Decode(c)
			if err == nil && c.Events != nil {
				attachment.EventsMetadata = c.Events
			}
		} else {
			l.Info("Unknown attachment type", "type", typ.MIME.Value)
		}
	}

	return attachment, nil
}

func NewDecoder() *mime.WordDecoder {
	decoder := new(mime.WordDecoder)
	// attach custom charset decoder
	decoder.CharsetReader = charset.Reader
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
	g, lCtx := errgroup.WithContext(ctx)

	if nbParsers == 0 {
		<-ctx.Done()
		return nil
	}

	for i := 0; i < nbParsers; i++ {
		g.Go(func() error {
		L:
			for {
				incoming, err := collector.PullCtx(lCtx)
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
				collector.ACK(incoming.UID)
				if err != nil {
					logger.Info("Failed to parse message", "error", err)
					continue L
				}
				g.Go(func() error {
					err := consumer.Consume(features)
					if err != nil {
						logger.Warn("Failed to consume parsing results", "error", err)
					} else {
						logger.Debug("Parsing results sent to consumer")
					}
					return nil
				})

			}
		})
	}
	return g.Wait()
}
