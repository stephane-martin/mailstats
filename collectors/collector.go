package collectors

import (
	"context"
	"errors"
	"github.com/inconshreveable/log15"
	"github.com/stephane-martin/mailstats/arguments"
	"github.com/stephane-martin/mailstats/models"
)

type Collector interface {
	Push(stop <-chan struct{}, info *models.IncomingMail) error
	PushCtx(ctx context.Context, info *models.IncomingMail) error
	Pull(stop <-chan struct{}) (*models.IncomingMail, error)
	PullCtx(ctx context.Context) (*models.IncomingMail, error)
	Close() error
}

func NewCollector(args *arguments.Args, logger log15.Logger) (Collector, error) {
	logger.Debug("Collector", "type", args.Collector)
	switch args.Collector {
	case "channel":
		return NewChanCollector(args.CollectorSize, logger)
	case "filesystem":
		return NewFSCollector(args.CollectorDir, logger)
	case "redis":
		return NewRedisCollector(args, logger)
	default:
		return nil, errors.New("unknown collector")
	}
}
