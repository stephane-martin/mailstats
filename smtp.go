package main

import (
	"context"
	"fmt"
	"github.com/stephane-martin/mailstats/arguments"
	"github.com/stephane-martin/mailstats/collectors"
	"github.com/stephane-martin/mailstats/consumers"
	"github.com/stephane-martin/mailstats/forwarders"
	"github.com/stephane-martin/mailstats/models"
	"github.com/stephane-martin/mailstats/utils"
	"io"
	"io/ioutil"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/emersion/go-smtp"

	"github.com/inconshreveable/log15"
	"github.com/urfave/cli"
	"golang.org/x/sync/errgroup"
)

type Backend struct {
	Collector collectors.Collector
	Stop      <-chan struct{}
	Logger    log15.Logger
	Port      int
}

func (b *Backend) Login(username, password string) (smtp.User, error) {
	b.Logger.Debug("Authenticating user")
	return &User{
		Collector: b.Collector,
		Stop:      b.Stop,
		Logger:    b.Logger,
		Port:      b.Port,
	}, nil
}

func (b *Backend) AnonymousLogin() (smtp.User, error) {
	b.Logger.Debug("Anonymous user")
	return &User{
		Collector: b.Collector,
		Stop:      b.Stop,
		Logger:    b.Logger,
		Port:      b.Port,
	}, nil
}

type User struct {
	Collector collectors.Collector
	Stop      <-chan struct{}
	Logger    log15.Logger
	Port      int
}

func (u *User) Send(from string, to []string, r io.Reader) error {
	u.Logger.Info("Received SMTP message")
	b, err := ioutil.ReadAll(r)
	if err != nil {
		return err
	}
	m := &models.IncomingMail{
		BaseInfos: models.BaseInfos{
			MailFrom:     from,
			RcptTo:       to,
			TimeReported: time.Now(),
			Port: u.Port,
		},
		Data: b,
	}
	return u.Collector.Push(u.Stop, m)
}

func (u *User) Logout() error {
	u.Logger.Debug("User logged out")
	return nil
}

func SMTPAction(c *cli.Context) error {
	args, err := arguments.GetArgs(c)
	if err != nil {
		return err
	}
	logger := args.Logging.Build()
	collector, err := collectors.NewCollector(*args, logger)
	if err != nil {
		return cli.NewExitError(fmt.Sprintf("Failed to build collector: %s", err), 3)
	}

	forwarder, err := forwarders.Build(args.Forward, logger)
	if err != nil {
		return cli.NewExitError(fmt.Sprintf("Failed to build forwarder: %s", err), 3)
	}
	consumer, err := consumers.MakeConsumer(*args, logger)
	if err != nil {
		return cli.NewExitError(fmt.Sprintf("Failed to build consumer: %s", err), 3)
	}

	if args.GeoIP.Enabled {
		err := utils.InitGeoIP(args.GeoIP.DatabasePath)
		if err != nil {
			return cli.NewExitError(fmt.Sprintf("Error loading GeoIP database: %s", err), 1)
		}
		//noinspection GoUnhandledErrorResult
		defer utils.CloseGeoIP()
	}

	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, syscall.SIGINT, syscall.SIGTERM)
	gctx, cancel := context.WithCancel(context.Background())

	go func() {
		for sig := range sigchan {
			logger.Info("Signal received", "signal", sig.String())
			cancel()
		}
	}()

	g, ctx := errgroup.WithContext(gctx)

	b := &Backend{Collector: collector, Stop: ctx.Done(), Logger: logger, Port: args.SMTP.ListenPort}
	s := smtp.NewServer(b)

	s.Domain = "localhost"
	s.MaxIdleSeconds = args.SMTP.MaxIdle
	s.MaxMessageBytes = args.SMTP.MaxMessageSize
	s.MaxRecipients = 0
	s.AllowInsecureAuth = true

	parser := NewParser(logger)

	var collG errgroup.Group
	collG.Go(func() error {
		return collector.Start()
	})

	g.Go(func() error {
		err := forwarder.Start(ctx)
		logger.Info("forwarder has returned", "error", err)
		return err
	})

	g.Go(func() error {
		defer func() {
			// in case the s.Close() is called whereas s does not have yet a Listener
			recover()
		}()
		<-ctx.Done()
		s.Close()
		return nil
	})

	if args.SMTP.Inetd {
		g.Go(func() error {
			err := ParseMails(ctx, collector, parser, consumer, forwarder, args.NbParsers, logger)
			_ = consumer.Close()
			_ = forwarder.Close()
			_ = parser.Close()
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

	var listener net.Listener
	s.Addr = net.JoinHostPort(args.SMTP.ListenAddr, fmt.Sprintf("%d", args.SMTP.ListenPort))
	listener, err = net.Listen("tcp", s.Addr)
	if err != nil {
		cancel()
		return cli.NewExitError(fmt.Sprintf("Listen() has failed: %s", err), 2)
	}
	listener = utils.WrapListener(listener, "SMTP", logger)

	g.Go(func() error {
		err := StartHTTP(ctx, args.HTTP, args.Secret, collector, consumer, logger)
		logger.Info("StartHTTP has returned", "error", err)
		return err
	})

	g.Go(func() error {
		err := ParseMails(ctx, collector, parser, consumer, forwarder, args.NbParsers, logger)
		logger.Info("ParseMails has returned", "error", err)
		return err
	})

	g.Go(func() error {
		logger.Debug("Starting SMTP service")
		err := s.Serve(listener)
		logger.Debug("Stopped SMTP service")
		return err
	})

	err = g.Wait()
	_ = collector.Close()
	_ = parser.Close()
	_ = forwarder.Close()
	_ = consumer.Close()
	_ = collG.Wait()
	if err != nil {
		logger.Debug("SMTP error after Wait()", "error", err)
	}
	return nil
}
