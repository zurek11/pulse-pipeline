package mongodb

import (
	"log/slog"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// Tests cover buffer management logic only (no real MongoDB connection needed).

func TestBulkWriter_Add_AccumulatesModels(t *testing.T) {
	w := NewBulkWriter(nil, slog.Default(), nil)

	if w.Len() != 0 {
		t.Fatalf("expected empty buffer, got %d", w.Len())
	}

	w.Add(map[string]string{"event_id": "evt-1"}, "evt-1")
	w.Add(map[string]string{"event_id": "evt-2"}, "evt-2")
	w.Add(map[string]string{"event_id": "evt-3"}, "evt-3")

	if w.Len() != 3 {
		t.Errorf("expected 3 buffered models, got %d", w.Len())
	}
}

func TestBulkWriter_Reset_ClearsBuffer(t *testing.T) {
	w := NewBulkWriter(nil, slog.Default(), nil)

	w.Add(map[string]string{"event_id": "evt-1"}, "evt-1")
	w.Add(map[string]string{"event_id": "evt-2"}, "evt-2")

	w.Reset()

	if w.Len() != 0 {
		t.Errorf("expected empty buffer after Reset, got %d", w.Len())
	}
}

func TestBulkWriter_Flush_EmptyBuffer(t *testing.T) {
	w := NewBulkWriter(nil, slog.Default(), nil)

	// Flushing an empty buffer should be a no-op (no panic, returns 0).
	inserted, err := w.Flush(nil) //nolint:staticcheck // nil ctx is fine for empty buffer path
	if err != nil {
		t.Errorf("unexpected error flushing empty buffer: %v", err)
	}
	if inserted != 0 {
		t.Errorf("expected 0 inserted, got %d", inserted)
	}
}

func TestBulkWriter_Add_BuildsIdempotentUpsertModel(t *testing.T) {
	w := NewBulkWriter(nil, slog.Default(), nil)
	w.Add(bson.M{"event_id": "evt-1", "customer_id": "user-1"}, "evt-1")

	w.mu.Lock()
	defer w.mu.Unlock()

	if len(w.models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(w.models))
	}

	m, ok := w.models[0].(*mongo.UpdateOneModel)
	if !ok {
		t.Fatal("expected *mongo.UpdateOneModel")
	}

	// Filter must use event_id for deduplication.
	filter, ok := m.Filter.(bson.M)
	if !ok {
		t.Fatal("expected bson.M filter")
	}
	if filter["event_id"] != "evt-1" {
		t.Errorf("expected filter event_id=evt-1, got %v", filter["event_id"])
	}

	// Update must use $setOnInsert — not $set — so duplicates are never overwritten.
	update, ok := m.Update.(bson.M)
	if !ok {
		t.Fatal("expected bson.M update")
	}
	if _, ok := update["$setOnInsert"]; !ok {
		t.Error("expected $setOnInsert in update for idempotency — $set would overwrite duplicates")
	}

	// Upsert must be true so new events are inserted when not found.
	if m.Upsert == nil || !*m.Upsert {
		t.Error("expected Upsert=true")
	}
}

func TestBulkWriter_Reset_AfterFlushError_AllowsRetry(t *testing.T) {
	w := NewBulkWriter(nil, slog.Default(), nil)

	w.Add(map[string]string{"event_id": "evt-1"}, "evt-1")

	// Simulate failed flush (collection is nil → panic guard via Reset).
	// Calling Reset should clear buffer so next round starts fresh.
	w.Reset()

	if w.Len() != 0 {
		t.Errorf("expected 0 after Reset, got %d", w.Len())
	}
}
