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

type ConsumerParams struct {
	fx.In
	Args   *arguments.Args
	Logger log15.Logger    `optional:"true"`
	Redis  utils.RedisConn `name:"consumer" optional:"true"`
}

func NewConsumer(args *arguments.Args, redis utils.RedisConn, logger log15.Logger) (Consumer, error) {
	typ := args.Consumer.GetType()
	if typ == arguments.Redis && redis == nil {
		return nil, errors.New("redis consumer required, but not redis connection provided")
	}
	switch typ {
	case arguments.Stdout:
		return StdoutConsumer, nil
	case arguments.Stderr:
		return StderrConsumer, nil
	case arguments.File:
		return NewFileConsumer(args.Consumer)
	case arguments.Redis:
		return NewRedisConsumer(args.Redis, redis)
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

var ConsumerService = fx.Provide(func(lc fx.Lifecycle, params ConsumerParams) (Consumer, error) {
	logger := params.Logger
	if logger == nil {
		logger = log15.New()
		logger.SetHandler(log15.DiscardHandler())
	}
	c, err := NewConsumer(params.Args, params.Redis, logger)
	if err != nil {
		return nil, err
	}
	utils.Append(lc, c, logger)
	return c, nil
})
