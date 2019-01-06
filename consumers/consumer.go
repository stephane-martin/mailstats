package consumers

import (
	"errors"
	"github.com/inconshreveable/log15"
	"github.com/stephane-martin/mailstats/arguments"
	"github.com/stephane-martin/mailstats/models"
	"github.com/stephane-martin/mailstats/utils"
	"go.uber.org/fx"
)

type Consumer interface {
	utils.Service
	Consume(features *models.FeaturesMail) error
}

func NewConsumer(args *arguments.Args, logger log15.Logger) (Consumer, error) {
	switch args.Consumer.GetType() {
	case arguments.Stdout:
		return StdoutConsumer, nil
	case arguments.Stderr:
		return StderrConsumer, nil
	case arguments.File:
		return NewFileConsumer(args.Consumer)
	case arguments.Redis:
		return NewRedisConsumer(args.Redis)
	case arguments.HTTP:
		return NewHTTPConsumer(args.Consumer)
	case arguments.Rabbit:
		return NewRabbitConsumer(args.Rabbit, logger)
	case arguments.Kafka:
		return NewKafkaConsumer(args.Kafka.Brokers, args.Kafka.Topic, logger)
	case arguments.Elasticsearch:
		return NewElasticsearchConsumer(args.Elasticsearch.Nodes, args.Elasticsearch.IndexName, logger)
	default:
		return nil, errors.New("unknown consumer type")
	}
}

var ConsumerService = fx.Provide(func(lc fx.Lifecycle, args *arguments.Args, logger log15.Logger) (Consumer, error) {
	c, err := NewConsumer(args, logger)
	if err != nil {
		return nil, err
	}
	utils.Append(lc, c, logger)
	return c, nil
})
