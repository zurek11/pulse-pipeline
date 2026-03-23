package middleware

import (
	"context"
	"crypto/rand"
	"fmt"
	"net/http"
)

type contextKey string

const requestIDKey contextKey = "request_id"

// RequestID generates a random request ID, stores it in the request context,
// and sets the X-Request-ID response header.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := generateRequestID()
		ctx := context.WithValue(r.Context(), requestIDKey, id)
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetRequestID retrieves the request ID from the context. Returns an empty
// string if no ID is present.
func GetRequestID(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}

func generateRequestID() string {
	b := make([]byte, 8)
	rand.Read(b) //nolint:errcheck — rand.Read never fails on supported platforms
	return fmt.Sprintf("%x", b)
}
