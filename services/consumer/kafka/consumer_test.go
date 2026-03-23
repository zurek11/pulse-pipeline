package kafka

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"

	"github.com/zurek11/pulse-pipeline/services/consumer/models"
)

// mockWriter implements Writer for testing.
type mockWriter struct {
	added  []string // event IDs added
	flushErr error
	flushed int
	reset   bool
}

func (m *mockWriter) Add(event interface{}, eventID string) {
	m.added = append(m.added, eventID)
}

func (m *mockWriter) Flush(_ context.Context) (int64, error) {
	if m.flushErr != nil {
		return 0, m.flushErr
	}
	m.flushed++
	return int64(len(m.added)), nil
}

func (m *mockWriter) Len() int { return len(m.added) }

func (m *mockWriter) Reset() {
	m.reset = true
	m.added = m.added[:0]
}

// --- parseMessage tests ---

func TestParseMessage_ValidEvent(t *testing.T) {
	event := models.Event{
		EventID:    "evt-123",
		CustomerID: "user-1",
		EventType:  "page_view",
		Timestamp:  time.Now().UTC(),
	}
	data, _ := json.Marshal(event)
	msg := kafka.Message{Value: data}

	got, err := parseMessage(msg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.EventID != event.EventID {
		t.Errorf("expected event_id %q, got %q", event.EventID, got.EventID)
	}
	if got.CustomerID != event.CustomerID {
		t.Errorf("expected customer_id %q, got %q", event.CustomerID, got.CustomerID)
	}
	if got.ProcessedAt.IsZero() {
		t.Error("expected ProcessedAt to be set")
	}
}

func TestParseMessage_InvalidJSON(t *testing.T) {
	msg := kafka.Message{Value: []byte("not-json")}

	_, err := parseMessage(msg)
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
	if !strings.Contains(err.Error(), "unmarshal event") {
		t.Errorf("expected error to mention 'unmarshal event', got: %v", err)
	}
}

func TestParseMessage_EmptyPayload(t *testing.T) {
	msg := kafka.Message{Value: []byte("")}

	_, err := parseMessage(msg)
	if err == nil {
		t.Fatal("expected error for empty payload, got nil")
	}
}

// --- mockWriter behaviour tests ---

func TestMockWriter_AddAndLen(t *testing.T) {
	w := &mockWriter{}
	w.Add(nil, "evt-1")
	w.Add(nil, "evt-2")

	if w.Len() != 2 {
		t.Errorf("expected Len=2, got %d", w.Len())
	}
}

func TestMockWriter_ResetClearsBuffer(t *testing.T) {
	w := &mockWriter{}
	w.Add(nil, "evt-1")
	w.Reset()

	if w.Len() != 0 {
		t.Errorf("expected Len=0 after Reset, got %d", w.Len())
	}
	if !w.reset {
		t.Error("expected reset flag to be true")
	}
}
