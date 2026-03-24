// Load test: sends 10,000 events via POST /api/v1/track/batch.
// Usage: go run scripts/load-test/main.go
// Override API URL: API_URL=http://localhost:8080 go run scripts/load-test/main.go
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

const (
	totalEvents = 10_000
	batchSize   = 100
	concurrency = 5
)

var eventTypes = []string{"page_view", "click", "purchase", "add_to_cart", "search", "custom"}

type event struct {
	CustomerID string         `json:"customer_id"`
	EventType  string         `json:"event_type"`
	Properties map[string]any `json:"properties,omitempty"`
}

type batchRequest struct {
	Events []event `json:"events"`
}

func main() {
	apiURL := os.Getenv("API_URL")
	if apiURL == "" {
		apiURL = "http://localhost:8080"
	}
	endpoint := apiURL + "/api/v1/track/batch"

	numBatches := totalEvents / batchSize
	fmt.Printf("🚀 Load test: %d events via %d batches (batch_size=%d, concurrency=%d)\n",
		totalEvents, numBatches, batchSize, concurrency)
	fmt.Printf("   Endpoint: %s\n\n", endpoint)

	var (
		succeeded int64
		failed    int64
		latencies []time.Duration
		mu        sync.Mutex
	)

	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	start := time.Now()

	for i := 0; i < numBatches; i++ {
		sem <- struct{}{}
		wg.Add(1)
		go func(batchNum int) {
			defer wg.Done()
			defer func() { <-sem }()

			events := make([]event, batchSize)
			for j := range events {
				events[j] = event{
					CustomerID: fmt.Sprintf("user-%d", (batchNum*batchSize+j)%1000),
					EventType:  eventTypes[(batchNum+j)%len(eventTypes)],
					Properties: map[string]any{"batch": batchNum, "seq": j},
				}
			}

			payload, _ := json.Marshal(batchRequest{Events: events})

			t0 := time.Now()
			resp, err := http.Post(endpoint, "application/json", bytes.NewReader(payload)) //nolint:noctx
			latency := time.Since(t0)

			if err != nil {
				atomic.AddInt64(&failed, int64(batchSize))
				fmt.Fprintf(os.Stderr, "  ❌ batch %d failed: %v\n", batchNum, err)
				return
			}
			resp.Body.Close()

			if resp.StatusCode != http.StatusAccepted {
				atomic.AddInt64(&failed, int64(batchSize))
				fmt.Fprintf(os.Stderr, "  ❌ batch %d returned HTTP %d\n", batchNum, resp.StatusCode)
				return
			}

			atomic.AddInt64(&succeeded, int64(batchSize))
			mu.Lock()
			latencies = append(latencies, latency)
			mu.Unlock()
		}(i)
	}

	wg.Wait()
	elapsed := time.Since(start)

	// Compute latency percentiles.
	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	var p50, p99 time.Duration
	if n := len(latencies); n > 0 {
		p50 = latencies[n*50/100]
		p99 = latencies[n*99/100]
	}

	throughput := float64(succeeded) / elapsed.Seconds()

	fmt.Printf("📊 Results:\n")
	fmt.Printf("   Total events:  %d\n", totalEvents)
	fmt.Printf("   Succeeded:     %d\n", succeeded)
	fmt.Printf("   Failed:        %d\n", failed)
	fmt.Printf("   Duration:      %s\n", elapsed.Round(time.Millisecond))
	fmt.Printf("   Throughput:    %.0f events/sec\n", throughput)
	fmt.Printf("   Batch RTT p50: %s\n", p50.Round(time.Millisecond))
	fmt.Printf("   Batch RTT p99: %s\n", p99.Round(time.Millisecond))

	if throughput >= 500 {
		fmt.Printf("\n✅ Acceptance criteria met: %.0f events/sec ≥ 500 events/sec\n", throughput)
	} else {
		fmt.Printf("\n⚠️  Below target: %.0f events/sec < 500 events/sec\n", throughput)
		os.Exit(1)
	}
}
