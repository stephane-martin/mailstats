package main

import (
	"encoding/json"
	"errors"
	"github.com/storozhukBM/verifier"
	"github.com/urfave/cli"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

type Consumer interface {
	Consume(features *FeaturesMail) error
	Close() error
}

type ConsumerType int

const (
	Stdout ConsumerType = iota
	Stderr
	File
	Redis
	HTTP
)

var ConsumerTypes = map[string]ConsumerType{
	"stdout": Stdout,
	"stderr": Stderr,
	"file": File,
	"redis": Redis,
	"http": HTTP,
}

type ConsumerArgs struct {
	Type string
	OutFile string
	OutURL string
}

func (args ConsumerArgs) GetURL() string {
	u, _ := url.Parse(args.OutURL)
	return u.String()
}

func (args ConsumerArgs) GetType() ConsumerType {
	return ConsumerTypes[args.Type]
}

func (args ConsumerArgs) Verify() error {
	v := verifier.New()
	_, ok := ConsumerTypes[args.Type]
	v.That(ok, "consumer type unknown")
	v.That(len(args.OutFile) > 0, "the output filename is empty")
	_, err := url.Parse(args.OutURL)
	v.That(err == nil, "invalid out URL: %s", err)
	return v.GetError()
}

func (args *ConsumerArgs) Populate(c *cli.Context) *ConsumerArgs {
	if args == nil {
		args = new(ConsumerArgs)
	}
	args.Type = strings.ToLower(strings.TrimSpace(c.GlobalString("out")))
	args.OutFile = strings.ToLower(strings.TrimSpace(c.GlobalString("outfile")))
	args.OutURL = strings.ToLower(strings.TrimSpace(c.GlobalString("outurl")))
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
	case HTTP:
		return NewHTTPConsumer(args.Consumer)
	default:
		return nil, errors.New("unknown consumer type")
	}
}


type HTTPConsumer struct {
	client *http.Client
	url string
}

func NewHTTPConsumer(args ConsumerArgs) (Consumer, error) {
	tr := &http.Transport{
		DisableCompression: true,
		MaxIdleConns: 16,
		MaxIdleConnsPerHost: 8,
		IdleConnTimeout: 30 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 2 * time.Second,
		Proxy: nil,
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
			DualStack: true,
		}).DialContext,
	}
	return &HTTPConsumer{
		client: &http.Client{Transport: tr},
		url: args.GetURL(),
	}, nil
}

func (c *HTTPConsumer) Consume(features *FeaturesMail) error {
	r, w := io.Pipe()
	go func() {
		err := json.NewEncoder(w).Encode(features)
		_ = w.CloseWithError(err)
	}()
	_, err := c.client.Post(c.url, "application/json", r)
	return err
}

func (c *HTTPConsumer) Close() error {
	return nil
}

var printLock sync.Mutex

type Writer struct {
	io.WriteCloser
}

func (w Writer) Consume(features *FeaturesMail) (err error) {
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

func (c *RedisConsumer) Consume(features *FeaturesMail) error {
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