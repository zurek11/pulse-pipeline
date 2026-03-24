package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zurek11/pulse-pipeline/services/api/models"
)

// mockProducer satisfies the KafkaProducer interface for testing.
type mockProducer struct {
	produceErr error
	calls      int
	lastKey    string
}

func (m *mockProducer) Produce(_ context.Context, key string, _ interface{}) error {
	m.calls++
	m.lastKey = key
	return m.produceErr
}

func (m *mockProducer) ProduceBatch(_ context.Context, events []*models.Event) error {
	for _, e := range events {
		m.calls++
		m.lastKey = e.CustomerID
	}
	return m.produceErr
}

func TestTrackHandler_success(t *testing.T) {
	mp := &mockProducer{}
	h := NewTrackHandler(mp, noopLogger())

	body := `{"customer_id":"user-1","event_type":"page_view"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/track", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Errorf("expected 202, got %d", rec.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["status"] != "accepted" {
		t.Errorf("expected status=accepted, got %q", resp["status"])
	}
	if resp["event_id"] == "" {
		t.Error("expected event_id in response")
	}
	if mp.calls != 1 {
		t.Errorf("expected 1 produce call, got %d", mp.calls)
	}
	if mp.lastKey != "user-1" {
		t.Errorf("expected kafka key=user-1, got %q", mp.lastKey)
	}
}

func TestTrackHandler_validationErrors(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		wantCode int
		wantMsg  string
	}{
		{
			name:     "missing customer_id",
			body:     `{"event_type":"page_view"}`,
			wantCode: http.StatusBadRequest,
			wantMsg:  "customer_id is required",
		},
		{
			name:     "missing event_type",
			body:     `{"customer_id":"user-1"}`,
			wantCode: http.StatusBadRequest,
			wantMsg:  "event_type is required",
		},
		{
			name:     "invalid event_type",
			body:     `{"customer_id":"user-1","event_type":"unknown"}`,
			wantCode: http.StatusBadRequest,
			wantMsg:  "invalid event_type",
		},
		{
			name:     "invalid JSON",
			body:     `{bad json}`,
			wantCode: http.StatusBadRequest,
			wantMsg:  "invalid JSON",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mp := &mockProducer{}
			h := NewTrackHandler(mp, noopLogger())

			req := httptest.NewRequest(http.MethodPost, "/api/v1/track", strings.NewReader(tt.body))
			rec := httptest.NewRecorder()

			h.ServeHTTP(rec, req)

			if rec.Code != tt.wantCode {
				t.Errorf("expected %d, got %d", tt.wantCode, rec.Code)
			}

			var resp map[string]string
			if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if resp["status"] != "error" {
				t.Errorf("expected status=error, got %q", resp["status"])
			}
			if !strings.Contains(resp["message"], tt.wantMsg) {
				t.Errorf("expected message to contain %q, got %q", tt.wantMsg, resp["message"])
			}
			if mp.calls != 0 {
				t.Error("producer should not be called on validation error")
			}
		})
	}
}

func TestTrackHandler_wrongMethod(t *testing.T) {
	mp := &mockProducer{}
	h := NewTrackHandler(mp, noopLogger())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/track", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rec.Code)
	}
}

func TestTrackHandler_kafkaError(t *testing.T) {
	mp := &mockProducer{produceErr: fmt.Errorf("broker unavailable")}
	h := NewTrackHandler(mp, noopLogger())

	body := `{"customer_id":"user-1","event_type":"click"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/track", strings.NewReader(body))
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rec.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["status"] != "error" {
		t.Errorf("expected status=error, got %q", resp["status"])
	}
}

func TestTrackHandler_preservesProvidedEventID(t *testing.T) {
	mp := &mockProducer{}
	h := NewTrackHandler(mp, noopLogger())

	body := `{"customer_id":"user-1","event_type":"search","event_id":"evt_custom-123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/track", strings.NewReader(body))
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Errorf("expected 202, got %d", rec.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["event_id"] != "evt_custom-123" {
		t.Errorf("expected event_id=evt_custom-123, got %q", resp["event_id"])
	}
}
