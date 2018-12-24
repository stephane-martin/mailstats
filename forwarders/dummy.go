package forwarders

import (
	"context"
	"github.com/stephane-martin/mailstats/models"
)

type DummyForwarder struct{}

func (_ DummyForwarder) Forward(_ *models.IncomingMail) {}

func (_ DummyForwarder) Close() error {
	return nil
}

func (_ DummyForwarder) Start(ctx context.Context) error {
	<-ctx.Done()
	return nil
}


