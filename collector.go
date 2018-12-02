package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/go-redis/redis"
	"github.com/tinylib/msgp/msgp"
	"strings"
	"sync"
	"time"

	"github.com/inconshreveable/log15"
	"github.com/pierrec/lz4"
)

type Collector interface {
	Push(stop <-chan struct{}, info *IncomingMail) error
	PushCtx(ctx context.Context, info *IncomingMail) error
	Pull(stop <-chan struct{}) (*IncomingMail, error)
	PullCtx(ctx context.Context) (*IncomingMail, error)
	Close() error
}

func NewCollector(args *Args, logger log15.Logger) (Collector, error) {
	logger.Debug("Collector", "type", args.Collector)
	switch args.Collector {
	case "channel":
		return NewChanCollector(args.CollectorSize, logger)
	case "filesystem":
		return NewFSCollector(args.CollectorDir, logger)
	case "redis":
		return NewRedisCollector(args, logger)
	default:
		return nil, errors.New("unknown collector")
	}
}

type RedisCollector struct {
	logger log15.Logger
	client *redis.Client
	key    string
}

func NewRedisCollector(args *Args, logger log15.Logger) (*RedisCollector, error) {
	client, err := NewRedisClient(args.Redis)
	if err != nil {
		return nil, err
	}
	return &RedisCollector{logger: logger, client: client, key: args.RedisCollectorKey}, nil
}

func (c *RedisCollector) Push(stop <-chan struct{}, info *IncomingMail) error {
	var buffer bytes.Buffer
	w := lz4.NewWriter(&buffer)
	w.Header = lz4.Header{
		CompressionLevel: 0,
	}
	err := msgp.Encode(w, info)
	if err != nil {
		return err
	}
	err = w.Close()
	if err != nil {
		return err
	}
	c.logger.Debug("lz4 encoded size", "size", len(buffer.Bytes()))
	_, err = c.client.RPush(c.key, buffer.Bytes()).Result()
	return err
}

func (c *RedisCollector) PushCtx(ctx context.Context, info *IncomingMail) error {
	return c.Push(ctx.Done(), info)
}

func (c *RedisCollector) Pull(stop <-chan struct{}) (*IncomingMail, error) {
	var res []string
	var err error
	gotit := make(chan struct{})
	go func() {
	L:
		for {
			res, err = c.client.BLPop(30 * time.Second, c.key).Result()
			if err == redis.Nil {
				continue L
			}
			if err != nil || len(res) > 0 {
				close(gotit)
				return
			}
		}
	}()
	select {
	case <-stop:
		return nil, context.Canceled
	case <-gotit:
	}
	if err != nil {
		return nil, fmt.Errorf("BLPOP error: %s", err)
	}
	if len(res) != 2 {
		return nil, fmt.Errorf("wrong number of returned variables by BLPOP: %d", len(res))
	}
	if len(res[1]) == 0 {
		return nil, errors.New("empty string returned by BLPOP")
	}
	c.logger.Debug("BLPOP result", "length", len(res[1]), "key", res[0])
	raw := strings.NewReader(res[1])
	lz4Reader := lz4.NewReader(raw)
	var mail IncomingMail
	err = msgp.Decode(lz4Reader, &mail)
	if err != nil {
		return nil, fmt.Errorf("messagepack unmarshal error: %s", err)
	}
	return &mail, nil
}

func (c *RedisCollector) PullCtx(ctx context.Context) (*IncomingMail, error) {
	return c.Pull(ctx.Done())
}

func (c *RedisCollector) Close() error {
	return c.client.Close()
}

type FSCollector struct {
	store *FileStore
}

func NewFSCollector(root string, logger log15.Logger) (*FSCollector, error) {
	store, err := NewFileStore(root, logger)
	if err != nil {
		return nil, err
	}
	return &FSCollector{store: store}, nil
}

func (c *FSCollector) Push(stop <-chan struct{}, info *IncomingMail) error {
	return c.store.New(info.UID, info)
}

func (c *FSCollector) PushCtx(ctx context.Context, info *IncomingMail) error {
	return c.Push(ctx.Done(), info)
}

func (c *FSCollector) Pull(stop <-chan struct{}) (*IncomingMail, error) {
	mail := new(IncomingMail)
	err := c.store.Get(stop, mail)
	if err != nil {
		return nil, err
	}
	return mail, nil
}

func (c *FSCollector) PullCtx(ctx context.Context) (*IncomingMail, error) {
	return c.Pull(ctx.Done())
}

func (c *FSCollector) Close() error {
	return c.store.Close()
}

type ChanCollector struct {
	ch     chan *IncomingMail
	once   sync.Once
	logger log15.Logger
	store  *FileStore
}

func NewChanCollector(size int, logger log15.Logger) (*ChanCollector, error) {
	c := new(ChanCollector)
	c.logger = logger
	store, err := NewFileStore("/home/stef/tmp/mailstats", logger)
	if err != nil {
		logger.Error("Error creating store", "error", err)
	}
	c.store = store
	if size <= 0 {
		c.ch = make(chan *IncomingMail)
	} else {
		c.ch = make(chan *IncomingMail, size)
	}
	return c, nil
}

func (c *ChanCollector) Push(stop <-chan struct{}, info *IncomingMail) error {
	select {
	case c.ch <- info:
		c.logger.Debug("New message pushed to collector")
		return nil
	case <-stop:
		return context.Canceled
	}
}

func (c *ChanCollector) PushCtx(ctx context.Context, info *IncomingMail) error {
	return c.Push(ctx.Done(), info)
}

func (c *ChanCollector) Pull(stop <-chan struct{}) (*IncomingMail, error) {
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

func (c *ChanCollector) PullCtx(ctx context.Context) (*IncomingMail, error) {
	return c.Pull(ctx.Done())
}

func (c *ChanCollector) Close() error {
	c.once.Do(func() {
		c.logger.Debug("Closing collector")
		close(c.ch)
	})
	return nil
}
