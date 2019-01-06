package utils

import (
	"context"
	"github.com/inconshreveable/log15"
	"go.uber.org/fx"
)

type Service interface {
	Name() string
}

type Startable interface {
	// Start starts long-running services. It should return only when the passed context is done.
	Start(ctx context.Context) error
}

type Prestartable interface {
	// Prestart perform some immediate initialization steps.
	Prestart() error
}

type Closeable interface {
	// Close stops and cleans the long-running services. It should return only when all the service goroutines have stopped.
	Close() error
}

func Append(lc fx.Lifecycle, svc Service, logger log15.Logger) {
	if lc == nil || svc == nil {
		return
	}

	if logger == nil {
		logger = log15.New()
		logger.SetHandler(log15.DiscardHandler())
	}

	ctx, cancel := context.WithCancel(context.Background())
	startHasReturned := make(chan struct{})

	preStart := func() error { return nil }
	start := func(ctx context.Context) error { <-ctx.Done(); return nil }
	cl := func() error { return nil }

	if s, ok := svc.(Prestartable); ok {
		preStart = func() error {
			logger.Info("Service prestart", "name", svc.Name())
			return s.Prestart()
		}
	}

	if s, ok := svc.(Startable); ok {
		start = func(ctx context.Context) error {
			logger.Info("Service start", "name", svc.Name())
			return s.Start(ctx)
		}
	}

	if s, ok := svc.(Closeable); ok {
		cl = func() error {
			logger.Info("Service close", "name", svc.Name())
			return s.Close()
		}
	}

	lc.Append(fx.Hook{
		OnStart: func(startCtx context.Context) error {
			err := preStart()
			if err != nil {
				close(startHasReturned)
				return err
			}
			go func() {
				err := start(ctx)
				close(startHasReturned)
				logger.Info("service has returned", "name", svc.Name(), "error", err)
			}()
			return nil
		},
		OnStop: func(stopCtx context.Context) error {
			cancel()
			<-startHasReturned
			return cl()
		},
	})
}
