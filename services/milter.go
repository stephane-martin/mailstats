package services

import (
	"bytes"
	"context"
	"fmt"
	"github.com/inconshreveable/log15"
	"github.com/stephane-martin/mailstats/arguments"
	"github.com/stephane-martin/mailstats/collectors"
	"github.com/stephane-martin/mailstats/forwarders"
	"github.com/stephane-martin/mailstats/logging"
	"github.com/stephane-martin/mailstats/models"
	"github.com/stephane-martin/mailstats/parser"
	"github.com/stephane-martin/mailstats/utils"
	"go.uber.org/fx"
	"net"
	"net/textproto"
	"time"

	"github.com/phalaaxx/milter"
	"github.com/urfave/cli"
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
			Host:         m.host,
			Family:       m.family,
			Port:         m.port,
			Addr:         m.addr,
			MailFrom:     m.from,
			RcptTo:       m.to,
			TimeReported: time.Now(),
		},
		Data: m.builder.Bytes(),
	}
	m.Reset()
	return info
}

type MilterServer struct {
	Collector  collectors.Collector
	Forwarder  forwarders.Forwarder
	ListenAddr string
	ListenPort int
	Logger     log15.Logger
	Listener   net.Listener
}

func (s *MilterServer) Prestart() error {
	listener, err := net.Listen(
		"tcp",
		net.JoinHostPort(
			s.ListenAddr,
			fmt.Sprintf("%d", s.ListenPort),
		),
	)
	if err != nil {
		return fmt.Errorf("milter listen() has failed: %s", err)
	}
	s.Listener = utils.WrapListener(listener, "milter", s.Logger)
	return nil
}

func (s *MilterServer) Start(ctx context.Context) error {
	go func() {
		<-ctx.Done()
		_ = s.Listener.Close()
	}()
	return milter.RunServer(s.Listener, func() (milter.Milter, milter.OptAction, milter.OptProtocol) {
		return NewMilterImpl(s.Collector, s.Forwarder), 0, 0
	})
}

func (s *MilterServer) Stop(context.Context) error {
	if s.Listener != nil {
		return s.Listener.Close()
	}
	return nil
}

func (s *MilterServer) Name() string { return "MilterServer"}

func NewMilterServer(args *arguments.Args, collector collectors.Collector, forwarder forwarders.Forwarder, logger log15.Logger) *MilterServer {
	return &MilterServer{
		Collector: collector,
		Logger: logger,
		Forwarder: forwarder,
		ListenAddr: args.Milter.ListenAddr,
		ListenPort: args.Milter.ListenPort,
	}
}

var MilterService = fx.Provide(func(lc fx.Lifecycle, args *arguments.Args, collector collectors.Collector, forwarder forwarders.Forwarder, p parser.Parser, logger log15.Logger) *MilterServer {
	s := NewMilterServer(args, collector, forwarder, logger)
	if lc != nil {
		utils.Append(lc, s, logger)
	}
	return s
})

type MilterImpl struct {
	message   milterMessage
	Collector collectors.Collector
	Forwarder forwarders.Forwarder
}

func NewMilterImpl(collector collectors.Collector, forwarder forwarders.Forwarder) *MilterImpl {
	m := MilterImpl{
		Collector: collector,
		Forwarder: forwarder,
	}
	return &m
}

func (e *MilterImpl) Helo(name string, m *milter.Modifier) (milter.Response, error) {
	e.message.helo = name
	return milter.RespContinue, nil
}

func (e *MilterImpl) Connect(host string, family string, port uint16, addr net.IP, m *milter.Modifier) (milter.Response, error) {
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

func (e *MilterImpl) MailFrom(from string, m *milter.Modifier) (milter.Response, error) {
	e.message.from = from
	return milter.RespContinue, nil
}

func (e *MilterImpl) RcptTo(rcptTo string, m *milter.Modifier) (milter.Response, error) {
	e.message.to = append(e.message.to, rcptTo)
	return milter.RespContinue, nil
}

func (e *MilterImpl) Header(name, value string, m *milter.Modifier) (milter.Response, error) {
	return milter.RespContinue, nil
}

func (e *MilterImpl) Headers(headers textproto.MIMEHeader, m *milter.Modifier) (milter.Response, error) {
	for k, vl := range headers {
		for _, v := range vl {
			_, _ = fmt.Fprintf(&e.message.builder, "%s: %s\n", k, v)
		}
	}
	return milter.RespContinue, nil
}

func (e *MilterImpl) BodyChunk(chunk []byte, m *milter.Modifier) (milter.Response, error) {
	e.message.builder.Write(chunk)
	return milter.RespContinue, nil
}

func (e *MilterImpl) Body(m *milter.Modifier) (milter.Response, error) {
	incoming := e.message.make()
	err := collectors.CollectAndForward(context.Background().Done(), incoming, e.Collector, e.Forwarder)
	if err == nil {
		return milter.RespAccept, nil
	}
	return milter.RespTempFail, err
}

func MilterAction(c *cli.Context) error {
	args, err := arguments.GetArgs(c)
	if err != nil {
		err = fmt.Errorf("error validating cli arguments: %s", err)
		return cli.NewExitError(err.Error(), 1)
	}

	logger := logging.NewLogger(args)
	withRedis := args.RedisRequired()
	invoke := fx.Invoke(func(h *HTTPServer, m *HTTPMasterServer, s *MilterServer) {
		// bootstrap the application
	})
	app := Builder(c, args, invoke, withRedis, logger)
	app.Run()
	return nil
}
