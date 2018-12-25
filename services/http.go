package services

import (
	"bytes"
	"context"
	"fmt"
	"github.com/stephane-martin/mailstats/forwarders"
	"github.com/stephane-martin/mailstats/parser"
	"github.com/stephane-martin/mailstats/utils"
	"io"
	"io/ioutil"
	"log"
	"mime"
	"net"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/stephane-martin/mailstats/arguments"
	"github.com/stephane-martin/mailstats/collectors"
	"github.com/stephane-martin/mailstats/metrics"
	"github.com/stephane-martin/mailstats/models"

	"github.com/alecthomas/chroma/quick"
	"github.com/gin-gonic/gin"
	"github.com/go-gomail/gomail"
	"github.com/inconshreveable/log15"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/text/encoding/htmlindex"
)

var GinMode string

func init() {
	gin.SetMode(GinMode)
	gin.DisableConsoleColor()
}


func StartHTTP(ctx context.Context, args arguments.HTTPArgs, collector collectors.Collector, forwarder forwarders.Forwarder, logger log15.Logger) error {
	wr := &utils.GinLogger{Logger: logger}
	gin.DefaultWriter = wr
	gin.DefaultErrorWriter = wr
	log.SetOutput(wr)

	router := gin.Default()


	router.Any("/metrics", gin.WrapH(
		promhttp.HandlerFor(
			metrics.M().Registry,
			promhttp.HandlerOpts{
				DisableCompression:  true,
				ErrorLog:            utils.PromLogger(logger),
				ErrorHandling:       promhttp.HTTPErrorOnError,
				MaxRequestsInFlight: -1,
				Timeout:             -1,
			},
		),
	))

	router.GET("/status", func(c *gin.Context) {
		c.Status(200)
	})

	analyzeMime := func(enqueue bool) func(c *gin.Context) {
		return func(c *gin.Context) {
			metrics.M().Connections.WithLabelValues(c.ClientIP(), "http").Inc()
			now := time.Now()

			ct := c.ContentType()
			_, params, err := mime.ParseMediaType(ct)
			if err != nil {
				logger.Warn("Error parsing media type", "error", err)
				c.Status(400)
				return
			}

			charset := strings.TrimSpace(params["charset"])
			if charset == "" {
				charset = "utf-8"
			}
			encoding, err := htmlindex.Get(charset)
			if err != nil {
				logger.Warn("Failed to get decoder", "charset", charset)
				c.Status(500)
				return
			}
			decoder := encoding.NewDecoder()
			decode := func(s string) string {
				res, err := decoder.String(s)
				if err != nil {
					return s
				}
				return res
			}

			var message []byte
			fh, err := c.FormFile("message")
			if err == http.ErrMissingFile {
				message = []byte(decode(c.PostForm("message")))
			} else if err != nil {
				logger.Warn("Failed to get message part", "error", err)
				c.Status(500)
				return
			} else {
				f, err := fh.Open()
				if err != nil {
					logger.Warn("Failed to get message part", "error", err)
					c.Status(500)
					return
				}
				//noinspection GoUnhandledErrorResult
				defer f.Close()
				b, err := ioutil.ReadAll(f)
				if err != nil {
					logger.Warn("Failed to read message part", "error", err)
					c.Status(500)
					return
				}
				message = b
			}
			message = bytes.TrimSpace(message)
			if len(message) == 0 {
				logger.Warn("Empty message")
				c.Status(400)
				return
			}

			var sender *mail.Address
			from := strings.TrimSpace(decode(c.PostForm("from")))
			if len(from) > 0 {
				s, err := mail.ParseAddress(from)
				if err == nil {
					sender = s
				}
			}

			recipients := make([]*mail.Address, 0)
			for _, recipient := range c.PostFormArray("to") {
				recipientAddr, err := mail.ParseAddress(decode(recipient))
				if err == nil {
					recipients = append(recipients, recipientAddr)
				}
			}

			parsed, err := mail.ReadMessage(bytes.NewReader(message))
			if err != nil {
				logger.Warn("ReadMessage() error", "error", err)
				c.Status(500)
				return
			}

			if sender == nil {
				from := strings.TrimSpace(parsed.Header.Get("From"))
				if len(from) > 0 {
					s, err := mail.ParseAddress(from)
					if err == nil {
						sender = s
					}
				}
			}

			if len(recipients) == 0 {
				to := parsed.Header["To"]
				for _, recipient := range to {
					r, err := mail.ParseAddress(recipient)
					if err == nil {
						recipients = append(recipients, r)
					}
				}
			}

			incoming := &models.IncomingMail{
				BaseInfos: models.BaseInfos{
					TimeReported: now,
					Addr:         c.ClientIP(),
					Family:       "http",
					Port:         args.ListenPortAPI,
				},
				Data: []byte(message),
			}
			if sender != nil {
				incoming.MailFrom = sender.Address
			}
			for _, recipient := range recipients {
				incoming.RcptTo = append(incoming.RcptTo, recipient.Address)
			}
			if enqueue {
				err := collectors.CollectAndForward(ctx.Done(), incoming, collector, forwarder)
				if err != nil {
					logger.Error("Error pushing HTTP message to collector", "error", err)
					c.Status(500)
				}
				return
			}

			theparser := parser.NewParser(logger)
			//noinspection GoUnhandledErrorResult
			defer theparser.Close()
			incoming.UID = utils.NewULID()
			features, err := theparser.Parse(incoming)
			if err != nil {
				logger.Warn("Error calculating features", "error", err)
				c.Status(500)
				return
			}
			printFeatures(features, c, logger)
		}
	}

	router.POST("/messages.mime", analyzeMime(true))
	router.POST("/analyze.mime", analyzeMime(false))

	analyze := func(enqueue bool) func(c *gin.Context) {
		return func(c *gin.Context) {
			metrics.M().Connections.WithLabelValues(c.ClientIP(), "http").Inc()
			// cf https://documentation.mailgun.com/en/latest/api-sending
			now := time.Now()
			_, params, err := mime.ParseMediaType(c.ContentType())
			if err != nil {
				logger.Warn("Error parsing media type", "error", err)
				c.Status(400)
				return
			}
			charset := strings.TrimSpace(params["charset"])
			if charset == "" {
				charset = "utf-8"
			}
			encoding, err := htmlindex.Get(charset)
			if err != nil {
				logger.Warn("Failed to get decoder", "charset", charset)
				c.Status(500)
				return
			}
			decoder := encoding.NewDecoder()
			decode := func(s string) string {
				res, err := decoder.String(s)
				if err != nil {
					return s
				}
				return res
			}

			message := gomail.NewMessage(
				gomail.SetCharset("utf8"),
				gomail.SetEncoding(gomail.Unencoded),
			)
			message.SetHeader("Date", message.FormatDate(now))

			// from, to, cc, bcc, subject, text, html, attachment
			from := strings.TrimSpace(decode(c.PostForm("from")))
			if len(from) > 0 {
				sender, err := mail.ParseAddress(from)
				if err == nil {
					from = sender.Address
					message.SetAddressHeader("From", sender.Address, sender.Name)
				} else {
					from = ""
				}
			}

			to := make([]*mail.Address, 0)
			for _, recipient := range c.PostFormArray("to") {
				recipientAddr, err := mail.ParseAddress(decode(recipient))
				if err == nil {
					to = append(to, recipientAddr)
				}
			}
			encodedTo := make([]string, 0)
			for _, addr := range to {
				encodedTo = append(encodedTo, message.FormatAddress(addr.Address, addr.Name))
			}
			if len(encodedTo) > 0 {
				message.SetHeader("To", encodedTo...)
			}

			cc := make([]string, 0)
			for _, recipient := range c.PostFormArray("cc") {
				r, err := mail.ParseAddress(decode(recipient))
				if err == nil {
					cc = append(cc, message.FormatAddress(r.Address, r.Name))
				}
			}
			if len(cc) > 0 {
				message.SetHeader("Cc", cc...)
			}

			bcc := make([]string, 0)
			for _, recipient := range c.PostFormArray("bcc") {
				r, err := mail.ParseAddress(decode(recipient))
				if err == nil {
					bcc = append(bcc, message.FormatAddress(r.Address, r.Name))
				}
			}
			if len(bcc) > 0 {
				message.SetHeader("Bcc", bcc...)
			}

			subject := decode(c.PostForm("subject"))
			if len(subject) > 0 {
				message.SetHeader("Subject", subject)
			}

			text := strings.TrimSpace(decode(c.PostForm("text")))
			html := strings.TrimSpace(decode(c.PostForm("html")))

			if len(text) > 0 && len(html) > 0 {
				message.SetBody("text/plain", text)
				message.AddAlternative("text/html", html)
			} else if len(text) > 0 {
				message.SetBody("text/plain", text)
			} else if len(html) > 0 {
				message.SetBody("text/html", html)
			}

			forms, err := c.MultipartForm()
			if err != nil {
				logger.Warn("Error parsing multipart", "error", err)
				c.Status(500)
				return
			}

			if forms != nil {
				for _, fheader := range forms.File["attachment"] {
					message.Attach(fheader.Filename, gomail.SetCopyFunc(func(w io.Writer) error {
						f, err := fheader.Open()
						if err != nil {
							return err
						}
						defer func() { _ = f.Close() }()
						_, err = io.Copy(w, f)
						return err
					}))
				}
			}

			var b bytes.Buffer
			_, err = message.WriteTo(&b)
			if err != nil {
				logger.Warn("Error marshalling message to MIME", "error", err)
				c.Status(500)
				return
			}
			incoming := &models.IncomingMail{
				BaseInfos: models.BaseInfos{
					TimeReported: now,
					Addr:         c.ClientIP(),
					Family:       "http",
					Port:         args.ListenPortAPI,
				},
				Data: b.Bytes(),
			}
			if len(from) > 0 {
				incoming.MailFrom = from
			}
			for _, addr := range to {
				incoming.RcptTo = append(incoming.RcptTo, addr.Address)
			}
			if enqueue {
				err := collectors.CollectAndForward(ctx.Done(), incoming, collector, forwarder)
				if err != nil {
					logger.Error("Error pushing HTTP message to collector", "error", err)
					c.Status(500)
				}
				return
			}

			theparser := parser.NewParser(logger)
			//noinspection GoUnhandledErrorResult
			defer theparser.Close()
			incoming.UID = utils.NewULID()
			features, err := theparser.Parse(incoming)
			if err != nil {
				logger.Warn("Error calculating features", "error", err)
				c.Status(500)
				return
			}
			printFeatures(features, c, logger)


		}
	}

	router.POST("/messages", analyze(true))
	router.POST("/analyze", analyze(false))

	svc := &http.Server{
		Addr:    net.JoinHostPort(args.ListenAddrAPI, fmt.Sprintf("%d", args.ListenPortAPI)),
		Handler: router,
	}

	go func() {
		<-ctx.Done()
		_ = svc.Close()
		logger.Info("HTTP service closed")
	}()

	logger.Info("Starting HTTP service")

	err := svc.ListenAndServe()
	if err != nil {
		logger.Info("HTTP service error", "error", err)
	}
	return err

}


func printFeatures(features *models.FeaturesMail, c *gin.Context, logger log15.Logger) {
	switch c.NegotiateFormat(
		"application/json",
		"text/html",
		"application/x-yaml",
		"text/yaml",
	) {
	case "application/json":
		c.JSON(200, features)
	case "text/html":
		b, err := features.Encode(true)
		if err != nil {
			logger.Warn("Failed to serialize features", "error", err)
			c.Status(500)
			return
		}
		err = quick.Highlight(c.Writer, string(b), "json", "html", "colorful")
		if err != nil {
			logger.Warn("Failed to colorize features", "error", err)
			c.Status(500)
			return
		}
	case "application/x-yaml", "text/yaml":
		c.YAML(200, features)
	default:
		c.JSON(200, features)
	}
}