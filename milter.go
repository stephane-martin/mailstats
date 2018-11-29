package main

import (
	"context"
	"fmt"
	"net"
	"net/textproto"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/phalaaxx/milter"
	"github.com/storozhukBM/verifier"
	"github.com/urfave/cli"
	"golang.org/x/sync/errgroup"
)

type MilterArgs struct {
	ListenAddr string
	ListenPort int
	Inetd      bool
}

func (args MilterArgs) Verify() error {
	v := verifier.New()
	v.That(args.ListenPort > 0, "The listen port must be positive")
	v.That(len(args.ListenAddr) > 0, "The listen address is empty")
	p := net.ParseIP(args.ListenAddr)
	v.That(p != nil, "The listen address is invalid")
	return v.GetError()
}

func (args *MilterArgs) Populate(c *cli.Context) *MilterArgs {
	if args == nil {
		args = new(MilterArgs)
	}
	args.ListenPort = c.Int("lport")
	args.ListenAddr = strings.TrimSpace(c.String("laddr"))
	args.Inetd = c.GlobalBool("inetd")
	return args
}

type milterMessage struct {
	host    string
	family  string
	port    int
	addr    string
	helo    string
	from    string
	to      []string
	builder strings.Builder
}

func (m *milterMessage) Reset() {
	m.host = ""
	m.family = ""
	m.port = 0
	m.addr = ""
	m.helo = ""
	m.from = ""
	m.to = make([]string, 0)
	m.builder.Reset()
}

func (m *milterMessage) make() *IncomingMail {
	info := new(IncomingMail)
	info.Host = m.host
	info.Family = m.family
	info.Port = m.port
	info.Addr = m.addr
	info.Helo = m.helo
	info.MailFrom = m.from
	info.RcptTo = append(info.RcptTo, m.to...)
	info.Data = m.builder.String()
	info.TimeReported = time.Now()
	m.Reset()
	return info
}

type StatsMilter struct {
	message   milterMessage
	collector Collector
	stop      <-chan struct{}
}

func (e *StatsMilter) Helo(name string, m *milter.Modifier) (milter.Response, error) {
	e.message.helo = name
	return milter.RespContinue, nil
}

func (e *StatsMilter) Connect(host string, family string, port uint16, addr net.IP, m *milter.Modifier) (milter.Response, error) {
	e.message.Reset()
	e.message.host = host
	e.message.family = family
	e.message.port = int(port)
	if len(addr) > 0 {
		e.message.addr = addr.String()
	} else {
		e.message.addr = ""
	}
	return milter.RespContinue, nil
}

func (e *StatsMilter) MailFrom(from string, m *milter.Modifier) (milter.Response, error) {
	e.message.from = from
	return milter.RespContinue, nil
}

func (e *StatsMilter) RcptTo(rcptTo string, m *milter.Modifier) (milter.Response, error) {
	e.message.to = append(e.message.to, rcptTo)
	return milter.RespContinue, nil
}

func (e *StatsMilter) Header(name, value string, m *milter.Modifier) (milter.Response, error) {
	return milter.RespContinue, nil
}

func (e *StatsMilter) Headers(headers textproto.MIMEHeader, m *milter.Modifier) (milter.Response, error) {
	for k, vl := range headers {
		for _, v := range vl {
			_, _ = fmt.Fprintf(&e.message.builder, "%s: %s\n", k, v)
		}
	}
	return milter.RespContinue, nil
}

func (e *StatsMilter) BodyChunk(chunk []byte, m *milter.Modifier) (milter.Response, error) {
	e.message.builder.Write(chunk)
	return milter.RespContinue, nil
}

func (e *StatsMilter) Body(m *milter.Modifier) (milter.Response, error) {
	err := e.collector.Push(e.stop, e.message.make())
	if err == nil {
		return milter.RespAccept, nil
	}
	return milter.RespTempFail, err
}

func Milter(c *cli.Context) error {
	args, err := GetArgs(c)
	if err != nil {
		return err
	}

	var httpArgs HTTPArgs
	httpArgs.Populate(c)
	err = httpArgs.Verify()
	if err != nil {
		return cli.NewExitError(err.Error(), 1)
	}

	logger := args.Logging.Build()

	consumer, err := MakeConsumer(*args)
	if err != nil {
		return cli.NewExitError(fmt.Sprintf("Failed to build consumer: %s", err), 3)
	}
	forwarder, err  := args.Forward.Build(logger)
	if err != nil {
		return cli.NewExitError(fmt.Sprintf("Failed to build forwarder: %s", err), 3)
	}

	listener, err := net.Listen(
		"tcp",
		net.JoinHostPort(
			args.Milter.ListenAddr,
			fmt.Sprintf("%d", args.Milter.ListenPort),
		),
	)
	if err != nil {
		return cli.NewExitError(fmt.Sprintf("Listen() has failed: %s", err), 2)
	}
	listener = WrapListener(listener, "milter", logger)

	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, syscall.SIGINT, syscall.SIGTERM)
	gctx, cancel := context.WithCancel(context.Background())
	go func() {
		for sig := range sigchan {
			logger.Debug("Signal received", "signal", sig.String())
			cancel()
		}
	}()

	parser := NewParser(logger)

	g, ctx := errgroup.WithContext(gctx)
	collector := NewChanCollector(args.QueueSize, logger)

	g.Go(func() error {
		return forwarder.Start(ctx)
	})

	g.Go(func() error {
		err := ParseMails(ctx, collector, parser, consumer, forwarder, logger)
		_ = consumer.Close()
		_ = forwarder.Close()
		_ = parser.Close()
		return err
	})

	g.Go(func() error {
		return StartHTTP(ctx, httpArgs, collector, logger)
	})

	g.Go(func() error {
		<-ctx.Done()
		_ = listener.Close()
		return nil
	})

	g.Go(func() error {
		logger.Info("Starting Milter service")
		err := milter.RunServer(listener, func() (milter.Milter, milter.OptAction, milter.OptProtocol) {
			return &StatsMilter{collector: collector, stop: ctx.Done()}, 0, 0
		})
		logger.Info("Service Milter stopped")
		_ = collector.Close()
		return err
	})

	err = g.Wait()
	if err != nil {
		logger.Debug("Milter error after Wait()", "error", err)
	}

	return nil
}
