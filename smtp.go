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

	gosmtp "github.com/emersion/go-smtp"
	smtp "github.com/emersion/go-smtp"
	"github.com/storozhukBM/verifier"
	"github.com/urfave/cli"
	"golang.org/x/sync/errgroup"
)

type SMTPArgs struct {
	ListenAddr     string
	ListenPort     int
	MaxMessageSize int
	MaxIdle        int
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
	args.MaxMessageSize = c.Int("maxsize")
	args.MaxIdle = c.Int("maxidle")
	return args
}

type Backend struct {
	Collector Collector
	Stop      <-chan struct{}
}

func (b *Backend) Login(username, password string) (gosmtp.User, error) {
	return &User{Collector: b.Collector, Stop: b.Stop}, nil
}

func (b *Backend) AnonymousLogin() (gosmtp.User, error) {
	return &User{Collector: b.Collector, Stop: b.Stop}, nil
}

type User struct {
	Collector Collector
	Stop      <-chan struct{}
}

func (u *User) Send(from string, to []string, r io.Reader) error {
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
	return nil
}

func SMTP(c *cli.Context) error {
	var args SMTPArgs
	args.Populate(c)
	err := args.Verify()
	if err != nil {
		return cli.NewExitError(err.Error(), 1)
	}
	queueSize := c.GlobalInt("queuesize")
	if queueSize <= 0 {
		queueSize = 10000
	}

	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, syscall.SIGINT, syscall.SIGTERM)
	gctx, cancel := context.WithCancel(context.Background())
	go func() {
		for range sigchan {
			cancel()
		}
	}()

	g, ctx := errgroup.WithContext(gctx)
	collector := NewChanCollector(queueSize)
	g.Go(func() error {
		return Consume(ctx, collector)
	})

	b := &Backend{Collector: collector, Stop: ctx.Done()}
	s := smtp.NewServer(b)

	s.Addr = net.JoinHostPort(args.ListenAddr, fmt.Sprintf("%d", args.ListenPort))
	s.Domain = "localhost"
	s.MaxIdleSeconds = args.MaxIdle
	s.MaxMessageBytes = args.MaxMessageSize
	s.MaxRecipients = 0
	s.AllowInsecureAuth = true

	g.Go(func() error {
		<-ctx.Done()
		s.Close()
		return nil
	})

	g.Go(func() error {
		err := s.ListenAndServe()
		collector.Close()
		return err
	})

	g.Wait()

	return nil
}
