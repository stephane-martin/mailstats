package main

import (
	"context"
	"fmt"
	"github.com/go-gomail/gomail"
	"github.com/inconshreveable/log15"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/storozhukBM/verifier"
	"github.com/urfave/cli"
	"io"
	"mime"
	"net"
	"net/http"
	"net/mail"
	"strings"
	"time"
)

type HTTPArgs struct {
	ListenAddr string
	ListenPort int
}

func (args HTTPArgs) Verify() error {
	v := verifier.New()
	v.That(args.ListenPort > 0, "The HTTP listen port must be positive")
	v.That(len(args.ListenAddr) > 0, "The HTTP listen address is empty")
	p := net.ParseIP(args.ListenAddr)
	v.That(p != nil, "The HTTP listen address is invalid")
	return v.GetError()
}

func (args *HTTPArgs) Populate(c *cli.Context) *HTTPArgs {
	if args == nil {
		args = new(HTTPArgs)
	}
	args.ListenPort = c.GlobalInt("http-port")
	args.ListenAddr = strings.TrimSpace(c.GlobalString("http-addr"))
	return args
}

func StartHTTP(ctx context.Context, args HTTPArgs, collector Collector, logger log15.Logger) error {
	if args.ListenPort <= 0 {
		return nil
	}
	if args.ListenAddr == "" {
		args.ListenAddr = "127.0.0.1"
	}

	muxer := http.NewServeMux()

	muxer.Handle(
		"/metrics",
		promhttp.HandlerFor(
			M().Registry,
			promhttp.HandlerOpts{
				DisableCompression:  true,
				ErrorLog:            adaptPromLogger(logger),
				ErrorHandling:       promhttp.HTTPErrorOnError,
				MaxRequestsInFlight: -1,
				Timeout:             -1,
			},
		),
	)

	muxer.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})

	muxer.HandleFunc("/messages", func(w http.ResponseWriter, r *http.Request) {
		// https://documentation.mailgun.com/en/latest/api-sending

		if r.Method != "POST" {
			logger.Warn("Incoming /messages is not POST", "method", r.Method)
			w.WriteHeader(400)
			return
		}
		ct := r.Header.Get("Content-Type")
		mt, _, err := mime.ParseMediaType(ct)
		if err != nil {
			logger.Warn("Error parsing media type", "error", err)
			w.WriteHeader(400)
			return
		}
		if mt != "multipart/form-data" {
			logger.Warn("Incoming /messages is not multipart", "contenttype", mt)
			w.WriteHeader(400)
			return
		}

		err = r.ParseMultipartForm(67108864)
		if err != nil {
			logger.Warn("Error parsing message from /messages HTTP endpoint", "error", err)
			w.WriteHeader(500)
			return
		}
		// TODO: Content-Type and charset?

		message := gomail.NewMessage(
			gomail.SetCharset("utf8"),
			gomail.SetEncoding(gomail.Unencoded),
		)
		now := time.Now()
		message.SetHeader("Date", message.FormatDate(now))

		// from, to, cc, bcc, subject, text, html, attachment
		from := strings.TrimSpace(r.Form.Get("from"))
		if len(from) > 0 {
			r, err := mail.ParseAddress(from)
			if err == nil {
				from = r.Address
				message.SetAddressHeader("From", r.Address, r.Name)
			} else {
				from = ""
			}
		}

		to := make([]*mail.Address, 0, len(r.Form["to"]))
		for _, recipient := range r.Form["to"] {
			r, err := mail.ParseAddress(recipient)
			if err == nil {
				to = append(to, r)
			}
		}
		encodedTo := make([]string, 0, len(to))
		for _, addr := range to {
			encodedTo = append(encodedTo, message.FormatAddress(addr.Address, addr.Name))
		}
		if len(encodedTo) > 0 {
			message.SetHeader("To", encodedTo...)
		}

		cc := make([]string, 0)
		for _, recipient := range r.Form["cc"] {
			r, err := mail.ParseAddress(recipient)
			if err == nil {
				cc = append(cc, message.FormatAddress(r.Address, r.Name))
			}
		}
		if len(cc) > 0 {
			message.SetHeader("Cc", cc...)
		}

		bcc := make([]string, 0)
		for _, recipient := range r.Form["bcc"] {
			r, err := mail.ParseAddress(recipient)
			if err == nil {
				bcc = append(bcc, message.FormatAddress(r.Address, r.Name))
			}
		}
		if len(bcc) > 0 {
			message.SetHeader("Bcc", bcc...)
		}

		subject := r.Form.Get("subject")
		if len(subject) > 0 {
			message.SetHeader("Subject", subject)
		}

		text := strings.TrimSpace(r.Form.Get("text"))
		html := strings.TrimSpace(r.Form.Get("html"))

		if len(text) > 0 && len(html) > 0 {
			message.SetBody("text/plain", text)
			message.AddAlternative("text/html", html)
		} else if len(text) > 0 {
			message.SetBody("text/plain", text)
		} else if len(html) > 0 {
			message.SetBody("text/html", html)
		}

		for _, fheader := range r.MultipartForm.File["attachment"] {
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

		var b strings.Builder
		_, err = message.WriteTo(&b)
		if err != nil {
			logger.Warn("Error marshalling HTTP message to MIME", "error", err)
			w.WriteHeader(500)
			return
		}
		infos := new(Infos)
		infos.Data = b.String()
		infos.TimeReported = now
		if len(from) > 0 {
			infos.MailFrom = from
		}
		for _, addr := range to {
			infos.RcptTo = append(infos.RcptTo, addr.Address)
		}
		infos.Addr = r.RemoteAddr
		infos.Family = "http"
		infos.Port = args.ListenPort
		err = collector.PushCtx(ctx, infos)
		if err != nil {
			logger.Error("Error pushing HTTP message to collector", "error", err)
			w.WriteHeader(500)
			return
		}
		w.WriteHeader(200)
	})

	muxer.HandleFunc("/messages.mime", func(w http.ResponseWriter, r *http.Request) {
		// https://documentation.mailgun.com/en/latest/api-sending

	})

	svc := &http.Server{
		Addr:    net.JoinHostPort(args.ListenAddr, fmt.Sprintf("%d", args.ListenPort)),
		Handler: muxer,
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
