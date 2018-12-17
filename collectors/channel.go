package collectors

import (
	"context"
	"github.com/inconshreveable/log15"
	"github.com/oklog/ulid"
	"github.com/stephane-martin/mailstats/metrics"
	"github.com/stephane-martin/mailstats/models"
	"github.com/stephane-martin/mailstats/utils"
)


type ChanCollector struct {
	BaseCollector
	ch         chan *models.IncomingMail
	logger     log15.Logger
}

func NewChanCollector(size int, logger log15.Logger) (*ChanCollector, error) {
	if size <= 0 {
		size = 10000
	}
	c := &ChanCollector{BaseCollector: newBaseCollector(size), logger: logger}
	c.ch = make(chan *models.IncomingMail, size)
	return c, nil
}


func (c *ChanCollector) Start() error {
	for m := range c.BaseCollector.Ch {
		_ = c.Push(c.BaseCollector.Stop, m)
	}
	return nil
}

func (c *ChanCollector) Push(stop <-chan struct{}, m *models.IncomingMail) error {
	metrics.M().MailFrom.WithLabelValues(m.MailFrom, m.Family).Inc()
	for _, r := range m.RcptTo {
		metrics.M().MailTo.WithLabelValues(r, m.Family).Inc()
	}
	m.UID = utils.NewULID()
	select {
	case c.ch <- m:
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
			c.BaseCollector.Add(ulid.ULID(m.UID), m)
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
	close(c.ch)
	c.BaseCollector.Close()
	return nil
}
