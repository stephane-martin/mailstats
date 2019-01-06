package collectors

import (
	"context"
	"github.com/oklog/ulid"
	"github.com/stephane-martin/mailstats/models"
	"sync"
	"time"
)

type BaseCollector struct {
	Cur *sync.Map
	// TODO: replace Ch with an unbounded queue?
	Ch chan *models.IncomingMail
}

func newBaseCollector(size int) BaseCollector {
	return BaseCollector{
		Cur: new(sync.Map),
		Ch:  make(chan *models.IncomingMail, size),
	}
}

func (c BaseCollector) Start(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			c.Cur.Range(func(k, v interface{}) bool {
				c.Cur.Delete(k.(ulid.ULID))
				c.Ch <- v.(*models.IncomingMail)
				return true
			})
			close(c.Ch)
			return
		case <-time.After(time.Minute):
			c.RePush(ctx)
		}
	}
}

func (c BaseCollector) Add(uid ulid.ULID, m *models.IncomingMail) {
	c.Cur.Store(uid, m)
}

func (c BaseCollector) RePush(ctx context.Context) {
	now := time.Now()
	c.Cur.Range(func(k, v interface{}) bool {
		uid := k.(ulid.ULID)
		if now.Sub(ulid.Time(uid.Time())) >= time.Minute {
			// not ACKed soon enough, push back
			select {
			case <-ctx.Done():
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
