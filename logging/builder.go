package logging

import (
	"github.com/gin-gonic/gin"
	"github.com/inconshreveable/log15"
	"github.com/stephane-martin/mailstats/arguments"
	"log"
	"log/syslog"
	"os"
)

func NewLogger(args *arguments.Args) log15.Logger {
	lvl, _ := log15.LvlFromString(args.Logging.LogLevel)
	logger := log15.New()

	if args.Logging.Syslog {
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
		return logger
	}

	logger.SetHandler(
		log15.LvlFilterHandler(
			lvl,
			log15.StreamHandler(
				os.Stderr,
				log15.LogfmtFormat(),
			),
		),
	)
	initGinLogging(logger)
	return logger
}


func initGinLogging(l log15.Logger) {
	wr := &GinLogger{Logger: l}
	gin.DefaultWriter = wr
	gin.DefaultErrorWriter = wr
	log.SetOutput(wr)
}
