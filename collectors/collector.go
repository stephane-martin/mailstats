package collectors

import (
	"context"
	"errors"
	"github.com/inconshreveable/log15"
	"github.com/oklog/ulid"
	"github.com/stephane-martin/mailstats/arguments"
	"github.com/stephane-martin/mailstats/forwarders"
	"github.com/stephane-martin/mailstats/models"
	"sync"
	"time"
)

type Collector interface {
	Push(stop <-chan struct{}, info *models.IncomingMail) error
	PushCtx(ctx context.Context, info *models.IncomingMail) error
	Pull(stop <-chan struct{}) (*models.IncomingMail, error)
	PullCtx(ctx context.Context) (*models.IncomingMail, error)
	ACK(uid ulid.ULID)
	Start() error
	Close() error
}

func CollectAndForward(done <-chan struct{}, incoming *models.IncomingMail, c Collector, f forwarders.Forwarder) error {
	f.Forward(incoming)
	return c.Push(done, incoming)
}

func NewCollector(args arguments.Args, logger log15.Logger) (Collector, error) {
	logger.Debug("Collector", "type", args.Collector.Collector)
	switch args.Collector.Collector {
	case "channel":
		return NewChanCollector(args.Collector.CollectorSize, logger)
	case "filesystem":
		return NewFSCollector(args.Collector.CollectorDir, logger)
	case "redis":
		return NewRedisCollector(args.Redis, logger)
	case "rabbitmq":
		return NewRabbitCollector(args.Rabbit, logger)
	default:
		return nil, errors.New("unknown collector")
	}
}

type BaseCollector struct {
	Cur  *sync.Map
	Stop chan struct{}
	// TODO: replace Ch with an unbounded queue?
	Ch chan *models.IncomingMail
}

func newBaseCollector(size int) BaseCollector {
	base := BaseCollector{
		Cur:  new(sync.Map),
		Stop: make(chan struct{}),
		Ch:   make(chan *models.IncomingMail, size),
	}
	go func() {
		for {
			select {
			case <-base.Stop:
				base.Cur.Range(func(k, v interface{}) bool {
					base.Cur.Delete(k.(ulid.ULID))
					base.Ch <- v.(*models.IncomingMail)
					return true
				})
				close(base.Ch)
				return
			case <-time.After(time.Minute):
				base.RePush()
			}
		}
	}()
	return base
}

func (c BaseCollector) Add(uid ulid.ULID, m *models.IncomingMail) {
	c.Cur.Store(uid, m)
}

func (c BaseCollector) RePush() {
	now := time.Now()
	c.Cur.Range(func(k, v interface{}) bool {
		uid := k.(ulid.ULID)
		if now.Sub(ulid.Time(uid.Time())) >= time.Minute {
			// not ACKed soon enough, push back
			select {
			case <-c.Stop:
				return false
			case c.Ch <- v.(*models.IncomingMail):
				c.Cur.Delete(uid)
			}
		}
		return true
	})
}

func (c BaseCollector) ACK(uid ulid.ULID) {
	c.Cur.Delete(uid)
}

func (c BaseCollector) Close() {
	close(c.Stop)
}
