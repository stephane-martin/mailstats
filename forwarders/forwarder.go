package forwarders

import (
	"context"
	"fmt"
	"github.com/inconshreveable/log15"
	"github.com/stephane-martin/mailstats/arguments"
	"github.com/stephane-martin/mailstats/models"
)

func Build(args arguments.ForwardArgs, logger log15.Logger) (Forwarder, error) {
	scheme, host, port, username, password := args.Parsed()
	if host == "" {
		logger.Info("No forwarding")
		return new(DummyForwarder), nil
	}
	switch scheme {
	case "smtp", "smtps":
		if len(username) == 0 || len(password) == 0 {
			return NewSMTPForwarder(scheme, host, port, "", "", logger), nil
		}
		return NewSMTPForwarder(scheme, host, port, username, password, logger), nil
	case "http", "https":
		return NewHTTPForwarder(args.URL, logger), nil
	default:
		return nil, fmt.Errorf("unknown forwarder type: %s", scheme)
	}


}

type Forwarder interface {
	Forward(mail *models.IncomingMail)
	Start(ctx context.Context) error
	Close() error
}



