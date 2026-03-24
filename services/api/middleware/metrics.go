package middleware

import (
	"net/http"
	"strconv"
	"time"

	apimetrics "github.com/zurek11/pulse-pipeline/services/api/metrics"
)

// statusRecorder wraps http.ResponseWriter to capture the written status code.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

// Metrics returns middleware that records per-request latency and request counts.
// Pass nil to skip instrumentation (useful in tests).
func Metrics(m *apimetrics.API) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if m == nil {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

			next.ServeHTTP(rec, r)

			duration := time.Since(start).Seconds()
			path := r.URL.Path

			m.RequestDuration.WithLabelValues(r.Method, path).Observe(duration)
			m.RequestsTotal.WithLabelValues(r.Method, path, strconv.Itoa(rec.status)).Inc()
		})
	}
}
