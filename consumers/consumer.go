package consumers

import (
	"errors"
	"github.com/stephane-martin/mailstats/arguments"
	"github.com/stephane-martin/mailstats/models"
)

type Consumer interface {
	Consume(features *models.FeaturesMail) error
	Close() error
}



func MakeConsumer(args arguments.Args) (Consumer, error) {
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
		return NewRabbitConsumer(args.Rabbit)
	default:
		return nil, errors.New("unknown consumer type")
	}
}





