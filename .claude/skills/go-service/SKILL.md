---
name: go-service
description: 'REQUIRED when creating any new .go file or package. Invoke this skill before writing Go code.'
allowed-tools: Read, Write, Edit, Glob, Grep, Bash(go:*), Bash(make:*)
---

# Go Service Patterns

## When to Use

- Creating a new Go file or package
- Adding a new HTTP handler
- Writing any Go function or struct

## HTTP Handler Template

```go
package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

type TrackHandler struct {
	producer KafkaProducer
	logger   *slog.Logger
}

func NewTrackHandler(producer KafkaProducer, logger *slog.Logger) *TrackHandler {
	return &TrackHandler{
		producer: producer,
		logger:   logger,
	}
}

func (h *TrackHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	requestID := middleware.GetRequestID(ctx)

	if r.Method != http.MethodPost {
		h.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var event models.Event
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if err := event.Validate(); err != nil {
		h.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.producer.Produce(ctx, event); err != nil {
		h.logger.ErrorContext(ctx, "failed to produce event",
			"request_id", requestID,
			"error", err,
		)
		h.writeError(w, http.StatusInternalServerError, "failed to process event")
		return
	}

	h.writeJSON(w, http.StatusAccepted, map[string]string{
		"status":   "accepted",
		"event_id": event.EventID,
	})
}

func (h *TrackHandler) writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *TrackHandler) writeError(w http.ResponseWriter, status int, message string) {
	h.writeJSON(w, status, map[string]string{
		"status":  "error",
		"message": message,
	})
}
```

## Graceful Shutdown Template

```go
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	// Initialize dependencies
	// producer := kafka.NewProducer(...)
	// mongoClient := mongodb.NewClient(...)

	mux := http.NewServeMux()
	// Register routes...

	server := &http.Server{
		Addr:         ":8080",
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Start server in goroutine
	go func() {
		logger.Info("server starting", "addr", server.Addr)
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			logger.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit

	logger.Info("shutdown signal received", "signal", sig)

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 1. Stop accepting new HTTP connections
	if err := server.Shutdown(ctx); err != nil {
		logger.Error("server shutdown error", "error", err)
	}

	// 2. Flush Kafka producer
	// producer.Close()

	// 3. Close MongoDB connection
	// mongoClient.Disconnect(ctx)

	logger.Info("server stopped gracefully")
}
```

## Struct with Validation Template

```go
package models

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

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

func (e *Event) Validate() error {
	if e.CustomerID == "" {
		return fmt.Errorf("customer_id is required")
	}
	if len(e.CustomerID) > 256 {
		return fmt.Errorf("customer_id exceeds 256 characters")
	}
	if !allowedEventTypes[e.EventType] {
		return fmt.Errorf("invalid event_type: %s", e.EventType)
	}
	if len(e.Properties) > 50 {
		return fmt.Errorf("properties exceeds 50 keys")
	}
	return nil
}

func (e *Event) SetDefaults() {
	if e.EventID == "" {
		e.EventID = "evt_" + uuid.New().String()
	}
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now().UTC()
	}
	e.ReceivedAt = time.Now().UTC()
}
```

## Rules

1. Standard library first — `net/http`, `log/slog`, `encoding/json`
2. No frameworks (no Gin, Echo, Fiber)
3. All functions must have explicit return types
4. Always check errors — `if err != nil { return fmt.Errorf("context: %w", err) }`
5. Pass `context.Context` as first parameter
6. Graceful shutdown in every `main.go` — SIGINT/SIGTERM handling
7. Dependency injection via constructors (`NewXxx(deps) *Xxx`)
8. No global variables or init() functions
9. Interfaces at point of use, not at implementation
10. Package names: short, lowercase, singular (e.g., `kafka` not `kafkaClient`)
11. File names: snake_case (e.g., `event_handler.go`)
12. Structured logging with slog — always include context (request_id, event_id)
