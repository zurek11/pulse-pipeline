package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBatchHandler_success(t *testing.T) {
	mp := &mockProducer{}
	h := NewBatchHandler(mp, noopLogger(), nil)

	body := `{"events":[
		{"customer_id":"user-1","event_type":"page_view"},
		{"customer_id":"user-2","event_type":"click"},
		{"customer_id":"user-3","event_type":"purchase"}
	]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/track/batch", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Errorf("expected 202, got %d", rec.Code)
	}

	var resp batchResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Accepted != 3 {
		t.Errorf("expected accepted=3, got %d", resp.Accepted)
	}
	if len(resp.EventIDs) != 3 {
		t.Errorf("expected 3 event_ids, got %d", len(resp.EventIDs))
	}
	if mp.calls != 3 {
		t.Errorf("expected 3 produce calls, got %d", mp.calls)
	}
}

func TestBatchHandler_validationErrors(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		wantCode int
		wantMsg  string
	}{
		{
			name:     "empty events array",
			body:     `{"events":[]}`,
			wantCode: http.StatusBadRequest,
			wantMsg:  "must not be empty",
		},
		{
			name:     "missing events field",
			body:     `{}`,
			wantCode: http.StatusBadRequest,
			wantMsg:  "must not be empty",
		},
		{
			name:     "first event missing customer_id",
			body:     `{"events":[{"event_type":"page_view"}]}`,
			wantCode: http.StatusBadRequest,
			wantMsg:  "event[0]: customer_id is required",
		},
		{
			name:     "second event invalid type",
			body:     `{"events":[{"customer_id":"user-1","event_type":"page_view"},{"customer_id":"user-2","event_type":"bad_type"}]}`,
			wantCode: http.StatusBadRequest,
			wantMsg:  "event[1]:",
		},
		{
			name:     "invalid JSON",
			body:     `{bad}`,
			wantCode: http.StatusBadRequest,
			wantMsg:  "invalid JSON",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mp := &mockProducer{}
			h := NewBatchHandler(mp, noopLogger(), nil)

			req := httptest.NewRequest(http.MethodPost, "/api/v1/track/batch", strings.NewReader(tt.body))
			rec := httptest.NewRecorder()

			h.ServeHTTP(rec, req)

			if rec.Code != tt.wantCode {
				t.Errorf("expected %d, got %d", tt.wantCode, rec.Code)
			}
			var resp map[string]string
			if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
				t.Fatalf("decode: %v", err)
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

func TestBatchHandler_exceedsMaxSize(t *testing.T) {
	mp := &mockProducer{}
	h := NewBatchHandler(mp, noopLogger(), nil)

	// Build a batch of 101 events.
	events := make([]string, 101)
	for i := range events {
		events[i] = `{"customer_id":"user-1","event_type":"page_view"}`
	}
	body := `{"events":[` + strings.Join(events, ",") + `]}`

	req := httptest.NewRequest(http.MethodPost, "/api/v1/track/batch", strings.NewReader(body))
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
	var resp map[string]string
	_ = json.NewDecoder(rec.Body).Decode(&resp)
	if !strings.Contains(resp["message"], "exceeds maximum") {
		t.Errorf("unexpected message: %q", resp["message"])
	}
	if mp.calls != 0 {
		t.Error("producer should not be called")
	}
}

func TestBatchHandler_wrongMethod(t *testing.T) {
	h := NewBatchHandler(&mockProducer{}, noopLogger(), nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/track/batch", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rec.Code)
	}
}

func TestBatchHandler_kafkaError(t *testing.T) {
	mp := &mockProducer{produceErr: fmt.Errorf("kafka down")}
	h := NewBatchHandler(mp, noopLogger(), nil)

	body := `{"events":[{"customer_id":"user-1","event_type":"page_view"}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/track/batch", strings.NewReader(body))
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rec.Code)
	}
}

func TestBatchHandler_preservesProvidedEventIDs(t *testing.T) {
	mp := &mockProducer{}
	h := NewBatchHandler(mp, noopLogger(), nil)

	body := `{"events":[
		{"event_id":"evt_custom-1","customer_id":"user-1","event_type":"page_view"},
		{"event_id":"evt_custom-2","customer_id":"user-2","event_type":"click"}
	]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/track/batch", strings.NewReader(body))
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Errorf("expected 202, got %d", rec.Code)
	}
	var resp batchResponse
	_ = json.NewDecoder(rec.Body).Decode(&resp)
	if resp.EventIDs[0] != "evt_custom-1" {
		t.Errorf("expected evt_custom-1, got %q", resp.EventIDs[0])
	}
	if resp.EventIDs[1] != "evt_custom-2" {
		t.Errorf("expected evt_custom-2, got %q", resp.EventIDs[1])
	}
}
