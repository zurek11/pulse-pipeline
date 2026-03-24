package handlers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/zurek11/pulse-pipeline/services/api/middleware"
	"github.com/zurek11/pulse-pipeline/services/api/models"
)

const (
	maxBatchSize      = 100
	maxBatchBodyBytes = 1024 * 1024 // 1 MB
)

type batchRequest struct {
	Events []models.Event `json:"events"`
}

type batchResponse struct {
	Accepted int      `json:"accepted"`
	EventIDs []string `json:"event_ids"`
}

// BatchHandler handles POST /api/v1/track/batch.
type BatchHandler struct {
	producer KafkaProducer
	logger   *slog.Logger
}

// NewBatchHandler constructs a BatchHandler with the given producer and logger.
func NewBatchHandler(producer KafkaProducer, logger *slog.Logger) *BatchHandler {
	return &BatchHandler{producer: producer, logger: logger}
}

func (h *BatchHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	requestID := middleware.GetRequestID(ctx)

	if r.Method != http.MethodPost {
		h.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxBatchBodyBytes)

	var req batchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	if len(req.Events) == 0 {
		h.writeError(w, http.StatusBadRequest, "events must not be empty")
		return
	}
	if len(req.Events) > maxBatchSize {
		h.writeError(w, http.StatusBadRequest,
			fmt.Sprintf("batch exceeds maximum of %d events", maxBatchSize))
		return
	}

	// Validate and set defaults for every event first — fail fast on first error.
	for i := range req.Events {
		if err := req.Events[i].Validate(); err != nil {
			h.logger.InfoContext(ctx, "batch event validation failed",
				"request_id", requestID,
				"index", i,
				"error", err,
			)
			h.writeError(w, http.StatusBadRequest,
				fmt.Sprintf("event[%d]: %s", i, err.Error()))
			return
		}
		if err := req.Events[i].SetDefaults(); err != nil {
			h.logger.ErrorContext(ctx, "failed to set event defaults",
				"request_id", requestID,
				"index", i,
				"error", err,
			)
			h.writeError(w, http.StatusInternalServerError, "failed to process event")
			return
		}
	}

	// Produce all events to Kafka.
	eventIDs := make([]string, 0, len(req.Events))
	for i := range req.Events {
		if err := h.producer.Produce(ctx, req.Events[i].CustomerID, &req.Events[i]); err != nil {
			h.logger.ErrorContext(ctx, "failed to produce batch event",
				"request_id", requestID,
				"index", i,
				"event_id", req.Events[i].EventID,
				"error", err,
			)
			h.writeError(w, http.StatusInternalServerError, "failed to produce to Kafka")
			return
		}
		eventIDs = append(eventIDs, req.Events[i].EventID)
	}

	h.logger.InfoContext(ctx, "batch accepted",
		"request_id", requestID,
		"count", len(eventIDs),
	)

	h.writeJSON(w, http.StatusAccepted, batchResponse{
		Accepted: len(eventIDs),
		EventIDs: eventIDs,
	})
}

func (h *BatchHandler) writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func (h *BatchHandler) writeError(w http.ResponseWriter, status int, message string) {
	h.writeJSON(w, status, map[string]string{
		"status":  "error",
		"message": message,
	})
}
