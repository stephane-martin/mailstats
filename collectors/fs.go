package collectors

import (
	"context"
	"github.com/inconshreveable/log15"
	"github.com/stephane-martin/mailstats/metrics"
	"github.com/stephane-martin/mailstats/models"
)

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

func (c *FSCollector) Push(stop <-chan struct{}, info *models.IncomingMail) error {
	metrics.M().MailFrom.WithLabelValues(info.MailFrom).Inc()
	for _, r := range info.RcptTo {
		metrics.M().MailTo.WithLabelValues(r).Inc()
	}
	err := c.store.New(info.UID, info)
	if err == nil {
		metrics.M().CollectorSize.Inc()
	}
	return err
}

func (c *FSCollector) PushCtx(ctx context.Context, info *models.IncomingMail) error {
	return c.Push(ctx.Done(), info)
}

func (c *FSCollector) Pull(stop <-chan struct{}) (*models.IncomingMail, error) {
	mail := new(models.IncomingMail)
	err := c.store.Get(stop, mail)
	if err != nil {
		return nil, err
	}
	metrics.M().CollectorSize.Dec()
	return mail, nil
}

func (c *FSCollector) PullCtx(ctx context.Context) (*models.IncomingMail, error) {
	return c.Pull(ctx.Done())
}

func (c *FSCollector) Close() error {
	return c.store.Close()
}

