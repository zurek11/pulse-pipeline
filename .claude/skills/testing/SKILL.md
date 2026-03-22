---
name: testing
description: 'REQUIRED when creating or modifying _test.go files. Invoke this skill before writing tests.'
allowed-tools: Read, Write, Edit, Glob, Grep, Bash(go:*)
---

# Go Testing Patterns

## Table-Driven Tests

```go
func TestEventValidation(t *testing.T) {
	tests := []struct {
		name    string
		event   models.Event
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid event",
			event:   models.Event{CustomerID: "user-1", EventType: "page_view"},
			wantErr: false,
		},
		{
			name:    "missing customer_id",
			event:   models.Event{EventType: "page_view"},
			wantErr: true,
			errMsg:  "customer_id is required",
		},
		{
			name:    "invalid event_type",
			event:   models.Event{CustomerID: "user-1", EventType: "invalid"},
			wantErr: true,
			errMsg:  "invalid event_type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.event.Validate()
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errMsg)
				} else if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errMsg, err.Error())
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}
```

## HTTP Handler Test

```go
func TestTrackHandler(t *testing.T) {
	mockProducer := &MockProducer{}
	handler := handlers.NewTrackHandler(mockProducer, slog.Default())

	body := `{"customer_id":"user-1","event_type":"page_view"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/track", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Errorf("expected status 202, got %d", rec.Code)
	}
}
```

## Rules

1. Test files: `*_test.go` in same package as code under test
2. Table-driven tests for validation and transformation logic
3. `httptest` for handler tests — no real HTTP server needed
4. Mock interfaces for Kafka producer and MongoDB writer
5. Use `t.Run()` for subtests — clear output per case
6. No external test dependencies (no testify) — use stdlib `testing`
7. Run: `go test ./... -v -race` (verbose + race detector)
8. Test names: `TestXxx_description` (e.g., `TestEventValidation_missingCustomerID`)
