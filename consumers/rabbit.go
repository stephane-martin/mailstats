package consumers

import (
	"context"
	"encoding/json"
	"github.com/rafaeljesus/rabbus"
	"github.com/stephane-martin/mailstats/arguments"
	"github.com/stephane-martin/mailstats/models"
	"golang.org/x/sync/errgroup"
	"time"
)

type RabbitConsumer struct {
	cancel context.CancelFunc
	publisher *rabbus.Rabbus
	routingKey string
	exchange string
	exchangeType string
}

func NewRabbitConsumer(args arguments.RabbitArgs) (Consumer, error) {
	c := &RabbitConsumer{
		routingKey: args.ResultsRoutingKey,
		exchange: args.ResultsExchange,
		exchangeType: args.ResultsExchangeType,
	}
	cbStateChangeFunc := func(name, from, to string) {

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
	c.publisher = publisher

	gctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel
	g, ctx := errgroup.WithContext(gctx)

	g.Go(func() error {
		for range c.publisher.EmitErr() {
		}
		return nil
	})
	g.Go(func() error {
		for range c.publisher.EmitOk() {
		}
		return nil
	})

	g.Go(func() error {
		_= c.publisher.Run(ctx)
		return c.publisher.Close()
	})

	return c, nil

}

func (c *RabbitConsumer) Consume(features *models.FeaturesMail) error {
	b, err := json.Marshal(features)
	if err != nil {
		return err
	}
	msg := rabbus.Message{
		Exchange:     c.exchange,
		Key:          c.routingKey,
		Payload:      b,
		Kind:         c.exchangeType,
		DeliveryMode: rabbus.Persistent,
		ContentType:  rabbus.ContentTypeJSON,
	}
	c.publisher.EmitAsync() <- msg
	return nil
}

func (c *RabbitConsumer) Close() error {
	if c.cancel != nil {
		c.cancel()
	}
	return nil
}

