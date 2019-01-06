package consumers

import (
	"context"
	"github.com/inconshreveable/log15"
	"github.com/rafaeljesus/rabbus"
	"github.com/stephane-martin/mailstats/arguments"
	"github.com/stephane-martin/mailstats/models"
	"github.com/stephane-martin/mailstats/utils"
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
	return c, nil

}

func (c *RabbitConsumer) Name() string {
	return "RabbitConsumer"
}

func (c *RabbitConsumer) Start(ctx context.Context) error {
	return c.publisher.Run(ctx)
}

func (c *RabbitConsumer) Consume(features *models.FeaturesMail) error {
	b, err := utils.JSONMarshal(features)
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
	return c.publisher.Close()
}
