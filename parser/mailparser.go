package parser

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/ahmetb/go-linq"
	"github.com/stephane-martin/mailstats/arguments"
	"github.com/stephane-martin/mailstats/metrics"
	"github.com/stephane-martin/mailstats/phishtank"
	"go.uber.org/fx"

	"github.com/oklog/ulid"
	"github.com/russross/blackfriday"
	"github.com/stephane-martin/mailstats/collectors"
	"github.com/stephane-martin/mailstats/consumers"
	"github.com/stephane-martin/mailstats/extractors"
	"github.com/stephane-martin/mailstats/models"
	"github.com/stephane-martin/mailstats/utils"
	"golang.org/x/sync/errgroup"

	"github.com/emersion/go-dkim"
	"github.com/inconshreveable/log15"
	"github.com/mvdan/xurls"
)

type Parser interface {
	utils.Service
	utils.Startable
	Parse(i *models.IncomingMail) (*models.FeaturesMail, error)
	ParseMany(context.Context, <-chan *models.IncomingMail, chan<- *models.FeaturesMail)
}

func NewParser(nbWorkers int,
	nodkim bool,
	collector collectors.Collector,
	consumer consumers.Consumer,
	geoip utils.GeoIP,
	tool extractors.ExifTool,
	phishtank phishtank.Phishtank,
	logger log15.Logger,
) Parser {

	parser := impl{
		logger:    logger,
		collector: collector,
		consumer:  consumer,
		nbWorkers: nbWorkers,
		tool:      tool,
		geoip:     geoip,
		noDKIM:    nodkim,
		phishtank: phishtank,
	}

	return &parser
}

type Params struct {
	fx.In
	Args      *arguments.Args      `optional:"true"`
	Collector collectors.Collector `optional:"true"`
	Consumer  consumers.Consumer   `optional:"true"`
	Tool      extractors.ExifTool  `optional:"true"`
	GeoIP     utils.GeoIP          `optional:"true"`
	Phishtank phishtank.Phishtank  `optional:"true"`
	Logger    log15.Logger         `optional:"true"`
}

var Service = fx.Provide(func(lc fx.Lifecycle, params Params) Parser {
	var nbWorkers = int(1)
	if params.Args != nil {
		nbWorkers = params.Args.NbParsers
	}
	logger := params.Logger
	if logger == nil {
		logger = log15.New()
		logger.SetHandler(log15.DiscardHandler())
	}
	p := NewParser(
		nbWorkers,
		params.Args.NoDKIM,
		params.Collector,
		params.Consumer,
		params.GeoIP,
		params.Tool,
		params.Phishtank,
		logger,
	)
	utils.Append(lc, p, logger)
	return p
})

type impl struct {
	logger    log15.Logger
	tool      extractors.ExifTool
	collector collectors.Collector
	consumer  consumers.Consumer
	nbWorkers int
	geoip     utils.GeoIP
	phishtank phishtank.Phishtank
	noDKIM    bool
}

func (p *impl) Name() string { return "Parser" }

func (p *impl) ParseMany(ctx context.Context, incoming <-chan *models.IncomingMail, results chan<- *models.FeaturesMail) {
	defer close(results)

	nbWorkers := p.nbWorkers
	if nbWorkers <= 0 {
		nbWorkers = 1
	}
	g, lCtx := errgroup.WithContext(ctx)
	for i := 0; i < nbWorkers; i++ {
		g.Go(func() error {
			for {
				select {
				case <-lCtx.Done():
					return lCtx.Err()
				case i, ok := <-incoming:
					if !ok {
						return nil
					}
					features, err := p.Parse(i)
					if err != nil {
						p.logger.Info("Error parsing incoming mail", "error", err)
					} else {
						results <- features
					}
				}
			}
		})
	}
	_ = g.Wait()
}

