package metrics

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

// TODO: received mail size
// TODO: parsing time

type metrics struct {
	Connections   *prometheus.CounterVec
	MailFrom      *prometheus.CounterVec
	MailTo *prometheus.CounterVec
	CollectorSize prometheus.Gauge
	Registry      *prometheus.Registry
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

	m.MailFrom = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mail_from",
			Help: "Number of received emails by sender",
		},
		[]string{"from"},
	)

	m.MailTo = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mail_to",
			Help: "Number of received emails by recipient",
		},
		[]string{"to"},
	)

	m.CollectorSize = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "collector_size",
			Help: "The number of messages currently queued in the collector",
		},
	)

	m.Registry = prometheus.NewRegistry()
	m.Registry.MustRegister(
		m.Connections,
		m.MailFrom,
		m.MailTo,
		m.CollectorSize,
	)
	return m
}
