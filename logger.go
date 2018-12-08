package main

import (
	"fmt"
	"github.com/inconshreveable/log15"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)



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
