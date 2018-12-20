package utils

import (
	"fmt"
	"github.com/inconshreveable/log15"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)



type PrometheusLogger struct {
	Logger log15.Logger
}

func (a *PrometheusLogger) Println(v ...interface{}) {
	a.Logger.Error(fmt.Sprintln(v...))
}

func PromLogger(logger log15.Logger) promhttp.Logger {
	return &PrometheusLogger{
		Logger: logger,
	}
}

type ElasticErrorLogger struct {
	Logger log15.Logger
}

func (l *ElasticErrorLogger) Printf(format string, v ...interface{}) {
	l.Logger.Warn("Elasticsearch error", "error", fmt.Sprintf(format, v...))
}