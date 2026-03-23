package models

import (
	"crypto/rand"
	"fmt"
	"time"
)

// EventContext holds optional device and session metadata.
type EventContext struct {
	Device    string `json:"device,omitempty"`
	SessionID string `json:"session_id,omitempty"`
	IP        string `json:"ip,omitempty"`
	UserAgent string `json:"user_agent,omitempty"`
}

// Event represents a single tracking event.
type Event struct {
	EventID    string                 `json:"event_id"`
	CustomerID string                 `json:"customer_id"`
	EventType  string                 `json:"event_type"`
	Timestamp  time.Time              `json:"timestamp"`
	Properties map[string]interface{} `json:"properties,omitempty"`
	Context    *EventContext          `json:"context,omitempty"`
	ReceivedAt time.Time              `json:"received_at"`
}

var allowedEventTypes = map[string]bool{
	"page_view":   true,
	"click":       true,
	"purchase":    true,
	"add_to_cart": true,
	"search":      true,
	"custom":      true,
}

// Validate checks all required fields and constraints.
func (e *Event) Validate() error {
	if e.CustomerID == "" {
		return fmt.Errorf("customer_id is required")
	}
	if len(e.CustomerID) > 256 {
		return fmt.Errorf("customer_id exceeds 256 characters")
	}
	if e.EventType == "" {
		return fmt.Errorf("event_type is required")
	}
	if !allowedEventTypes[e.EventType] {
		return fmt.Errorf("invalid event_type: %s", e.EventType)
	}
	if len(e.Properties) > 50 {
		return fmt.Errorf("properties exceeds 50 keys")
	}
	for k, v := range e.Properties {
		if !isAllowedPropertyValue(v) {
			return fmt.Errorf("property %q has invalid type: must be string, number, or boolean", k)
		}
	}
	return nil
}

// SetDefaults fills in event_id (if missing) and timestamps.
// Returns an error only if random ID generation fails (extremely unlikely).
func (e *Event) SetDefaults() error {
	if e.EventID == "" {
		id, err := generateID()
		if err != nil {
			return fmt.Errorf("generate event_id: %w", err)
		}
		e.EventID = "evt_" + id
	}
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now().UTC()
	}
	e.ReceivedAt = time.Now().UTC()
	return nil
}

func isAllowedPropertyValue(v interface{}) bool {
	switch v.(type) {
	case string, bool, float64, int, int64, float32:
		return true
	default:
		return false
	}
}

func generateID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	// UUID v4 format
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:]), nil
}
