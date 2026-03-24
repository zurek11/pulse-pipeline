package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"

	"github.com/zurek11/pulse-pipeline/services/consumer/models"
)

// --- mockWriter implements Writer ---

type mockWriter struct {
	added      []string
	flushErr   error
	failTimes  int // fail this many consecutive Flush calls before succeeding
	flushCount int
	reset      bool
}

func (m *mockWriter) Add(_ interface{}, eventID string) { m.added = append(m.added, eventID) }

func (m *mockWriter) Flush(_ context.Context) (int64, error) {
	m.flushCount++
	if m.failTimes > 0 {
		m.failTimes--
		return 0, m.flushErr
	}
	return int64(len(m.added)), nil
}

func (m *mockWriter) Len() int { return len(m.added) }

func (m *mockWriter) Reset() {
	m.reset = true
	m.added = m.added[:0]
}

// --- mockDLQ implements dlqSender ---

type mockDLQ struct {
	sent     []kafka.Message
	writeErr error
}

func (m *mockDLQ) WriteMessages(_ context.Context, msgs ...kafka.Message) error {
	if m.writeErr != nil {
		return m.writeErr
	}
	m.sent = append(m.sent, msgs...)
	return nil
}

func (m *mockDLQ) Close() error { return nil }

// --- parseMessage tests ---

func TestParseMessage_ValidEvent(t *testing.T) {
	event := models.Event{
		EventID:    "evt-123",
		CustomerID: "user-1",
		EventType:  "page_view",
		Timestamp:  time.Now().UTC(),
	}
	data, _ := json.Marshal(event)

	got, err := parseMessage(kafka.Message{Value: data})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.EventID != event.EventID {
		t.Errorf("expected event_id %q, got %q", event.EventID, got.EventID)
	}
	if got.ProcessedAt.IsZero() {
		t.Error("expected ProcessedAt to be set")
	}
}

func TestParseMessage_InvalidJSON(t *testing.T) {
	_, err := parseMessage(kafka.Message{Value: []byte("not-json")})
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
	if !strings.Contains(err.Error(), "unmarshal event") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestParseMessage_EmptyPayload(t *testing.T) {
	_, err := parseMessage(kafka.Message{Value: []byte("")})
	if err == nil {
		t.Fatal("expected error for empty payload, got nil")
	}
}

// --- flushWithRetry tests ---

func TestFlushWithRetry_SuccessOnFirstAttempt(t *testing.T) {
	w := &mockWriter{}
	allFailed := flushWithRetry(context.Background(), w, slog.Default(), nil)
	if allFailed {
		t.Error("expected success, got all-failed")
	}
	if w.flushCount != 1 {
		t.Errorf("expected 1 flush attempt, got %d", w.flushCount)
	}
}

func TestFlushWithRetry_AllRetriesExhausted(t *testing.T) {
	w := &mockWriter{flushErr: fmt.Errorf("mongodb unavailable"), failTimes: maxRetries + 10}
	allFailed := flushWithRetry(context.Background(), w, slog.Default(), nil)
	if !allFailed {
		t.Error("expected all-failed, got success")
	}
	if w.flushCount != maxRetries {
		t.Errorf("expected %d flush attempts, got %d", maxRetries, w.flushCount)
	}
}

func TestFlushWithRetry_SuccessOnSecondAttempt(t *testing.T) {
	w := &mockWriter{flushErr: fmt.Errorf("transient error"), failTimes: 1}
	allFailed := flushWithRetry(context.Background(), w, slog.Default(), nil)
	if allFailed {
		t.Error("expected eventual success, got all-failed")
	}
	if w.flushCount != 2 {
		t.Errorf("expected 2 flush attempts, got %d", w.flushCount)
	}
}

// --- DLQ routing tests ---

func TestSendToDLQ_ForwardsMessageToDLQWriter(t *testing.T) {
	dlq := &mockDLQ{}
	c := &Consumer{dlq: dlq, logger: slog.Default()}

	msg := kafka.Message{Key: []byte("user-1"), Value: []byte(`{"event_id":"evt-1"}`)}

	if err := c.sendToDLQ(context.Background(), msg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(dlq.sent) != 1 {
		t.Fatalf("expected 1 DLQ message, got %d", len(dlq.sent))
	}
	if string(dlq.sent[0].Value) != string(msg.Value) {
		t.Errorf("DLQ message value mismatch: got %q", dlq.sent[0].Value)
	}
}

func TestSendToDLQ_ReturnsErrorWhenDLQWriteFails(t *testing.T) {
	dlq := &mockDLQ{writeErr: fmt.Errorf("kafka unavailable")}
	c := &Consumer{dlq: dlq, logger: slog.Default()}

	err := c.sendToDLQ(context.Background(), kafka.Message{Value: []byte("x")})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "write to DLQ") {
		t.Errorf("unexpected error: %v", err)
	}
}
