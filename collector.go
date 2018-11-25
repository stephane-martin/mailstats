package main

import (
	"context"
	"encoding/json"
	"runtime"
	"sync"

	"github.com/inconshreveable/log15"
	"golang.org/x/sync/errgroup"
)



type Collector interface {
	Push(stop <-chan struct{}, info *Infos) error
	PushCtx(ctx context.Context, info *Infos) error
	Pull(stop <-chan struct{}) (*Infos, error)
	PullCtx(ctx context.Context) (*Infos, error)
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

func ParseMessages(ctx context.Context, collector Collector, consumer Consumer, forwarder Forwarder, logger log15.Logger) error {
	defer func() { _ = consumer.Close() }()

	var g errgroup.Group
	cpus := runtime.NumCPU()
	for i := 0; i < cpus; i++ {
		g.Go(func() error {
			for {
				info, err := collector.PullCtx(ctx)
				if info == nil {
					return err
				}
				// TODO: refactor
				copyInfo := *info
				go func() {
					err := forwarder.Forward(copyInfo)
					if err != nil {
						logger.Warn("Error forwarding message", "error", err)
					}
				}()
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
	return g.Wait()
}

