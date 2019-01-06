package forwarders

import (
	"github.com/stephane-martin/mailstats/models"
)

type DummyForwarder struct{}

func (_ *DummyForwarder) Forward(_ *models.IncomingMail) {}

func (_ *DummyForwarder) Name() string { return "DummyForwarder" }



