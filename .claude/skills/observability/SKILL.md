---
name: observability
description: 'REQUIRED when adding Prometheus metrics, Grafana dashboards, or alerting. Invoke this skill before observability changes.'
allowed-tools: Read, Write, Edit, Glob, Grep
---

# Observability Patterns

## Prometheus Metrics in Go

```go
import "github.com/prometheus/client_golang/prometheus"

var (
	eventsProduced = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "pulse_api_events_produced_total",
			Help: "Total events produced to Kafka",
		},
		[]string{"event_type"},
	)
	requestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "pulse_api_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)
)

func init() {
	prometheus.MustRegister(eventsProduced, requestDuration)
}
```

## Metric Naming Convention

- Prefix: `pulse_api_` or `pulse_consumer_`
- Type suffix: `_total` (counter), `_seconds` (histogram/duration), no suffix (gauge)
- Labels: lowercase, snake_case, max 3 labels per metric

## Grafana Dashboard

- Dashboard JSON in `monitoring/grafana/dashboards/pipeline.json`
- Auto-provisioned via `monitoring/grafana/provisioning/`
- 8 panels: events/sec, API latency, consumer lag, write latency, error rate, events by type, buffer size, DLQ count

## Rules

1. Use `prometheus/client_golang` — official Go Prometheus client
2. Expose `/metrics` endpoint on both API (8080) and consumer (8081)
3. Counter for counts (requests, events, errors), Histogram for latency, Gauge for current state (lag, buffer)
4. Dashboard as JSON code — version controlled, auto-provisioned
5. Prometheus scrapes every 15 seconds
6. Grafana admin credentials: admin/admin (dev only)
