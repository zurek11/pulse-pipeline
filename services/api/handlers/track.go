package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/zurek11/pulse-pipeline/services/api/metrics"
	"github.com/zurek11/pulse-pipeline/services/api/middleware"
	"github.com/zurek11/pulse-pipeline/services/api/models"
)

const maxBodyBytes = 64 * 1024 // 64 KB

// KafkaProducer is the interface the TrackHandler uses to emit events.
// Defined here at the point of use — implemented by kafka.AsyncProducer.
type KafkaProducer interface {
	Produce(ctx context.Context, key string, value interface{}) error
}

// TrackHandler handles POST /api/v1/track.
type TrackHandler struct {
	producer KafkaProducer
	logger   *slog.Logger
	metrics  *metrics.API
}

// NewTrackHandler constructs a TrackHandler with the given producer, logger, and metrics.
// Pass nil for metrics to disable instrumentation (e.g. in tests).
func NewTrackHandler(producer KafkaProducer, logger *slog.Logger, m *metrics.API) *TrackHandler {
	return &TrackHandler{
		producer: producer,
		logger:   logger,
		metrics:  m,
	}
}

func (h *TrackHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	requestID := middleware.GetRequestID(ctx)

	if r.Method != http.MethodPost {
		h.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)

	var event models.Event
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	if err := event.Validate(); err != nil {
		h.logger.InfoContext(ctx, "event validation failed",
			"request_id", requestID,
			"error", err,
		)
		if h.metrics != nil {
			h.metrics.ValidationErrors.Inc()
		}
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := event.SetDefaults(); err != nil {
		h.logger.ErrorContext(ctx, "failed to set event defaults",
			"request_id", requestID,
			"error", err,
		)
		h.writeError(w, http.StatusInternalServerError, "failed to process event")
		return
	}

	if err := h.producer.Produce(ctx, event.CustomerID, &event); err != nil {
		h.logger.ErrorContext(ctx, "failed to produce event",
			"request_id", requestID,
			"event_id", event.EventID,
			"error", err,
		)
		h.writeError(w, http.StatusInternalServerError, "failed to produce to Kafka")
		return
	}

	if h.metrics != nil {
		h.metrics.EventsProduced.WithLabelValues(event.EventType).Inc()
	}

	h.logger.InfoContext(ctx, "event accepted",
		"request_id", requestID,
		"event_id", event.EventID,
		"event_type", event.EventType,
		"customer_id", event.CustomerID,
	)

	h.writeJSON(w, http.StatusAccepted, map[string]string{
		"status":   "accepted",
		"event_id": event.EventID,
	})
}

func (h *TrackHandler) writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func (h *TrackHandler) writeError(w http.ResponseWriter, status int, message string) {
	h.writeJSON(w, status, map[string]string{
		"status":  "error",
		"message": message,
	})
}
