package kafka

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	consumerLag = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "kafka_consumer_lag",
		Help: "Records between the last fetched offset and the partition high watermark.",
	}, []string{"group", "topic", "partition"})

	messagesProcessed = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "kafka_messages_processed_total",
		Help: "Messages handled, by outcome (ok, retry, dlq).",
	}, []string{"group", "topic", "outcome"})

	dlqMessages = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "kafka_dlq_messages_total",
		Help: "Messages moved to a dead-letter topic. Any increase must be investigated.",
	}, []string{"topic", "reason"})

	handlerDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "kafka_handler_duration_seconds",
		Help:    "Time spent handling one message, retries in place included.",
		Buckets: []float64{0.01, 0.05, 0.1, 0.5, 1, 5, 15, 30, 60, 120, 300},
	}, []string{"group", "topic"})
)
