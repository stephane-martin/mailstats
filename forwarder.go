package main

import (
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
		return "","", "", "", ""
	}
	u, _ := url.Parse(args.URL)
	host, port , _ = net.SplitHostPort(u.Host)
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
	if len(username) == 0 || len(password) == 0 {
		logger.Info("Forwarding without auth", "scheme", scheme, "host", host, "port", port)
		return SMTPForwarder{scheme: scheme, host: host, port: port, logger: logger}, nil
	}
	logger.Info("Forwarding", "scheme", scheme, "host", host, "port", port, "username", username)
	return SMTPForwarder{scheme: scheme, host: host, port: port, username: username, password: password, logger: logger}, nil
}

type Forwarder interface {
	Forward(info Infos) error
	GetLogger() log15.Logger
}

type DummyForwarder struct {}


func (_ DummyForwarder) Forward(_ Infos) error {
	return nil
}

func (_ DummyForwarder) GetLogger() log15.Logger {
	return nil
}

type SMTPForwarder struct {
	scheme string
	host string
	port string
	username string
	password string
	logger log15.Logger
}

func (f SMTPForwarder) GetLogger() log15.Logger {
	return f.logger
}

// TODO: put that in a goroutine
func (f SMTPForwarder) Forward(info Infos) (err error) {
	if len(info.RcptTo) == 0 || len(info.MailFrom) == 0 {
		return nil
	}
	var conn net.Conn
	var client *smtp.Client
	if f.scheme == "http" {
		conn, err = net.Dial("tcp", net.JoinHostPort(f.host, f.port))
	} else {
		conn, err = tls.Dial("tcp", net.JoinHostPort(f.host, f.port), nil)
	}
	if err != nil {
		return fmt.Errorf("failed to dial remote SMTP service: %s", err)
	}
	client, err = smtp.NewClient(conn, f.host)
	if err != nil {
		return fmt.Errorf("failed to build SMTP client: %s", err)
	}
	defer func() {
		if err == nil {
			err = client.Quit()
			if err != nil {
				err = fmt.Errorf("error while quitting SMTP service: %s", err)
				return
			}
			f.logger.Info("Message has been forwarded")
		}
	}()
	err = client.Hello(ifempty(info.Helo, "localhost"))
	if err != nil {
		return fmt.Errorf("error at HELO: %s", err)
	}
	supportStartTLS, _ := client.Extension("STARTTLS")
	if supportStartTLS && f.scheme == "smtp" {
		err := client.StartTLS(&tls.Config{ServerName: f.host})
		if err != nil {
			return fmt.Errorf("error while doing STARTTLS: %s", err)
		}
	}
	supportAuth, _ := client.Extension("AUTH")
	if supportAuth && len(f.username) > 0 {
		err := client.Auth(sasl.NewPlainClient("", f.username, f.password))
		if err != nil {
			return fmt.Errorf("error performing AUTH with remote SMTP service: %s", err)
		}
	}
	err = client.Mail(info.MailFrom)
	if err != nil {
		return fmt.Errorf("error at MAIL FROM: %s", err)
	}
	for _, to := range info.RcptTo {
		err = client.Rcpt(to)
		if err != nil {
			return fmt.Errorf("error at RCPT TO: %s", err)
		}
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("error at DATA: %s", err)
	}
	_, err = io.WriteString(w, info.Data)
	_ = w.Close()
	if err != nil {
		return fmt.Errorf("error writing DATA: %s", err)
	}
	return nil
}
