package forwarders

import (
	"fmt"
	"github.com/inconshreveable/log15"
	"github.com/stephane-martin/mailstats/arguments"
	"github.com/stephane-martin/mailstats/models"
	"github.com/stephane-martin/mailstats/utils"
	"go.uber.org/fx"
)

type Forwarder interface {
	utils.Service
	Forward(mail *models.IncomingMail)
}

func NewForwarder(args *arguments.Args, logger log15.Logger) (Forwarder, error) {
	scheme, host, port, username, password := args.Forward.Parsed()
	if host == "" {
		logger.Info("No forwarding")
		return new(DummyForwarder), nil
	}
	var f Forwarder
	switch scheme {
	case "smtp", "smtps":
		if len(username) == 0 || len(password) == 0 {
			f = NewSMTPForwarder(scheme, host, port, "", "", logger)
		} else {
			f = NewSMTPForwarder(scheme, host, port, username, password, logger)
		}
	case "http", "https":
		f = NewHTTPForwarder(args.Forward.URL, logger)
	default:
		return nil, fmt.Errorf("unknown forwarder type: %s", scheme)
	}

	return f, nil

}


var ForwarderService = fx.Provide(func(lc fx.Lifecycle, args *arguments.Args, logger log15.Logger) (Forwarder, error) {
	f, err := NewForwarder(args, logger)
	if err != nil {
		return nil, err
	}
	utils.Append(lc, f, logger)
	return f, nil
})