func (p *impl) Start(ctx context.Context) error {
	if p.nbWorkers == 0 || p.collector == nil || p.consumer == nil {
		<-ctx.Done()
		return nil
	}

	g, lCtx := errgroup.WithContext(ctx)

	for i := 0; i < p.nbWorkers; i++ {
		g.Go(func() error {
		L:
			for {
				incoming, err := p.collector.PullCtx(lCtx)
				if err == context.Canceled {
					return err
				}
				if err != nil {
					p.logger.Warn("Error fetching mail from collector", "error", err)
					continue L
				}
				if incoming == nil {
					continue L
				}
				features, err := p.Parse(incoming)
				p.collector.ACK(incoming.UID)
				if err != nil {
					p.logger.Info("Failed to parse message", "error", err)
					continue L
				}
				g.Go(func() error {
					err := p.consumer.Consume(features)
					if err != nil {
						p.logger.Warn("Failed to consume parsing results", "error", err)
					} else {
						p.logger.Debug("Parsing results sent to consumer")
					}
					return nil
				})

			}
		})
	}
	return g.Wait()
}

func (p *impl) Parse(i *models.IncomingMail) (features *models.FeaturesMail, err error) {
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
	features.Size = int64(len(i.Data))

	contentType, plain, htmls, attachments := ParsePart(bytes.NewReader(i.Data), p.tool, p.logger)
	features.ContentType = contentType
	features.Attachments = attachments
	plain = filterPlain(plain)
	urls := make([]string, 0)
	images := make([]string, 0)
	moreURLs := xurls.Relaxed().FindAllString(plain, -1)

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
	if len(ahtml) > 0 && (len(ahtml) >= (len(plain) / 2)) {
		plain = ahtml
	}
	// unicode normalization
	plain = utils.Normalize(plain)

	moreURLs = append(moreURLs, xurls.Relaxed().FindAllString(ahtml, -1)...)
	for _, u := range moreURLs {
		v, err := url.PathUnescape(u)
		if err == nil {
			urls = append(features.URLs, strings.Replace(v, "&amp;", "&", -1))
		} else {
			p.logger.Debug("Error decoding URL", "error", err, "url", u)
		}
	}
	features.URLs = distinct(urls)
	if p.phishtank != nil {
		features.PhishtankURLS = p.phishtank.URLMany(features.URLs)
	}

	emails := findEmailAddresses(plain)
	emails = append(emails, findEmailAddresses(ahtml)...)
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
		paddr, err := ParseAddress(addr)
		features.From = new(models.FromAddress)
		if err == nil {
			addrInName, multiple, different, spoofed := nameContainsAddress(paddr)
			features.From.Address.Name = paddr.Name
			features.From.Address.Address = paddr.Address
			features.From.NameContainsAddress = addrInName
			features.From.Multiple = multiple
			features.From.Different = different
			features.From.Spoofed = spoofed
		} else {
			features.From.Address.Address = addr
			features.From.Error = err.Error()
		}
		delete(features.Headers, "from")
	}

	if len(features.Headers["to"]) > 0 {
		pAddrs, err := ParseAddressList(features.Headers["to"][0])
		if err == nil {
			for _, pAddr := range pAddrs {
				features.To = append(features.To, models.Address{
					Name:    pAddr.Name,
					Address: pAddr.Address,
				})
			}
			delete(features.Headers, "to")
		} else {
			p.logger.Info("Error parsing the address list from the To: header", "error", err)
		}
	}

	if len(features.Headers["subject"]) > 0 {
		features.Title = features.Headers["subject"][0]
		delete(features.Headers, "subject")
	}

	if len(features.Headers["received"]) > 0 {
		features.Received = make([]models.ReceivedElement, 0)
		for _, h := range features.Headers["received"] {
			features.Received = append(features.Received, extractors.ParseReceivedHeader(h, p.geoip, p.logger))
		}
		delete(features.Headers, "received")
	}

	if len(features.Headers["dkim-signature"]) > 0 && !p.noDKIM {
		r := bytes.NewReader(i.Data)
		v, err := dkim.Verify(r)
		if err != nil {
			p.logger.Warn("Error trying to validate DKIM signature", "error", err)
		} else {
			var err error
			var verifications []models.DKIMVerification
			for _, verif := range v {
				if verif.Err != nil {
					err = verif.Err
					break
				}
				verifications = append(verifications, models.DKIMVerification{
					Domain:     verif.Domain,
					Identifier: verif.Identifier,
				})
			}
			if err != nil {
				features.DKIM = &models.DKIMValidation{Error: err.Error()}
			} else {
				features.DKIM = &models.DKIMValidation{
					Verifications: verifications,
				}
			}
		}
	}

	return features, nil
}

