package models

import (
	"strings"
	"testing"
)

func TestEventValidate(t *testing.T) {
	tests := []struct {
		name    string
		event   Event
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid page_view",
			event:   Event{CustomerID: "user-1", EventType: "page_view"},
			wantErr: false,
		},
		{
			name:    "valid purchase with properties",
			event:   Event{CustomerID: "user-2", EventType: "purchase", Properties: map[string]interface{}{"price": 99.99}},
			wantErr: false,
		},
		{
			name:    "valid all event types",
			event:   Event{CustomerID: "u", EventType: "click"},
			wantErr: false,
		},
		{
			name:    "valid add_to_cart",
			event:   Event{CustomerID: "u", EventType: "add_to_cart"},
			wantErr: false,
		},
		{
			name:    "valid search",
			event:   Event{CustomerID: "u", EventType: "search"},
			wantErr: false,
		},
		{
			name:    "valid custom",
			event:   Event{CustomerID: "u", EventType: "custom"},
			wantErr: false,
		},
		{
			name:    "missing customer_id",
			event:   Event{EventType: "page_view"},
			wantErr: true,
			errMsg:  "customer_id is required",
		},
		{
			name:    "customer_id too long",
			event:   Event{CustomerID: strings.Repeat("x", 257), EventType: "page_view"},
			wantErr: true,
			errMsg:  "customer_id exceeds 256 characters",
		},
		{
			name:    "missing event_type",
			event:   Event{CustomerID: "user-1"},
			wantErr: true,
			errMsg:  "event_type is required",
		},
		{
			name:    "invalid event_type",
			event:   Event{CustomerID: "user-1", EventType: "unknown"},
			wantErr: true,
			errMsg:  "invalid event_type: unknown",
		},
		{
			name:    "properties too many keys",
			event:   Event{CustomerID: "user-1", EventType: "page_view", Properties: makeProps(51)},
			wantErr: true,
			errMsg:  "properties exceeds 50 keys",
		},
		{
			name:    "properties invalid value type",
			event:   Event{CustomerID: "user-1", EventType: "page_view", Properties: map[string]interface{}{"nested": map[string]string{"key": "val"}}},
			wantErr: true,
			errMsg:  "invalid type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.event.Validate()
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.errMsg)
				}
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errMsg, err.Error())
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestEventSetDefaults(t *testing.T) {
	t.Run("generates event_id when empty", func(t *testing.T) {
		e := Event{CustomerID: "u", EventType: "click"}
		if err := e.SetDefaults(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if e.EventID == "" {
			t.Error("expected event_id to be set")
		}
		if !strings.HasPrefix(e.EventID, "evt_") {
			t.Errorf("expected event_id to start with 'evt_', got %q", e.EventID)
		}
	})

	t.Run("preserves existing event_id", func(t *testing.T) {
		e := Event{CustomerID: "u", EventType: "click", EventID: "evt_existing"}
		if err := e.SetDefaults(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if e.EventID != "evt_existing" {
			t.Errorf("expected event_id to remain %q, got %q", "evt_existing", e.EventID)
		}
	})

	t.Run("sets timestamp when zero", func(t *testing.T) {
		e := Event{CustomerID: "u", EventType: "click"}
		if err := e.SetDefaults(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if e.Timestamp.IsZero() {
			t.Error("expected Timestamp to be set")
		}
	})

	t.Run("always sets received_at", func(t *testing.T) {
		e := Event{CustomerID: "u", EventType: "click"}
		if err := e.SetDefaults(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if e.ReceivedAt.IsZero() {
			t.Error("expected ReceivedAt to be set")
		}
	})
}

func makeProps(n int) map[string]interface{} {
	m := make(map[string]interface{}, n)
	for i := 0; i < n; i++ {
		m[string(rune('a'+i%26))+strings.Repeat("x", i/26)] = i
	}
	return m
}
