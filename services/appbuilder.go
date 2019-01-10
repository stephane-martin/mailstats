package services

import (
	"github.com/inconshreveable/log15"
	"github.com/stephane-martin/mailstats/arguments"
	"github.com/stephane-martin/mailstats/collectors"
	"github.com/stephane-martin/mailstats/consumers"
	"github.com/stephane-martin/mailstats/extractors"
	"github.com/stephane-martin/mailstats/forwarders"
	"github.com/stephane-martin/mailstats/logging"
	"github.com/stephane-martin/mailstats/parser"
	"github.com/stephane-martin/mailstats/phishtank"
	"github.com/stephane-martin/mailstats/utils"
	"github.com/urfave/cli"
	"go.uber.org/fx"
)

func Builder(c *cli.Context, args *arguments.Args, invoke fx.Option, logger log15.Logger) *fx.App {
	provides := []fx.Option{
		forwarders.ForwarderService,
		consumers.ConsumerService,
		collectors.CollectorService,
		parser.Service,
		extractors.ExifToolService,
		HTTPService,
		HTTPMasterService,
		SMTPService,
		MilterService,
		IMAPMonitorService,
		utils.GeoIPService,
		utils.RedisService,
		phishtank.Service,
		fx.Provide(
			func() *cli.Context { return c },
			func() *arguments.Args { return args },
			func() log15.Logger { return logger },
			NewSMTPBackend,
		),
	}

	options := make([]fx.Option, 0)
	options = append(options, provides...)
	options = append(options, fx.Logger(logging.PrintfLogger{Logger: logger}))
	options = append(options, invoke)

	return fx.New(options...)
}
