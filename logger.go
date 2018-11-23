package main

import (
	"fmt"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/storozhukBM/verifier"
	"github.com/urfave/cli"
	"log/syslog"
	"os"
	"strings"

	"github.com/inconshreveable/log15"
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

type adaptedPromLogger struct {
	logger log15.Logger
}

func (a *adaptedPromLogger) Println(v ...interface{}) {
	a.logger.Error(fmt.Sprintln(v...))
}

func adaptPromLogger(logger log15.Logger) promhttp.Logger {
	return &adaptedPromLogger{
		logger: logger,
	}
}
