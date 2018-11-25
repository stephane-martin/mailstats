package main

import (
	"errors"
	"github.com/storozhukBM/verifier"
	"github.com/urfave/cli"
	"io"
	"os"
	"strings"
	"sync"
)

type Consumer interface {
	Consume(infos string) error
	Close() error
}

type ConsumerType int

const (
	Stdout ConsumerType = iota
	Stderr
	File
	Redis
)

var ConsumerTypes = map[string]ConsumerType{
	"stdout": Stdout,
	"stderr": Stderr,
	"file": File,
	"redis": Redis,
}

type ConsumerArgs struct {
	Type string
	OutFile string
}

func (args ConsumerArgs) GetType() ConsumerType {
	return ConsumerTypes[args.Type]
}

func (args ConsumerArgs) Verify() error {
	types := []string{"stdout", "stderr", "file", "redis", "syslog"}
	v := verifier.New()
	have := false
	for _, t := range types {
		if t == args.Type {
			have = true
			break
		}
	}
	v.That(have, "consumer type unknown")
	v.That(len(args.OutFile) > 0, "the output filename is empty")
	return v.GetError()
}

func (args *ConsumerArgs) Populate(c *cli.Context) *ConsumerArgs {
	if args == nil {
		args = new(ConsumerArgs)
	}
	args.Type = strings.ToLower(strings.TrimSpace(c.GlobalString("out")))
	args.OutFile = strings.ToLower(strings.TrimSpace(c.GlobalString("outfile")))
	return args
}

func MakeConsumer(args Args) (Consumer, error) {
	// TODO: HTTP
	switch args.Consumer.GetType() {
	case Stdout:
		return StdoutConsumer, nil
	case Stderr:
		return StderrConsumer, nil
	case File:
		return NewFileConsumer(args.Consumer)
	case Redis:
		return NewRedisConsumer(args.Redis)
	default:
		return nil, errors.New("unknown consumer type")
	}
}


var printLock sync.Mutex

type Writer struct {
	io.WriteCloser
}

func (w Writer) Consume(infos string) error {
	printLock.Lock()
	_, err := io.WriteString(w.WriteCloser, infos)
	printLock.Unlock()
	return err
}


var StdoutConsumer Consumer = Writer{WriteCloser: os.Stdout}
var StderrConsumer Consumer = Writer{WriteCloser: os.Stderr}

func NewFileConsumer(args ConsumerArgs) (Consumer, error) {
	f, err := os.OpenFile(args.OutFile, os.O_APPEND | os.O_CREATE | os.O_WRONLY, 0664)
	if err != nil {
		return nil, err
	}
	return Writer{WriteCloser: f}, nil
}
