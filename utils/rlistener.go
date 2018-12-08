package utils

import (
	"github.com/inconshreveable/log15"
	"github.com/stephane-martin/mailstats/metrics"
	"net"
)

func WrapListener(l net.Listener, stype string, logger log15.Logger) net.Listener {
	return RListener{Listener: l, stype: stype, logger: logger}
}

type RListener struct {
	net.Listener
	logger log15.Logger
	stype string
}

func (l RListener) Accept() (net.Conn, error) {
	c, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	h, _, _ := net.SplitHostPort(c.RemoteAddr().String())
	l.logger.Info("New connection", "client", h, "service", l.stype)
	metrics.M().Connections.WithLabelValues(h, l.stype).Inc()
	return RConn{Conn: c, logger: l.logger, stype: l.stype}, nil
}

func (l RListener) Close() error {
	l.logger.Info("Listener closed")
	return l.Listener.Close()
}

type RConn struct {
	net.Conn
	logger log15.Logger
	stype string
}

func (c RConn) Close() error {
	c.logger.Info("Connection closed", "service", c.stype)
	return c.Conn.Close()
}
