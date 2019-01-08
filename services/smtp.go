package services

import (
	"context"
	"fmt"
	"github.com/stephane-martin/mailstats/arguments"
	"github.com/stephane-martin/mailstats/collectors"
	"github.com/stephane-martin/mailstats/forwarders"
	"github.com/stephane-martin/mailstats/logging"
	"github.com/stephane-martin/mailstats/models"
	"github.com/stephane-martin/mailstats/parser"
	"github.com/stephane-martin/mailstats/utils"
	"go.uber.org/fx"
	"io"
	"io/ioutil"
	"net"
	"time"

	"github.com/emersion/go-smtp"

	"github.com/inconshreveable/log15"
	"github.com/urfave/cli"
)

type Backend struct {
	Port      int
	Collector collectors.Collector
	Forwarder forwarders.Forwarder
	Logger    log15.Logger
}

func NewSMTPBackend(args *arguments.Args, collector collectors.Collector, forwarder forwarders.Forwarder, logger log15.Logger) smtp.Backend {
	return &Backend{
		Port:      args.SMTP.ListenPort,
		Collector: collector,
		Forwarder: forwarder,
		Logger:    logger,
	}
}

func (b *Backend) Login(username, password string) (smtp.User, error) {
	b.Logger.Debug("Authenticated user")
	return &User{
		Port:      b.Port,
		Collector: b.Collector,
		Forwarder: b.Forwarder,
		Logger:    b.Logger,
	}, nil
}

func (b *Backend) AnonymousLogin() (smtp.User, error) {
	b.Logger.Debug("Anonymous user")
	return &User{
		Port:      b.Port,
		Collector: b.Collector,
		Forwarder: b.Forwarder,
		Logger:    b.Logger,
	}, nil
}

type User struct {
	Port      int
	Collector collectors.Collector
	Forwarder forwarders.Forwarder
	Logger    log15.Logger
	Stop      <-chan struct{}
}

func (u *User) Send(from string, to []string, r io.Reader) error {
	u.Logger.Info("Received SMTP message")
	b, err := ioutil.ReadAll(r)
	if err != nil {
		return err
	}
	incoming := &models.IncomingMail{
		BaseInfos: models.BaseInfos{
			MailFrom:     from,
			RcptTo:       to,
			TimeReported: time.Now(),
			Port:         u.Port,
		},
		Data: b,
	}
	return collectors.CollectAndForward(context.Background().Done(), incoming, u.Collector, u.Forwarder)
}

func (u *User) Logout() error {
	u.Logger.Debug("User logged out")
	return nil
}

type SMTPServer struct {
	*smtp.Server
	listener net.Listener
	logger log15.Logger
	addr string
}

func (s *SMTPServer) Name() string { return "SMTPServer" }

func (s *SMTPServer) Prestart() error {
	l, err := net.Listen("tcp", s.Addr)
	if err != nil {
		return fmt.Errorf("SMTP Listen() has failed: %s", err)
	}
	s.listener = utils.WrapListener(l, "SMTP", s.logger)
	return nil
}


func (s *SMTPServer) Start(ctx context.Context) error {
	s.logger.Debug("Start SMTP service")
	go func() {
		<-ctx.Done()
		s.Server.Close()
	}()
	return s.Server.Serve(s.listener)
}

func NewSMTPService(args *arguments.Args, backend smtp.Backend, logger log15.Logger) *SMTPServer {
	s := smtp.NewServer(backend)
	s.Domain = "localhost"
	s.MaxIdleSeconds = args.SMTP.MaxIdle
	s.MaxMessageBytes = args.SMTP.MaxMessageSize
	s.MaxRecipients = 0
	s.AllowInsecureAuth = true
	return &SMTPServer{
		Server: s,
		addr: net.JoinHostPort(args.SMTP.ListenAddr, fmt.Sprintf("%d", args.SMTP.ListenPort)),
		logger: logger,
	}
}

var SMTPService = fx.Provide(func(lc fx.Lifecycle, args *arguments.Args, backend smtp.Backend, p parser.Parser, logger log15.Logger) *SMTPServer {
	s := NewSMTPService(args, backend, logger)
	utils.Append(lc, s, logger)
	return s
})

func SMTPAction(c *cli.Context) error {
	args, err := arguments.GetArgs(c)
	if err != nil {
		err = fmt.Errorf("error validating cli arguments: %s", err)
		return cli.NewExitError(err.Error(), 1)
	}

	logger := logging.NewLogger(args)
	invoke := fx.Invoke(func(h *HTTPServer, m *HTTPMasterServer, s *SMTPServer) {
		// bootstrap the application
	})
	app := Builder(c, args, invoke, logger)
	app.Run()
	return nil

	/*
	if args.SMTP.Inetd {
		g.Go(func() error {
			err := parser.ParseMails(ctx, collector, theparser, consumer, args.NbParsers, logger)
			_ = consumer.Close()
			_ = forwarder.Close()
			_ = theparser.Close()
			return err
		})
		logger.Debug("Starting SMTP service as inetd")
		l := utils.NewStdinListener()
		g.Go(func() error {
			err := s.Serve(l)
			logger.Debug("Stopped SMTP service as inetd")
			_ = collector.Close()
			return err
		})
		return g.Wait()
	}
	*/

}
