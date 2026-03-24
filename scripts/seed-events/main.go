// Seed realistic e-commerce events into the pipeline.
// Usage: go run scripts/seed-events/main.go
// Override API URL: API_URL=http://localhost:8080 go run scripts/seed-events/main.go
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

type event struct {
	EventID    string         `json:"event_id,omitempty"`
	CustomerID string         `json:"customer_id"`
	EventType  string         `json:"event_type"`
	Properties map[string]any `json:"properties,omitempty"`
}

// seedEvents represents a realistic e-commerce session across three customers.
var seedEvents = []event{
	// Alice: browses, searches, buys
	{CustomerID: "alice-001", EventType: "page_view", Properties: map[string]any{"page": "/", "referrer": "google.com"}},
	{CustomerID: "alice-001", EventType: "search", Properties: map[string]any{"query": "winter jacket", "results_count": 24}},
	{CustomerID: "alice-001", EventType: "page_view", Properties: map[string]any{"page": "/products/winter-jacket-blue"}},
	{CustomerID: "alice-001", EventType: "click", Properties: map[string]any{"element": "size-selector", "value": "M"}},
	{CustomerID: "alice-001", EventType: "add_to_cart", Properties: map[string]any{"product_id": "prod-wj-blue-M", "price": 89.99, "currency": "EUR"}},
	{CustomerID: "alice-001", EventType: "page_view", Properties: map[string]any{"page": "/cart"}},
	{CustomerID: "alice-001", EventType: "purchase", Properties: map[string]any{"order_id": "ord-001", "total": 89.99, "currency": "EUR", "items": 1}},
	{CustomerID: "alice-001", EventType: "custom", Properties: map[string]any{"action": "newsletter_signup", "source": "checkout"}},

	// Bob: browses, adds to cart, abandons
	{CustomerID: "bob-002", EventType: "page_view", Properties: map[string]any{"page": "/", "referrer": "newsletter"}},
	{CustomerID: "bob-002", EventType: "page_view", Properties: map[string]any{"page": "/sale"}},
	{CustomerID: "bob-002", EventType: "click", Properties: map[string]any{"element": "filter-category", "value": "shoes"}},
	{CustomerID: "bob-002", EventType: "search", Properties: map[string]any{"query": "running shoes", "results_count": 42}},
	{CustomerID: "bob-002", EventType: "add_to_cart", Properties: map[string]any{"product_id": "prod-rs-blk-42", "price": 129.00, "currency": "EUR"}},
	{CustomerID: "bob-002", EventType: "click", Properties: map[string]any{"element": "remove-from-cart", "product_id": "prod-rs-blk-42"}},
	{CustomerID: "bob-002", EventType: "custom", Properties: map[string]any{"action": "wishlist_add", "product_id": "prod-rs-blk-42"}},

	// Carol: repeat buyer, multi-item order
	{CustomerID: "carol-003", EventType: "page_view", Properties: map[string]any{"page": "/account/orders"}},
	{CustomerID: "carol-003", EventType: "search", Properties: map[string]any{"query": "gift wrap", "results_count": 5}},
	{CustomerID: "carol-003", EventType: "add_to_cart", Properties: map[string]any{"product_id": "prod-gw-01", "price": 4.99, "currency": "EUR"}},
	{CustomerID: "carol-003", EventType: "add_to_cart", Properties: map[string]any{"product_id": "prod-scarf-red", "price": 24.99, "currency": "EUR"}},
	{CustomerID: "carol-003", EventType: "purchase", Properties: map[string]any{"order_id": "ord-002", "total": 29.98, "currency": "EUR", "items": 2}},
}

func main() {
	apiURL := os.Getenv("API_URL")
	if apiURL == "" {
		apiURL = "http://localhost:8080"
	}
	endpoint := apiURL + "/api/v1/track"

	fmt.Printf("🌱 Seeding %d events to %s\n\n", len(seedEvents), endpoint)

	ok, errs := 0, 0
	for _, e := range seedEvents {
		payload, _ := json.Marshal(e)
		resp, err := http.Post(endpoint, "application/json", bytes.NewReader(payload)) //nolint:noctx
		if err != nil {
			fmt.Fprintf(os.Stderr, "  ❌ %-12s  %-12s — %v\n", e.EventType, e.CustomerID, err)
			errs++
			continue
		}
		resp.Body.Close()

		if resp.StatusCode == http.StatusAccepted {
			fmt.Printf("  ✅ %-12s  %s\n", e.EventType, e.CustomerID)
			ok++
		} else {
			fmt.Fprintf(os.Stderr, "  ❌ %-12s  %-12s — HTTP %d\n", e.EventType, e.CustomerID, resp.StatusCode)
			errs++
		}
		time.Sleep(50 * time.Millisecond) // gentle pacing to avoid flooding
	}

	fmt.Printf("\n📊 Done: %d accepted, %d failed\n", ok, errs)
	if errs > 0 {
		os.Exit(1)
	}
}
