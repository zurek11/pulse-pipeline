package models

import "time"

// EventContext holds optional device and session metadata.
type EventContext struct {
	Device    string `json:"device,omitempty" bson:"device,omitempty"`
	SessionID string `json:"session_id,omitempty" bson:"session_id,omitempty"`
	IP        string `json:"ip,omitempty" bson:"ip,omitempty"`
	UserAgent string `json:"user_agent,omitempty" bson:"user_agent,omitempty"`
}

// Event represents a tracking event consumed from Kafka.
type Event struct {
	EventID     string                 `json:"event_id" bson:"event_id"`
	CustomerID  string                 `json:"customer_id" bson:"customer_id"`
	EventType   string                 `json:"event_type" bson:"event_type"`
	Timestamp   time.Time              `json:"timestamp" bson:"timestamp"`
	Properties  map[string]interface{} `json:"properties,omitempty" bson:"properties,omitempty"`
	Context     *EventContext          `json:"context,omitempty" bson:"context,omitempty"`
	ReceivedAt  time.Time              `json:"received_at" bson:"received_at"`
	ProcessedAt time.Time              `json:"processed_at" bson:"processed_at"`
}
