package metrics

import "github.com/prometheus/client_golang/prometheus"

// API holds all Prometheus metrics for the API service.
type API struct {
	// RequestsTotal counts every HTTP request, labelled by method, path, and status code.
	RequestsTotal *prometheus.CounterVec
	// RequestDuration tracks HTTP request latency as a histogram.
	RequestDuration *prometheus.HistogramVec
	// EventsProduced counts events successfully queued for Kafka, labelled by event_type.
	EventsProduced *prometheus.CounterVec
	// ValidationErrors counts requests rejected due to invalid payload.
	ValidationErrors prometheus.Counter
	// ProduceErrors counts events that failed to reach Kafka in the async background loop.
	ProduceErrors prometheus.Counter
	// QueueDepth is a gauge showing the current async producer queue length.
	QueueDepth prometheus.Gauge
}

// NewAPI creates and registers all API metrics with the given registerer.
func NewAPI(reg prometheus.Registerer) *API {
	m := &API{
		RequestsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "pulse_api_requests_total",
			Help: "Total HTTP requests received, labelled by method, path, and status code.",
		}, []string{"method", "path", "status"}),

		RequestDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "pulse_api_request_duration_seconds",
			Help:    "HTTP request latency in seconds.",
			Buckets: prometheus.DefBuckets,
		}, []string{"method", "path"}),

		EventsProduced: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "pulse_api_events_produced_total",
			Help: "Total events successfully queued for Kafka, labelled by event_type.",
		}, []string{"event_type"}),

		ValidationErrors: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "pulse_api_validation_errors_total",
			Help: "Total requests rejected due to payload validation failure.",
		}),

		ProduceErrors: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "pulse_api_produce_errors_total",
			Help: "Total events that failed Kafka write in the async producer loop.",
		}),

		QueueDepth: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "pulse_api_queue_depth",
			Help: "Current number of events waiting in the async producer queue.",
		}),
	}

	reg.MustRegister(
		m.RequestsTotal,
		m.RequestDuration,
		m.EventsProduced,
		m.ValidationErrors,
		m.ProduceErrors,
		m.QueueDepth,
	)

	return m
}
