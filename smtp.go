package main

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/emersion/go-smtp"
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

type Backend struct {
	Collector Collector
	Stop      <-chan struct{}
	Logger    log15.Logger
}

func (b *Backend) Login(username, password string) (smtp.User, error) {
	b.Logger.Debug("Authenticating user")
	return &User{Collector: b.Collector, Stop: b.Stop, Logger: b.Logger}, nil
}

func (b *Backend) AnonymousLogin() (smtp.User, error) {
	b.Logger.Debug("Anonymous user")
	return &User{Collector: b.Collector, Stop: b.Stop, Logger: b.Logger}, nil
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
	infos := new(Infos)
	infos.MailFrom = from
	infos.RcptTo = to
	infos.Data = string(b)
	infos.TimeReported = time.Now()
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

	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, syscall.SIGINT, syscall.SIGTERM)
	gctx, cancel := context.WithCancel(context.Background())

	go func() {
		for sig := range sigchan {
			logger.Debug("Signal received", "signal", sig.String())
			cancel()
		}
	}()

	g, ctx := errgroup.WithContext(gctx)
	collector := NewChanCollector(args.QueueSize, logger)

	b := &Backend{Collector: collector, Stop: ctx.Done(), Logger: logger}
	s := smtp.NewServer(b)

	s.Domain = "localhost"
	s.MaxIdleSeconds = args.SMTP.MaxIdle
	s.MaxMessageBytes = args.SMTP.MaxMessageSize
	s.MaxRecipients = 0
	s.AllowInsecureAuth = true

	consumer, err := MakeConsumer(*args)
	if err != nil {
		return cli.NewExitError(fmt.Sprintf("Failed to build consumer: %s", err), 3)
	}

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
			return ParseMessages(ctx, collector, StdoutConsumer, logger)
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
		return StartHTTP(ctx, args.HTTP, logger)
	})

	g.Go(func() error {
		return ParseMessages(ctx, collector, consumer, logger)
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
