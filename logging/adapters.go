package logging

import (
	"bytes"
	"fmt"
	"github.com/inconshreveable/log15"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"strings"
)



type PrometheusLogger struct {
	Logger log15.Logger
}

func (a PrometheusLogger) Println(v ...interface{}) {
	a.Logger.Error(fmt.Sprintln(v...))
}

func PromLogger(logger log15.Logger) promhttp.Logger {
	return &PrometheusLogger{
		Logger: logger,
	}
}

type PrintfLogger struct {
	Logger log15.Logger
}

func (l PrintfLogger) Printf(format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	if strings.HasPrefix(msg, "[Fx]") {
		msg2 := strings.TrimSpace(msg[4:])
		parts := strings.SplitN(msg2, "\t", 2)
		if len(parts) == 1 {
			l.Logger.Info(msg)
		} else {
			l.Logger.Debug("Dependency injection", "action", strings.TrimSpace(parts[0]), "details", strings.TrimSpace(parts[1]))
		}
	} else {
		l.Logger.Info(msg)
	}
}

type GinLogger struct {
	Logger log15.Logger
}

func (w GinLogger) Write(b []byte) (int, error) {
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