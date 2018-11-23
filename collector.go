package main

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/storozhukBM/verifier"
	"github.com/urfave/cli"
	"io"
	"os"
	"runtime"
	"strings"
	"sync"

	"github.com/inconshreveable/log15"
	"golang.org/x/sync/errgroup"
)

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
	v.That(have, "out type unknown")
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

type Collector interface {
	Push(stop <-chan struct{}, info *Infos) error
	PushCtx(ctx context.Context, info *Infos) error
	Pull(stop <-chan struct{}) (*Infos, error)
	PullCtx(ctx context.Context) (*Infos, error)
	Close() error
}

type Consumer interface {
	Consume(infos string) error
	Close() error
}

type ChanCollector struct {
	ch     chan *Infos
	once   sync.Once
	logger log15.Logger
}

func NewChanCollector(size int, logger log15.Logger) *ChanCollector {
	c := new(ChanCollector)
	c.logger = logger
	if size <= 0 {
		c.ch = make(chan *Infos)
	} else {
		c.ch = make(chan *Infos, size)
	}
	return c
}

func (c *ChanCollector) Push(stop <-chan struct{}, info *Infos) error {
	select {
	case c.ch <- info:
		c.logger.Debug("New message pushed to collector")
		return nil
	case <-stop:
		return context.Canceled
	}
}

func (c *ChanCollector) PushCtx(ctx context.Context, info *Infos) error {
	return c.Push(ctx.Done(), info)
}

func (c *ChanCollector) Pull(stop <-chan struct{}) (*Infos, error) {
	select {
	case info, ok := <-c.ch:
		if ok {
			c.logger.Debug("New message pulled from collector")
			return info, nil
		}
		return nil, context.Canceled
	case <-stop:
		return nil, context.Canceled
	}
}

func (c *ChanCollector) PullCtx(ctx context.Context) (*Infos, error) {
	return c.Pull(ctx.Done())
}

func (c *ChanCollector) Close() error {
	c.once.Do(func() {
		c.logger.Debug("Closing collector")
		close(c.ch)
	})
	return nil
}

func ParseMessages(ctx context.Context, collector Collector, consumer Consumer, logger log15.Logger) error {
	var g errgroup.Group
	cpus := runtime.NumCPU()
	for i := 0; i < cpus; i++ {
		g.Go(func() error {
			for {
				info, err := collector.PullCtx(ctx)
				if info == nil {
					return err
				}
				parsed, err := info.Parse(logger)
				if err != nil {
					logger.Info("Failed to parse message", "error", err)
					continue
				}
				b, err := json.Marshal(parsed)
				if err != nil {
					logger.Error("Failed to marshal message information", "error", err)
					continue
				}
				err = consumer.Consume(string(b))
				if err != nil {
					logger.Error("Failed to consume parsing results", "error", err)
					continue
				}
				logger.Info("Parsing results sent to consumer")

			}
		})
	}
	err := g.Wait()
	_ = consumer.Close()
	return err
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
