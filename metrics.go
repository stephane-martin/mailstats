package main

import (
	"github.com/prometheus/client_golang/prometheus"
)

var instance *metrics

func init() {
	instance = newMetrics()
}

func M() *metrics {
	return instance
}

type metrics struct {
	Connections *prometheus.CounterVec
	Registry    *prometheus.Registry
}

func newMetrics() *metrics {
	m := new(metrics)
	m.Connections = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "smtp_connections_total",
			Help: "Number of SMTP client connections",
		},
		[]string{"client_addr", "service"},
	)

	m.Registry = prometheus.NewRegistry()
	m.Registry.MustRegister(
		m.Connections,
	)
	return m
}
