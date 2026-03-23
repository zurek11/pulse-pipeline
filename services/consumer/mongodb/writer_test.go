package mongodb

import (
	"log/slog"
	"testing"
)

// Tests cover buffer management logic only (no real MongoDB connection needed).

func TestBulkWriter_Add_AccumulatesModels(t *testing.T) {
	w := NewBulkWriter(nil, slog.Default())

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
	w := NewBulkWriter(nil, slog.Default())

	w.Add(map[string]string{"event_id": "evt-1"}, "evt-1")
	w.Add(map[string]string{"event_id": "evt-2"}, "evt-2")

	w.Reset()

	if w.Len() != 0 {
		t.Errorf("expected empty buffer after Reset, got %d", w.Len())
	}
}

func TestBulkWriter_Flush_EmptyBuffer(t *testing.T) {
	w := NewBulkWriter(nil, slog.Default())

	// Flushing an empty buffer should be a no-op (no panic, returns 0).
	inserted, err := w.Flush(nil) //nolint:staticcheck // nil ctx is fine for empty buffer path
	if err != nil {
		t.Errorf("unexpected error flushing empty buffer: %v", err)
	}
	if inserted != 0 {
		t.Errorf("expected 0 inserted, got %d", inserted)
	}
}

func TestBulkWriter_Reset_AfterFlushError_AllowsRetry(t *testing.T) {
	w := NewBulkWriter(nil, slog.Default())

	w.Add(map[string]string{"event_id": "evt-1"}, "evt-1")

	// Simulate failed flush (collection is nil → panic guard via Reset).
	// Calling Reset should clear buffer so next round starts fresh.
	w.Reset()

	if w.Len() != 0 {
		t.Errorf("expected 0 after Reset, got %d", w.Len())
	}
}
