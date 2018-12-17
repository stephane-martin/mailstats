package collectors

import (
	"context"
	"fmt"
	"github.com/inconshreveable/log15"
	"github.com/oklog/ulid"
	"github.com/rafaeljesus/rabbus"
	"github.com/stephane-martin/mailstats/arguments"
	"github.com/stephane-martin/mailstats/metrics"
	"github.com/stephane-martin/mailstats/models"
	"github.com/stephane-martin/mailstats/utils"
	"golang.org/x/sync/errgroup"
	"sync"
	"time"
)

type RabbitCollector struct {
	logger    log15.Logger
	publisher *rabbus.Rabbus
	consumer  *rabbus.Rabbus
	exchange  string
	queue     string
	cancel    context.CancelFunc
	incoming  chan rabbus.ConsumerMessage
	uidToTag  sync.Map
}

func NewRabbitCollector(args arguments.RabbitArgs, logger log15.Logger) (*RabbitCollector, error) {
	cbStateChangeFunc := func(name, from, to string) {
		logger.Info("RabbitMQ collector state change", "name", name, "from", from, "to", to)
	}
	publisher, err := rabbus.New(
		args.URI,
		rabbus.Durable(true),
		rabbus.Attempts(5),
		rabbus.Sleep(time.Second*2),
		rabbus.Threshold(3),
		rabbus.OnStateChange(cbStateChangeFunc),
	)
	if err != nil {
		return nil, err
	}

	consumer, err := rabbus.New(
		args.URI,
		rabbus.Durable(true),
		rabbus.Attempts(5),
		rabbus.Sleep(time.Second*2),
		rabbus.Threshold(3),
		rabbus.OnStateChange(cbStateChangeFunc),
	)
	if err != nil {
		return nil, err
	}

	incoming, err := consumer.Listen(rabbus.ListenConfig{
		Exchange: args.CollectorExchange,
		Kind:     rabbus.ExchangeDirect,
		Key:      args.CollectorQueue,
		Queue:    args.CollectorQueue,
	})
	if err != nil {
		return nil, err
	}

	return &RabbitCollector{
		logger:    logger,
		consumer:  consumer,
		publisher: publisher,
		queue:     args.CollectorQueue,
		exchange:  args.CollectorExchange,
		incoming:  incoming,
	}, nil
}

func (c *RabbitCollector) Push(stop <-chan struct{}, m *models.IncomingMail) error {
	metrics.M().MailFrom.WithLabelValues(m.MailFrom, m.Family).Inc()
	for _, r := range m.RcptTo {
		metrics.M().MailTo.WithLabelValues(r, m.Family).Inc()
	}
	m.UID = utils.NewULID()
	b, _ := m.MarshalMsg(nil)
	msg := rabbus.Message{
		Exchange:     c.exchange,
		Key:          c.queue,
		Payload:      b,
		Kind:         rabbus.ExchangeDirect,
		DeliveryMode: rabbus.Persistent,
		ContentType:  "application/octet-stream",
		Headers: map[string]interface{}{
			"uid": ulid.ULID(m.UID).String(),
		},
	}
	select {
	case <-stop:
		return context.Canceled
	case c.publisher.EmitAsync() <- msg:
		return nil
	}
}

func (c *RabbitCollector) PushCtx(ctx context.Context, m *models.IncomingMail) error {
	return c.Push(ctx.Done(), m)
}

func (c *RabbitCollector) Pull(stop <-chan struct{}) (*models.IncomingMail, error) {
	select {
	case <-stop:
		return nil, context.Canceled
	case msg, ok := <-c.incoming:
		if !ok {
			return nil, context.Canceled
		}
		metrics.M().CollectorSize.Dec()
		var m models.IncomingMail
		_, err := m.UnmarshalMsg(msg.Body)
		msg.Body = nil // free memory
		if err != nil {
			_ = msg.Ack(true)
			return nil, fmt.Errorf("messagepack unmarshal error: %s", err)
		}
		c.uidToTag.Store(ulid.MustParse(msg.Headers["uid"].(string)), &msg)
		return &m, nil
	}
}

func (c *RabbitCollector) PullCtx(ctx context.Context) (*models.IncomingMail, error) {
	return c.Pull(ctx.Done())
}

func (c *RabbitCollector) ACK(uid ulid.ULID) {
	msg, ok := c.uidToTag.Load(uid)
	if !ok {
		c.logger.Warn("Unknown UID in ACK", "uid", uid.String())
		return
	}
	c.uidToTag.Delete(uid)
	err := msg.(*rabbus.ConsumerMessage).Ack(false)
	if err != nil {
		c.logger.Warn("Error ACK RabbitMQ", "error", err)
	}
}

func (c *RabbitCollector) Start() error {
	gctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel
	g, ctx := errgroup.WithContext(gctx)

	go func() {
		err, ok := <-c.publisher.EmitErr()
		if ok {
			c.logger.Error("Error pushing message to RabbitMQ", "error", err)
			for range c.publisher.EmitErr() {
			}
			cancel()
		}
	}()
	go func() {
		for range c.publisher.EmitOk() {
			metrics.M().CollectorSize.Inc()
		}
	}()

	g.Go(func() error {
		return c.consumer.Run(ctx)
	})
	g.Go(func() error {
		return c.publisher.Run(ctx)
	})

	err := g.Wait()
	_ = c.consumer.Close()
	_ = c.publisher.Close()
	return err
}

func (c *RabbitCollector) Close() error {
	if c.cancel != nil {
		c.cancel()
	}
	return nil
}
