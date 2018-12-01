package main

import (
	"context"
	"fmt"
	"github.com/emersion/go-smtp"
	"io"
	"io/ioutil"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/inconshreveable/log15"
	"github.com/storozhukBM/verifier"
	"github.com/urfave/cli"
	"golang.org/x/sync/errgroup"
)

type SMTPArgs struct {
	ListenAddr     string
	ListenPort     int
	MaxMessageSize int
	MaxIdle        int
	Inetd          bool
}

func (args SMTPArgs) Verify() error {
	v := verifier.New()
	v.That(args.ListenPort > 0, "The listen port must be positive")
	v.That(len(args.ListenAddr) > 0, "The listen address is empty")
	v.That(args.MaxMessageSize >= 0, "The message size must be positive")
	v.That(args.MaxIdle >= 0, "The idle time must be positive")
	p := net.ParseIP(args.ListenAddr)
	v.That(p != nil, "The listen address is invalid")
	return v.GetError()
}

type Backend struct {
	Collector Collector
	Stop      <-chan struct{}
	Logger    log15.Logger
}

func (b *Backend) Login(username, password string) (smtp.User, error) {
	b.Logger.Debug("Authenticating user")
	return &User{
		Collector: b.Collector,
		Stop:      b.Stop,
		Logger:    b.Logger,
	}, nil
}

func (b *Backend) AnonymousLogin() (smtp.User, error) {
	b.Logger.Debug("Anonymous user")
	return &User{
		Collector: b.Collector,
		Stop:      b.Stop,
		Logger:    b.Logger,
	}, nil
}

type User struct {
	Collector Collector
	Stop      <-chan struct{}
	Logger    log15.Logger
}

func (u *User) Send(from string, to []string, r io.Reader) error {
	u.Logger.Info("Received SMTP message")
	b, err := ioutil.ReadAll(r)
	if err != nil {
		return err
	}
	infos := new(IncomingMail)
	infos.MailFrom = from
	infos.RcptTo = to
	infos.Data = b
	infos.TimeReported = time.Now()
	infos.UID = NewULID()
	return u.Collector.Push(u.Stop, infos)
}

func (u *User) Logout() error {
	u.Logger.Debug("User logged out")
	return nil
}

func SMTP(c *cli.Context) error {
	args, err := GetArgs(c)
	if err != nil {
		return err
	}
	logger := args.Logging.Build()
	collector, err := NewCollector(args, logger)
	if err != nil {
		return cli.NewExitError(fmt.Sprintf("Failed to build collector: %s", err), 3)
	}

	forwarder, err := args.Forward.Build(logger)
	if err != nil {
		return cli.NewExitError(fmt.Sprintf("Failed to build forwarder: %s", err), 3)
	}
	consumer, err := MakeConsumer(*args)
	if err != nil {
		return cli.NewExitError(fmt.Sprintf("Failed to build consumer: %s", err), 3)
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

	b := &Backend{Collector: collector, Stop: ctx.Done(), Logger: logger}
	s := smtp.NewServer(b)

	s.Domain = "localhost"
	s.MaxIdleSeconds = args.SMTP.MaxIdle
	s.MaxMessageBytes = args.SMTP.MaxMessageSize
	s.MaxRecipients = 0
	s.AllowInsecureAuth = true

	parser := NewParser(logger)

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
			err := ParseMails(ctx, collector, parser, consumer, forwarder, logger)
			_ = consumer.Close()
			_ = forwarder.Close()
			_ = parser.Close()
			return err
		})
		logger.Debug("Starting SMTP service as inetd")
		l := NewStdinListener()
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
	listener = WrapListener(listener, "SMTP", logger)

	g.Go(func() error {
		err := StartHTTP(ctx, args.HTTP, collector, logger)
		logger.Info("StartHTTP has returned", "error", err)
		return err
	})

	g.Go(func() error {
		err := ParseMails(ctx, collector, parser, consumer, forwarder, logger)
		_ = consumer.Close()
		_ = forwarder.Close()
		_ = parser.Close()
		logger.Info("ParseMails has returned", "error", err)
		return err
	})

	g.Go(func() error {
		logger.Debug("Starting SMTP service")
		err := s.Serve(listener)
		logger.Debug("Stopped SMTP service")
		_ = collector.Close()
		return err
	})

	return g.Wait()
}
