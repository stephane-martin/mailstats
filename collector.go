package main

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"sync"

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
	ch   chan *Infos
	once sync.Once
}

func NewChanCollector(size int) *ChanCollector {
	c := new(ChanCollector)
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
	case info := <-c.ch:
		return info, nil
	case <-stop:
		return nil, context.Canceled
	}
}

func (c *ChanCollector) PullCtx(ctx context.Context) (*Infos, error) {
	return c.Pull(ctx.Done())
}

func (c *ChanCollector) Close() error {
	c.once.Do(func() {
		close(c.ch)
	})
	return nil
}

func Consume(ctx context.Context, collector Collector) error {
	var g errgroup.Group
	cpus := runtime.NumCPU()
	for i := 0; i < cpus; i++ {
		g.Go(func() error {
			for {
				info, err := collector.PullCtx(ctx)
				if info == nil {
					return err
				}
				consume(info)
			}
		})
	}
	return g.Wait()
}

var printLock sync.Mutex

func consume(info *Infos) {
	parsed, err := info.Parse()
	if err != nil {
		return
	}
	b, err := json.Marshal(parsed)
	if err != nil {
		return
	}
	if err == nil {
		printLock.Lock()
		fmt.Println(string(b))
		printLock.Unlock()
	}
}
