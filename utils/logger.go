package utils

import (
	"bytes"
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

type GinLogger struct {
	Logger log15.Logger
}

func (w *GinLogger) Write(b []byte) (int, error) {
	l := len(b)
	dolog := w.Logger.Info
	b = bytes.TrimSpace(b)
	b = bytes.Replace(b, []byte{'\t'}, []byte{' '}, -1)
	b = bytes.Replace(b, []byte{'"'}, []byte{'\''}, -1)
	if bytes.HasPrefix(b, []byte("[GIN-debug] ")) {
		b = b[12:]
	}
	if bytes.HasPrefix(b, []byte("[WARNING] ")) {
		b = b[10:]
		dolog = w.Logger.Warn
	}
	lines := bytes.Split(b, []byte{'\n'})
	for _, line := range lines {
		dolog(string(line))
	}
	return l, nil
}