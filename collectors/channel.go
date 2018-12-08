package collectors

import (
	"context"
	"github.com/inconshreveable/log15"
	"github.com/stephane-martin/mailstats/metrics"
	"github.com/stephane-martin/mailstats/models"
	"sync"
)

type ChanCollector struct {
	ch     chan *models.IncomingMail
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
		c.ch = make(chan *models.IncomingMail)
	} else {
		c.ch = make(chan *models.IncomingMail, size)
	}
	return c, nil
}

func (c *ChanCollector) Push(stop <-chan struct{}, info *models.IncomingMail) error {
	metrics.M().MailFrom.WithLabelValues(info.MailFrom).Inc()
	for _, r := range info.RcptTo {
		metrics.M().MailTo.WithLabelValues(r).Inc()
	}
	select {
	case c.ch <- info:
		metrics.M().CollectorSize.Inc()
		return nil
	case <-stop:
		return context.Canceled
	}
}

func (c *ChanCollector) PushCtx(ctx context.Context, info *models.IncomingMail) error {
	return c.Push(ctx.Done(), info)
}

func (c *ChanCollector) Pull(stop <-chan struct{}) (*models.IncomingMail, error) {
	select {
	case m, ok := <-c.ch:
		if ok {
			metrics.M().CollectorSize.Dec()
			return m, nil
		}
		return nil, context.Canceled
	case <-stop:
		return nil, context.Canceled
	}
}

func (c *ChanCollector) PullCtx(ctx context.Context) (*models.IncomingMail, error) {
	return c.Pull(ctx.Done())
}

func (c *ChanCollector) Close() error {
	c.once.Do(func() {
		close(c.ch)
	})
	return nil
}

