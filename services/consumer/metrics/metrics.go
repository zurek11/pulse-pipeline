package metrics

import "github.com/prometheus/client_golang/prometheus"

// Consumer holds all Prometheus metrics for the consumer service.
type Consumer struct {
	// EventsConsumed counts events read from Kafka, labelled by event_type.
	EventsConsumed *prometheus.CounterVec
	// DLQTotal counts events routed to the Dead Letter Queue.
	DLQTotal prometheus.Counter
	// WriteDuration tracks MongoDB bulk write latency as a histogram.
	WriteDuration prometheus.Histogram
	// WritesTotal counts bulk write attempts, labelled by status (success/error).
	WritesTotal *prometheus.CounterVec
	// BufferSize is a gauge showing how many events are currently buffered.
	BufferSize prometheus.Gauge
	// DuplicatesSkipped counts events that already existed in MongoDB (deduplicated).
	DuplicatesSkipped prometheus.Counter
}

// NewConsumer creates and registers all consumer metrics with the given registerer.
func NewConsumer(reg prometheus.Registerer) *Consumer {
	m := &Consumer{
		EventsConsumed: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "pulse_consumer_events_consumed_total",
			Help: "Total events consumed from Kafka, labelled by event_type.",
		}, []string{"event_type"}),

		DLQTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "pulse_consumer_dlq_total",
			Help: "Total events routed to the Dead Letter Queue.",
		}),

		WriteDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "pulse_consumer_write_duration_seconds",
			Help:    "MongoDB bulk write latency in seconds.",
			Buckets: prometheus.DefBuckets,
		}),

		WritesTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "pulse_consumer_writes_total",
			Help: "Total MongoDB bulk write attempts, labelled by status.",
		}, []string{"status"}),

		BufferSize: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "pulse_consumer_buffer_size",
			Help: "Current number of events buffered and waiting for the next flush.",
		}),

		DuplicatesSkipped: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "pulse_consumer_duplicates_skipped_total",
			Help: "Total duplicate events silently skipped by MongoDB $setOnInsert.",
		}),
	}

	reg.MustRegister(
		m.EventsConsumed,
		m.DLQTotal,
		m.WriteDuration,
		m.WritesTotal,
		m.BufferSize,
		m.DuplicatesSkipped,
	)

	return m
}
