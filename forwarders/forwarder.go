package forwarders

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/emersion/go-sasl"
	"github.com/emersion/go-smtp"
	"github.com/inconshreveable/log15"
	"github.com/stephane-martin/mailstats/arguments"
	"github.com/stephane-martin/mailstats/models"
)

func Build(args arguments.ForwardArgs, logger log15.Logger) (Forwarder, error) {
	scheme, host, port, username, password := args.Parsed()
	if host == "" {
		logger.Info("No forwarding")
		return DummyForwarder{}, nil
	}
	mailsChan := make(chan models.IncomingMail, 10000)
	f := &SMTPForwarder{
		Scheme: scheme,
		Host:   host,
		Port:   port,
		Logger: logger,
		Mails:  mailsChan,
	}
	if len(username) == 0 || len(password) == 0 {
		logger.Info("Forwarding without auth", "scheme", scheme, "host", host, "port", port)
	} else {
		logger.Info("Forwarding", "scheme", scheme, "host", host, "port", port, "username", username)
		f.Username = username
		f.Password = password
	}
	return f, nil
}

func chan2buffer(c chan models.IncomingMail) (buffer []models.IncomingMail, stop bool) {
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
	Push(mail models.IncomingMail)
	Start(ctx context.Context) error
	Close() error
	GetLogger() log15.Logger
}

type DummyForwarder struct{}

func (_ DummyForwarder) Push(_ models.IncomingMail) {}

func (_ DummyForwarder) GetLogger() log15.Logger {
	return nil
}

func (_ DummyForwarder) Close() error {
	return nil
}

func (_ DummyForwarder) Start(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

type SMTPForwarder struct {
	Scheme    string
	Host      string
	Port      string
	Username  string
	Password  string
	Logger    log15.Logger
	Mails     chan models.IncomingMail
	CloseOnce sync.Once
}

func (f *SMTPForwarder) GetLogger() log15.Logger {
	return f.Logger
}

func (f *SMTPForwarder) Close() error {
	close(f.Mails)
	return nil
}

func (f *SMTPForwarder) Start(ctx context.Context) error {
	var stop bool
	var rest []models.IncomingMail
	var err error
	buffer := make([]models.IncomingMail, 0)
	for {
		buffer, stop = chan2buffer(f.Mails)
		if len(rest) > 0 {
			buffer = append(buffer, rest...)
		}
		if len(buffer) > 0 {
			rest, err = f.forward(buffer)
			if err != nil {
				f.Logger.Warn("Error forwarding emails", "error", err)
				select {
				case <-ctx.Done():
					return context.Canceled
				case <-time.After(10 * time.Second):
				}
			}
		} else if stop {
			f.Logger.Info("Stop forwarding emails")
			return nil
		} else {
			f.Logger.Debug("No email to forward")
			select {
			case <-ctx.Done():
				return context.Canceled
			case <-time.After(2 * time.Second):
			}
		}
	}
}

func (f *SMTPForwarder) Push(email models.IncomingMail) {
	f.Mails <- email
}

func _forward(email models.IncomingMail, client *smtp.Client) (err error) {
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
	_, err = w.Write(email.Data)
	_ = w.Close()
	if err != nil {
		return fmt.Errorf("error writing DATA: %s", err)
	}
	return nil
}

func (f *SMTPForwarder) forward(emails []models.IncomingMail) (rest []models.IncomingMail, err error) {
	if len(emails) == 0 {
		return nil, nil
	}
	f.GetLogger().Info("Emails to forward", "nb", len(emails))

	var conn net.Conn
	var client *smtp.Client

	if f.Scheme == "http" {
		conn, err = net.Dial("tcp", net.JoinHostPort(f.Host, f.Port))
	} else {
		conn, err = tls.Dial("tcp", net.JoinHostPort(f.Host, f.Port), nil)
	}
	if err != nil {
		return emails, fmt.Errorf("failed to dial remote SMTP service: %s", err)
	}
	defer func() { _ = conn.Close() }()

	client, err = smtp.NewClient(conn, f.Host)
	if err != nil {
		return emails, fmt.Errorf("failed to build SMTP client: %s", err)
	}
	defer func() { _ = client.Quit() }()

	err = client.Hello("mailstats")
	if err != nil {
		return emails, fmt.Errorf("error at HELO: %s", err)
	}

	supportStartTLS, _ := client.Extension("STARTTLS")
	if supportStartTLS && f.Scheme == "smtp" {
		err := client.StartTLS(&tls.Config{ServerName: f.Host})
		if err != nil {
			return emails, fmt.Errorf("error while doing STARTTLS: %s", err)
		}
	}
	supportAuth, _ := client.Extension("AUTH")
	if supportAuth && len(f.Username) > 0 {
		err := client.Auth(sasl.NewPlainClient("", f.Username, f.Password))
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
