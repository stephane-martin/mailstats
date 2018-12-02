package main

import (
	"encoding/json"
	"errors"
	"github.com/storozhukBM/verifier"
	"github.com/urfave/cli"
	"io"
	"os"
	"strings"
	"sync"
)

type Consumer interface {
	Consume(features FeaturesMail) error
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
	types := []string{"stdout", "stderr", "file", "redis"}
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

func (w Writer) Consume(features FeaturesMail) (err error) {
	printLock.Lock()
	err = json.NewEncoder(w.WriteCloser).Encode(features)
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


func NewRedisConsumer(args RedisArgs) (*RedisConsumer, error) {
	client, err := NewRedisClient(args)
	if err != nil {
		return nil, err
	}
	return &RedisConsumer{client: client, args: args}, nil
}

func (c *RedisConsumer) Consume(features FeaturesMail) error {
	b, err := json.Marshal(features)
	if err != nil {
		return err
	}
	_, err = c.client.RPush(c.args.ResultsKey, b).Result()
	return err
}

func (c *RedisConsumer) Close() error {
	return c.client.Close()
}