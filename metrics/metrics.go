package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

var instance *Metrics

func init() {
	instance = newMetrics()
}

func M() *Metrics {
	return instance
}

type Metrics struct {
	Connections          *prometheus.CounterVec
	MailFrom             *prometheus.CounterVec
	MailTo               *prometheus.CounterVec
	CollectorSize        prometheus.Gauge
	CollectorPendingSize prometheus.Gauge
	ParsingDuration      prometheus.Histogram
	ParsingErrors        *prometheus.CounterVec
	MessageSize prometheus.Histogram
	Registry             *prometheus.Registry
}

func newMetrics() *Metrics {
	m := new(Metrics)

	m.Connections = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "connections_total",
			Help: "Number of SMTP client connections",
		},
		[]string{"client_addr", "service"},
	)

	m.MailFrom = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mail_from_total",
			Help: "Number of received emails by sender",
		},
		[]string{"from", "family"},
	)

	m.MailTo = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mail_to_total",
			Help: "Number of received emails by recipient",
		},
		[]string{"to", "family"},
	)

	m.CollectorSize = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "collector_size",
			Help: "The number of messages currently queued in the collector",
		},
	)

	m.CollectorPendingSize = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "collector_pending_size",
			Help: "The number of messages currently processed by workers",
		},
	)

	m.ParsingDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "parsing_duration",
			Help:    "histogram of the duration of message parsing in seconds",
			Buckets: prometheus.DefBuckets,
		})

	m.ParsingErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "parsing_errors_total",
			Help: "The number of parsing errors",
		},
		[]string{"family"},
	)

	m.MessageSize = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name: "message_size",
			Help: "histogram of message size in bytes",
			Buckets: prometheus.ExponentialBuckets(1000, 10, 6),
		},
	)

	m.Registry = prometheus.NewRegistry()
	m.Registry.MustRegister(
		m.Connections,
		m.MailFrom,
		m.MailTo,
		m.CollectorSize,
		m.CollectorPendingSize,
		m.ParsingDuration,
		m.ParsingErrors,
		m.MessageSize,
	)
	return m
}
