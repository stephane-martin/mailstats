package services

import (
	"bytes"
	"context"
	"fmt"
	"github.com/stephane-martin/mailstats/arguments"
	"github.com/stephane-martin/mailstats/collectors"
	"github.com/stephane-martin/mailstats/consumers"
	"github.com/stephane-martin/mailstats/forwarders"
	"github.com/stephane-martin/mailstats/models"
	"github.com/stephane-martin/mailstats/parser"
	"github.com/stephane-martin/mailstats/utils"
	"net"
	"net/textproto"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/phalaaxx/milter"
	"github.com/urfave/cli"
	"golang.org/x/sync/errgroup"
)



type milterMessage struct {
	host    string
	family  string
	port    int
	addr    string
	helo    string
	from    string
	to      []string
	builder bytes.Buffer
}

func (m *milterMessage) Reset() {
	m.host = ""
	m.family = ""
	m.port = 0
	m.addr = ""
	m.helo = ""
	m.from = ""
	m.to = make([]string, 0)
	m.builder = bytes.Buffer{}
}

func (m *milterMessage) make() *models.IncomingMail {
	info := &models.IncomingMail{
		BaseInfos: models.BaseInfos{
			Host: m.host,
			Family: m.family,
			Port: m.port,
			Addr: m.addr,
			MailFrom: m.from,
			RcptTo: m.to,
			TimeReported: time.Now(),
		},
		Data: m.builder.Bytes(),
	}
	m.Reset()
	return info
}

type StatsMilter struct {
	message   milterMessage
	Collector collectors.Collector
	Forwarder forwarders.Forwarder
	stop      <-chan struct{}
}

func newStatsMilter(ctx context.Context, collector collectors.Collector, forwarder forwarders.Forwarder) *StatsMilter {
	return &StatsMilter{
		Collector: collector,
		Forwarder: forwarder,
		stop: ctx.Done(),
	}
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
	incoming := e.message.make()
	err := collectors.CollectAndForward(e.stop, incoming, e.Collector, e.Forwarder)
	if err == nil {
		return milter.RespAccept, nil
	}
	return milter.RespTempFail, err
}

func MilterAction(c *cli.Context) error {
	args, err := arguments.GetArgs(c)
	if err != nil {
		return err
	}

	logger := args.Logging.Build()

	consumer, err := consumers.MakeConsumer(*args, logger)
	if err != nil {
		return cli.NewExitError(fmt.Sprintf("Failed to build consumer: %s", err), 3)
	}
	forwarder, err := forwarders.Build(args.Forward, logger)
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
	listener = utils.WrapListener(listener, "milter", logger)

	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, syscall.SIGINT, syscall.SIGTERM)
	gctx, cancel := context.WithCancel(context.Background())
	go func() {
		for sig := range sigchan {
			logger.Debug("Signal received", "signal", sig.String())
			cancel()
		}
	}()

	theparser := parser.NewParser(logger)
	collector, err := collectors.NewCollector(*args, logger)
	if err != nil {
		return cli.NewExitError(fmt.Sprintf("Failed to build collector: %s", err), 3)
	}
	var collG errgroup.Group
	collG.Go(func() error {
		return collector.Start()
	})

	g, ctx := errgroup.WithContext(gctx)

	g.Go(func() error {
		return forwarder.Start(ctx)
	})

	g.Go(func() error {
		return parser.ParseMails(ctx, collector, theparser, consumer, args.NbParsers, logger)
	})

	g.Go(func() error {
		err := StartHTTP(ctx, args.HTTP, collector, forwarder, logger)
		logger.Debug("StartHTTP has returned", "error", err)
		return err
	})

	if args.Secret != nil {
		g.Go(func() error {
			err := StartMaster(ctx, args.HTTP, args.Secret, collector, consumer, logger)
			logger.Debug("StartMaster has returned", "error", err)
			return err
		})
	}
	g.Go(func() error {
		<-ctx.Done()
		_ = listener.Close()
		return nil
	})

	g.Go(func() error {
		logger.Info("Starting Milter service")
		err := milter.RunServer(listener, func() (milter.Milter, milter.OptAction, milter.OptProtocol) {
			return newStatsMilter(ctx, collector, forwarder), 0, 0
		})
		logger.Info("Service Milter stopped")
		return err
	})

	err = g.Wait()
	_ = collector.Close()
	_ = theparser.Close()
	_ = forwarder.Close()
	_ = consumer.Close()
	_ = collG.Wait()
	if err != nil {
		logger.Debug("Milter error after Wait()", "error", err)
	}

	return nil
}
