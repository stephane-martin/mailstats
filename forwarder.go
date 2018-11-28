package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"github.com/emersion/go-sasl"
	"github.com/emersion/go-smtp"
	"github.com/inconshreveable/log15"
	"github.com/storozhukBM/verifier"
	"github.com/urfave/cli"
	"io"
	"net"
	"net/url"
	"strings"
	"sync"
	"time"
)

func (args *SMTPArgs) Populate(c *cli.Context) *SMTPArgs {
	if args == nil {
		args = new(SMTPArgs)
	}
	args.ListenPort = c.Int("lport")
	args.ListenAddr = strings.TrimSpace(c.String("laddr"))
	args.MaxMessageSize = c.Int("max-size")
	args.MaxIdle = c.Int("max-idle")
	args.Inetd = c.GlobalBool("inetd")
	return args
}

type ForwardArgs struct {
	URL string
}

func (args ForwardArgs) Parsed() (scheme, host, port, username, password string) {
	if args.URL == "" {
		return "", "", "", "", ""
	}
	u, _ := url.Parse(args.URL)
	host, port, _ = net.SplitHostPort(u.Host)
	password, _ = u.User.Password()
	return strings.ToLower(strings.TrimSpace(u.Scheme)),
		strings.TrimSpace(host),
		strings.TrimSpace(port),
		strings.TrimSpace(u.User.Username()),
		strings.TrimSpace(password)
}

func (args ForwardArgs) Verify() error {
	if args.URL == "" {
		return nil
	}
	u, err := url.Parse(args.URL)
	v := verifier.New()
	v.That(err == nil, "Invalid SMTP forward URL")
	scheme := strings.ToLower(strings.TrimSpace(u.Scheme))
	v.That(scheme == "smtp" || scheme == "smtps", "Forward URL scheme is not smtp")
	v.That(len(u.Host) > 0, "Forward host is empty")
	h, p, err := net.SplitHostPort(u.Host)
	v.That(err == nil, "Forward host must be host:port")
	v.That(len(h) > 0, "Forward host is empty")
	v.That(len(p) > 0, "Forward port is empty")
	return v.GetError()
}

func (args *ForwardArgs) Populate(c *cli.Context) *ForwardArgs {
	if args == nil {
		args = new(ForwardArgs)
	}
	args.URL = strings.TrimSpace(c.GlobalString("forward"))
	return args
}

func (args ForwardArgs) Build(logger log15.Logger) (Forwarder, error) {
	scheme, host, port, username, password := args.Parsed()
	if host == "" {
		logger.Info("No forwarding")
		return DummyForwarder{}, nil
	}
	var f SMTPForwarder
	mailsChan := make(chan IncomingMail, 10000)
	if len(username) == 0 || len(password) == 0 {
		logger.Info("Forwarding without auth", "scheme", scheme, "host", host, "port", port)
		f = SMTPForwarder{
			scheme: scheme,
			host:   host,
			port:   port,
			logger: logger,
			mails:  mailsChan,
		}
	} else {
		logger.Info("Forwarding", "scheme", scheme, "host", host, "port", port, "username", username)
		f = SMTPForwarder{
			scheme:   scheme,
			host:     host,
			port:     port,
			username: username,
			password: password,
			logger:   logger,
			mails:    mailsChan,
		}
	}
	return f, nil
}

func chan2buffer(c chan IncomingMail) (buffer []IncomingMail, stop bool) {
	for {
		select {
		case email, more := <-c:
			if !more {
				// no more mails to forward
				return buffer, true
			}
			buffer = append(buffer, email)
		default:
			return buffer, false
		}
	}
}

type Forwarder interface {
	Push(mail IncomingMail)
	Start(ctx context.Context) error
	Close() error
	GetLogger() log15.Logger
}

type DummyForwarder struct{}

func (_ DummyForwarder) Push(_ IncomingMail) {}

func (_ DummyForwarder) GetLogger() log15.Logger {
	return nil
}

