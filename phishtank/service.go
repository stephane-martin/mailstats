package phishtank

import (
	"context"
	"github.com/inconshreveable/log15"
	"github.com/stephane-martin/mailstats/arguments"
	"github.com/stephane-martin/mailstats/models"
	"github.com/stephane-martin/mailstats/utils"
	"go.uber.org/fx"
	"runtime"
	"sync/atomic"
	"time"
)

type Phishtank interface {
	utils.Service
	utils.Startable
	URL(string) []*models.PhishtankEntry
	URLMany([]string) []*models.PhishtankEntry
}

type impl struct {
	logger log15.Logger
	tree atomic.Value
	active bool
	appKey string
	cacheDir string
}

func (i *impl) URL(u string) []*models.PhishtankEntry {
	if !i.active {
		return nil
	}
	tree := i.getTree()
	if tree == nil {
		return nil
	}
	return tree.Get(u)
}

func (i *impl) URLMany(urls []string) []*models.PhishtankEntry {
	if !i.active || len(urls) == 0 {
		return nil
	}
	tree := i.getTree()
	if tree == nil {
		return nil
	}
	res := make([]*models.PhishtankEntry, 0, len(urls))
	for _, u := range urls {
		res = append(res, tree.Get(u)...)
	}
	return res
}

func (i *impl) Name() string {
	return "Phishtank"
}

func (i *impl) Start(ctx context.Context) error {
	for {
		entries, errs := Download(ctx, i.cacheDir, i.appKey, i.logger)
		tree, err := BuildTree(ctx, entries, errs, i.logger)
		if err == context.Canceled {
			return err
		}
		if err != nil {
			i.logger.Warn("Error building phishtank tree", "error", err)
		} else {
			i.tree.Store(tree)
		}
		runtime.GC()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Hour):
		}
	}
}

func (i *impl) getTree() *Tree {
	if !i.active {
		return nil
	}
	tree := i.tree.Load()
	if tree == nil {
		return nil
	}
	return tree.(*Tree)
}

func NewPhishtank(active bool, appKey string, cacheDir string, logger log15.Logger) Phishtank {
	return &impl{
		active: active,
		logger: logger,
		appKey: appKey,
		cacheDir: cacheDir,
	}
}

type Params struct {
	fx.In
	Args   *arguments.Args `optional:"true"`
	Logger log15.Logger    `optional:"true"`
}

var Service = fx.Provide(func(lc fx.Lifecycle, params Params) Phishtank {
	logger := params.Logger
	if logger == nil {
		logger = log15.New()
		logger.SetHandler(log15.DiscardHandler())
	}
	p := NewPhishtank(
		params.Args.Phishtank.Active,
		params.Args.Phishtank.ApplicationKey,
		params.Args.Phishtank.CacheDir,
		logger,
	)

	utils.Append(lc, p, logger)
	return p
})
