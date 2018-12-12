package collectors

import (
	"context"
	"github.com/inconshreveable/log15"
	"github.com/oklog/ulid"
	"github.com/stephane-martin/mailstats/metrics"
	"github.com/stephane-martin/mailstats/models"
	"github.com/stephane-martin/mailstats/utils"
)

type FSCollector struct {
	BaseCollector
	store *FileStore
}

func NewFSCollector(root string, logger log15.Logger) (*FSCollector, error) {
	store, err := NewFileStore(root, logger)
	if err != nil {
		return nil, err
	}
	c := &FSCollector{BaseCollector: newBaseCollector(10000), store: store}

	return c, nil
}

func (c *FSCollector) Start() error {
	for m := range c.BaseCollector.Ch {
		_ = c.Push(nil, m)
	}
	return nil
}

func (c *FSCollector) Push(stop <-chan struct{}, m *models.IncomingMail) error {
	metrics.M().MailFrom.WithLabelValues(m.MailFrom).Inc()
	for _, r := range m.RcptTo {
		metrics.M().MailTo.WithLabelValues(r).Inc()
	}
	m.UID = utils.NewULID()
	err := c.store.New(m.UID, m)
	if err == nil {
		metrics.M().CollectorSize.Inc()
	}
	return err
}

func (c *FSCollector) PushCtx(ctx context.Context, info *models.IncomingMail) error {
	return c.Push(ctx.Done(), info)
}

func (c *FSCollector) Pull(stop <-chan struct{}) (*models.IncomingMail, error) {
	m := new(models.IncomingMail)
	err := c.store.Get(stop, m)
	if err != nil {
		return nil, err
	}
	metrics.M().CollectorSize.Dec()
	c.BaseCollector.Add(ulid.ULID(m.UID), m)
	return m, nil
}

func (c *FSCollector) PullCtx(ctx context.Context) (*models.IncomingMail, error) {
	return c.Pull(ctx.Done())
}

func (c *FSCollector) Close() error {
	c.BaseCollector.Close()
	return c.store.Close()
}