func (_ DummyForwarder) Close() error {
	return nil
}

func (_ DummyForwarder) Start(ctx context.Context) error {
	return nil
}

type SMTPForwarder struct {
	scheme    string
	host      string
	port      string
	username  string
	password  string
	logger    log15.Logger
	mails     chan IncomingMail
	closeOnce sync.Once
}

func (f SMTPForwarder) GetLogger() log15.Logger {
	return f.logger
}

func (f SMTPForwarder) Close() error {
	close(f.mails)
	return nil
}

func (f SMTPForwarder) Start(ctx context.Context) error {
	var stop bool
	var rest []IncomingMail
	var err error
	buffer := make([]IncomingMail, 0)
	for {
		buffer, stop = chan2buffer(f.mails)
		if len(rest) > 0 {
			buffer = append(buffer, rest...)
		}
		if len(buffer) > 0 {
			rest, err = f.forward(buffer)
			if err != nil {
				f.logger.Warn("Error forwarding emails", "error", err)
				select {
				case <-ctx.Done():
					return context.Canceled
				case <-time.After(10 * time.Second):
				}
			}
		} else if stop {
			f.logger.Info("Stop forwarding emails")
			return nil
		} else {
			f.logger.Debug("No email to forward")
			select {
			case <-ctx.Done():
				return context.Canceled
			case <-time.After(2 * time.Second):
			}
		}
	}
}

func (f SMTPForwarder) Push(email IncomingMail) {
	f.mails <- email
}

func _forward(email IncomingMail, client *smtp.Client) (err error) {
	if len(email.RcptTo) == 0 || len(email.MailFrom) == 0 {
		return nil
	}

	err = client.Mail(email.MailFrom)
	if err != nil {
		return fmt.Errorf("error at MAIL FROM: %s", err)
	}
	for _, to := range email.RcptTo {
		err = client.Rcpt(to)
		if err != nil {
			return fmt.Errorf("error at RCPT TO: %s", err)
		}
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("error at DATA: %s", err)
	}
	_, err = io.WriteString(w, email.Data)
	_ = w.Close()
	if err != nil {
		return fmt.Errorf("error writing DATA: %s", err)
	}
	return nil
}

func (f SMTPForwarder) forward(emails []IncomingMail) (rest []IncomingMail, err error) {
	if len(emails) == 0 {
		return nil, nil
	}
	f.GetLogger().Info("Emails to forward", "nb", len(emails))

	var conn net.Conn
	var client *smtp.Client

	if f.scheme == "http" {
		conn, err = net.Dial("tcp", net.JoinHostPort(f.host, f.port))
	} else {
		conn, err = tls.Dial("tcp", net.JoinHostPort(f.host, f.port), nil)
	}
	if err != nil {
		return emails, fmt.Errorf("failed to dial remote SMTP service: %s", err)
	}
	defer func() { _ = conn.Close() }()

	client, err = smtp.NewClient(conn, f.host)
	if err != nil {
		return emails, fmt.Errorf("failed to build SMTP client: %s", err)
	}
	defer func() { _ = client.Quit() }()

	err = client.Hello("mailstats")
	if err != nil {
		return emails, fmt.Errorf("error at HELO: %s", err)
	}

	supportStartTLS, _ := client.Extension("STARTTLS")
	if supportStartTLS && f.scheme == "smtp" {
		err := client.StartTLS(&tls.Config{ServerName: f.host})
		if err != nil {
			return emails, fmt.Errorf("error while doing STARTTLS: %s", err)
		}
	}
	supportAuth, _ := client.Extension("AUTH")
	if supportAuth && len(f.username) > 0 {
		err := client.Auth(sasl.NewPlainClient("", f.username, f.password))
		if err != nil {
			return emails, fmt.Errorf("error performing AUTH with remote SMTP service: %s", err)
		}
	}

	for len(emails) > 0 {
		err := _forward(emails[0], client)
		if err != nil {
			return emails, err
		}
		emails = emails[1:]
	}
	return nil, nil
}
