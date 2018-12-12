package utils

import (
	"errors"
	"net"
	"os"
	"sync"
)

type StdinListener struct {
	connectionOnce sync.Once
	closeOnce      sync.Once
	connChan       chan net.Conn
}

func NewStdinListener() net.Listener {
	l := new(StdinListener)
	l.connChan = make(chan net.Conn, 1)
	return l
}

type stdinConn struct {
	net.Conn
	l net.Listener
}

func (c stdinConn) Close() (err error) {
	err = c.Conn.Close()
	c.l.Close()
	return err
}

func (l *StdinListener) Accept() (net.Conn, error) {
	l.connectionOnce.Do(func() {
		conn, err := net.FileConn(os.Stdin)
		if err == nil {
			l.connChan <- stdinConn{Conn: conn, l: l}
			os.Stdin.Close()
		} else {
			l.Close()
		}
	})
	conn, ok := <-l.connChan
	if ok {
		return conn, nil
	}
	return nil, errors.New("Closed")
}

func (l *StdinListener) Close() error {
	l.closeOnce.Do(func() { close(l.connChan) })
	return nil
}

func (l *StdinListener) Addr() net.Addr {
	return nil
}