func nameContainsAddress(fullAddr *mail.Address) (bool, bool, bool, bool) {
	if fullAddr == nil {
		return false, false, false, false
	}
	name := strings.TrimSpace(fullAddr.Name)
	if name == "" {
		return false, false, false, false
	}
	addr := strings.TrimSpace(fullAddr.Address)
	if addr == "" {
		return false, false, false, false
	}
	addrDomain := getDomainFromAddress(addr)
	if addrDomain == "" {
		return false, false, false, false
	}
	addrsInName := findEmailAddresses(name)
	if len(addrsInName) == 0 {
		return false, false, false, false
	}
	totalAddressLength := int(0)
	nonAlphaChars := len(alphaChars.ReplaceAllString(name, ""))
	for _, addr := range addrsInName {
		nonAlphaChars -= len(alphaChars.ReplaceAllString(addr, ""))
		totalAddressLength += len(addr)
	}
	if (len(name) - totalAddressLength) > (5 - nonAlphaChars) {
		return false, false, false, false
	}
	linq.From(addrsInName).WhereT(func(a string) bool {
		return getDomainFromAddress(a) != ""
	}).ToSlice(&addrsInName)

	if len(addrsInName) == 0 {
		return false, false, false, false
	}
	if len(addrsInName) > 1 {
		// name contains multiple addresses
		return true, true, false, false
	}
	addrInName := addrsInName[0]
	differentAddress := addr != addrInName
	spoofed := addrDomain != getDomainFromAddress(addrInName)
	return true, false, differentAddress, spoofed
}

func getDomainFromAddress(addr string) string {
	u, err := url.Parse("mailto://" + addr)
	if err != nil {
		return ""
	}
	return u.Host
}

var emailRE = regexp.MustCompile(`(?i)\b[A-Z0-9._%+-]+@[A-Z0-9.-]+\.[A-Z]{2,}\b`)
var alphaChars = regexp.MustCompile(`[A-Za-z0-9]`)

func findEmailAddresses(s string) []string {
	return emailRE.FindAllString(s, -1)
}

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

func ParsePart(part io.Reader, tool extractors.ExifTool, logger log15.Logger) (string, string, []string, []*models.Attachment) {
	plain := ""
	var allHTML []string
	var attachments []*models.Attachment

	msg, err := mail.ReadMessage(part)
	if err != nil {
		logger.Info("ReadMessage", "error", err)
		return "", "", nil, nil
	}
	ctHeader := strings.TrimSpace(msg.Header.Get("Content-Type"))
	if len(ctHeader) == 0 {
		logger.Info("No Content-Type header, assume text/plain in UTF-8")
		ctHeader = "text/plain; charset=UTF-8"
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
	if contentType == "text/markdown" || contentType == "text/x-markdown" || contentType == "text/x-gfm" {
		html := blackfriday.Run([]byte(decodeBody(msg.Body, charSet, transferEncoding)))
		return contentType, "", []string{string(html)}, nil
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
			allHTML = append(allHTML, subhtmls...)
			attachments = append(attachments, subAttachments...)
			continue
		}

		if strings.HasPrefix(subContentType, "multipart/") {
			h := fmt.Sprintf("Content-Type: %s\n", subContentTypeHeader)
			if subTransferHeader != "" {
				h += fmt.Sprintf("Content-Transfer-Encoding: %s\n", subTransferHeader)
			}
			h += "\n"

			_, subPlain, subAllHTML, subAttachments := ParsePart(
				io.MultiReader(
					strings.NewReader(h),
					subPart,
				),
				tool,
				logger,
			)
			plain = plain + subPlain + "\n"
			allHTML = append(allHTML, subAllHTML...)
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
				allHTML = append(allHTML, decodeBody(subPart, subCharset, subTransferHeader))
			}
			if subContentType == "text/markdown" || subContentType == "text/x-markdown" || subContentType == "text/x-gfm" {
				html := blackfriday.Run([]byte(decodeBody(subPart, subCharset, subTransferHeader)))
				allHTML = append(allHTML, string(html))
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
	return contentType, plain, allHTML, attachments
}
