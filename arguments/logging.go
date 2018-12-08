package arguments

import (
	"github.com/inconshreveable/log15"
	"github.com/storozhukBM/verifier"
	"github.com/urfave/cli"
	"log/syslog"
	"os"
	"strings"
)

type LoggingArgs struct {
	LogLevel string
	Syslog bool
}

func (args *LoggingArgs) Populate(c *cli.Context) *LoggingArgs {
	if args == nil {
		args = new(LoggingArgs)
	}
	args.LogLevel = strings.ToLower(strings.TrimSpace(c.GlobalString("loglevel")))
	args.Syslog = c.GlobalBool("syslog")
	return args
}

func (args LoggingArgs) Verify() error {
	v := verifier.New()
	v.That(len(args.LogLevel) > 0, "loglevel is empty")
	return v.GetError()
}

func (args LoggingArgs) Build() log15.Logger {

	lvl, _ := log15.LvlFromString(args.LogLevel)
	logger := log15.New()
	if args.Syslog {
		logger.SetHandler(
			log15.LvlFilterHandler(
				lvl,
				log15.Must.SyslogHandler(
					syslog.LOG_INFO|syslog.LOG_DAEMON,
					"mailstats",
					log15.JsonFormat(),
				),
			),
		)
	} else {
		logger.SetHandler(
			log15.LvlFilterHandler(
				lvl,
				log15.StreamHandler(
					os.Stderr,
					log15.LogfmtFormat(),
				),
			),
		)
	}
	return logger
}
