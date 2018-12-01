package main

import (
	"context"
	"errors"
	"sync"

	"github.com/inconshreveable/log15"
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
	default:
		return nil, errors.New("unknown collector")
	}
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
	store *FileStore
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



