package mongodb

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/zurek11/pulse-pipeline/services/consumer/metrics"
)

// BulkWriter batches events and writes them to MongoDB using idempotent upserts.
// Safe for concurrent use.
type BulkWriter struct {
	collection *mongo.Collection
	logger     *slog.Logger
	metrics    *metrics.Consumer
	mu         sync.Mutex
	models     []mongo.WriteModel
}

// NewBulkWriter creates a new BulkWriter targeting the given collection.
// Pass nil for metrics to disable instrumentation (e.g. in tests).
func NewBulkWriter(collection *mongo.Collection, logger *slog.Logger, m *metrics.Consumer) *BulkWriter {
	return &BulkWriter{
		collection: collection,
		logger:     logger,
		metrics:    m,
		models:     make([]mongo.WriteModel, 0),
	}
}

// Add appends an idempotent upsert model for the given event to the buffer.
// If event_id already exists in MongoDB, the document will not be overwritten.
func (w *BulkWriter) Add(event interface{}, eventID string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	model := mongo.NewUpdateOneModel().
		SetFilter(bson.M{"event_id": eventID}).
		SetUpdate(bson.M{"$setOnInsert": event}).
		SetUpsert(true)

	w.models = append(w.models, model)
}

// Flush writes all buffered events to MongoDB and clears the buffer.
// Returns the count of newly inserted events (duplicates are not counted).
// If the write fails, the buffer is NOT cleared so the caller can retry.
func (w *BulkWriter) Flush(ctx context.Context) (int64, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if len(w.models) == 0 {
		return 0, nil
	}

	start := time.Now()

	opts := options.BulkWrite().SetOrdered(false)
	result, err := w.collection.BulkWrite(ctx, w.models, opts)

	duration := time.Since(start).Seconds()

	if err != nil {
		return 0, fmt.Errorf("bulk write: %w", err)
	}

	if w.metrics != nil {
		w.metrics.WriteDuration.Observe(duration)
		w.metrics.DuplicatesSkipped.Add(float64(result.MatchedCount))
	}

	w.logger.Info("bulk write completed",
		"inserted", result.UpsertedCount,
		"duplicates_skipped", result.MatchedCount,
		"batch_size", len(w.models),
		"duration_ms", duration*1000,
	)

	w.models = w.models[:0]
	return result.UpsertedCount, nil
}

// Len returns the number of events currently buffered.
func (w *BulkWriter) Len() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return len(w.models)
}

// Reset clears the buffer without writing to MongoDB.
// Used when routing a failed batch to the DLQ.
func (w *BulkWriter) Reset() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.models = w.models[:0]
}
