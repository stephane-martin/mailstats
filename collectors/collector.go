package collectors

import (
	"context"
	"errors"
	"github.com/inconshreveable/log15"
	"github.com/oklog/ulid"
	"github.com/stephane-martin/mailstats/arguments"
	"github.com/stephane-martin/mailstats/forwarders"
	"github.com/stephane-martin/mailstats/models"
	"github.com/stephane-martin/mailstats/utils"
	"go.uber.org/fx"
)

type Collector interface {
	utils.Service
	Push(stop <-chan struct{}, info *models.IncomingMail) error
	PushCtx(ctx context.Context, info *models.IncomingMail) error
	Pull(stop <-chan struct{}) (*models.IncomingMail, error)
	PullCtx(ctx context.Context) (*models.IncomingMail, error)
	ACK(uid ulid.ULID)
}

func CollectAndForward(done <-chan struct{}, incoming *models.IncomingMail, c Collector, f forwarders.Forwarder) error {
	if f != nil {
		f.Forward(incoming)
	}
	if c != nil {
		return c.Push(done, incoming)
	}
	return nil
}

func NewCollector(args *arguments.Args, logger log15.Logger) (Collector, error) {
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

var CollectorService = fx.Provide(func(lc fx.Lifecycle, args *arguments.Args, logger log15.Logger) (Collector, error) {
	c, err := NewCollector(args, logger)
	if err != nil {
		return nil, err
	}
	if lc != nil {
		utils.Append(lc, c, logger)
	}
	return c, nil
})
