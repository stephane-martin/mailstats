package consumers

import (
	"context"
	"encoding/json"
	"github.com/inconshreveable/log15"
	"github.com/rafaeljesus/rabbus"
	"github.com/stephane-martin/mailstats/arguments"
	"github.com/stephane-martin/mailstats/models"
	"time"
)

type RabbitConsumer struct {
	cancel context.CancelFunc
	publisher *rabbus.Rabbus
	routingKey string
	exchange string
	exchangeType string
	logger log15.Logger
}

func NewRabbitConsumer(args arguments.RabbitArgs, logger log15.Logger) (Consumer, error) {
	c := &RabbitConsumer{
		routingKey: args.ResultsRoutingKey,
		exchange: args.ResultsExchange,
		exchangeType: args.ResultsExchangeType,
		logger: logger,
	}
	publisherChangeFunc := func(name, from, to string) {
		logger.Info("RabbitMQ consumer state change", "name", name, "from", from, "to", to)
	}

	publisher, err := rabbus.New(
		args.URI,
		rabbus.Durable(true),
		rabbus.Attempts(5),
		rabbus.Sleep(time.Second*2),
		rabbus.Threshold(3),
		rabbus.OnStateChange(publisherChangeFunc),
	)
	if err != nil {
		return nil, err
	}
	c.publisher = publisher

	var ctx context.Context
	ctx, c.cancel = context.WithCancel(context.Background())

	go func() {
		_ = c.publisher.Run(ctx)
		_ = c.publisher.Close()
	}()

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
	select {
	case <-c.publisher.EmitOk():
		return nil
	case err := <-c.publisher.EmitErr():
		return err
	}
}

func (c *RabbitConsumer) Close() error {
	if c.cancel != nil {
		c.cancel()
	}
	return nil
}

