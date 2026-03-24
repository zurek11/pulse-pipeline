package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"

	kafkago "github.com/segmentio/kafka-go"

	"github.com/zurek11/pulse-pipeline/services/api/metrics"
	"github.com/zurek11/pulse-pipeline/services/api/models"
)

const queueCapacity = 10_000

// asyncMetrics is the subset of API metrics used by AsyncProducer.
type asyncMetrics interface {
	incProduceErrors()
	setQueueDepth(n float64)
}

// metricsAdapter adapts *metrics.API to asyncMetrics (nil-safe).
type metricsAdapter struct{ m *metrics.API }

func (a *metricsAdapter) incProduceErrors() {
	if a.m != nil {
		a.m.ProduceErrors.Inc()
	}
}
func (a *metricsAdapter) setQueueDepth(n float64) {
	if a.m != nil {
		a.m.QueueDepth.Set(n)
	}
}

// AsyncProducer writes events to Kafka without blocking the caller.
// Callers enqueue events via Produce / ProduceBatch and receive an immediate nil error.
// The background loop drains the queue and performs the actual Kafka write.
// Call Close() during graceful shutdown to drain and flush.
type AsyncProducer struct {
	writer  *kafkago.Writer
	queue   chan kafkago.Message
	metrics asyncMetrics
	logger  *slog.Logger
	wg      sync.WaitGroup
}

// NewAsyncProducer creates an AsyncProducer and starts the background drain loop.
func NewAsyncProducer(brokers []string, topic string, m *metrics.API, logger *slog.Logger) *AsyncProducer {
	ap := &AsyncProducer{
		writer: &kafkago.Writer{
			Addr:         kafkago.TCP(brokers...),
			Topic:        topic,
			Balancer:     &kafkago.Hash{},
			RequiredAcks: kafkago.RequireAll,
			MaxAttempts:  3,
		},
		queue:   make(chan kafkago.Message, queueCapacity),
		metrics: &metricsAdapter{m: m},
		logger:  logger,
	}

	ap.wg.Add(1)
	go ap.loop()

	return ap
}

// Produce serialises value as JSON and enqueues it for async Kafka delivery.
// Returns immediately — never blocks on Kafka.
func (ap *AsyncProducer) Produce(_ context.Context, key string, value interface{}) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal value: %w", err)
	}

	msg := kafkago.Message{Key: []byte(key), Value: data}

	select {
	case ap.queue <- msg:
		ap.metrics.setQueueDepth(float64(len(ap.queue)))
		return nil
	default:
		return fmt.Errorf("async producer queue full (%d)", queueCapacity)
	}
}

// ProduceBatch enqueues all events for async Kafka delivery in a single call.
// Returns immediately — never blocks on Kafka.
func (ap *AsyncProducer) ProduceBatch(_ context.Context, events []*models.Event) error {
	msgs := make([]kafkago.Message, 0, len(events))
	for i, e := range events {
		data, err := json.Marshal(e)
		if err != nil {
			return fmt.Errorf("marshal event %d: %w", i, err)
		}
		msgs = append(msgs, kafkago.Message{Key: []byte(e.CustomerID), Value: data})
	}

	for _, msg := range msgs {
		select {
		case ap.queue <- msg:
		default:
			return fmt.Errorf("async producer queue full (%d)", queueCapacity)
		}
	}

	ap.metrics.setQueueDepth(float64(len(ap.queue)))
	return nil
}

// Close drains the queue, flushes remaining messages to Kafka, and closes the writer.
// Must be called during graceful shutdown.
func (ap *AsyncProducer) Close() error {
	close(ap.queue)
	ap.wg.Wait()
	if err := ap.writer.Close(); err != nil {
		return fmt.Errorf("close kafka writer: %w", err)
	}
	return nil
}

// loop is the background goroutine that drains the queue and writes to Kafka.
// It batches whatever is available in the channel for efficiency.
func (ap *AsyncProducer) loop() {
	defer ap.wg.Done()

	for {
		// Block until at least one message is available (or channel closed).
		msg, ok := <-ap.queue
		if !ok {
			return
		}

		// Drain remaining messages without blocking (natural batching).
		batch := []kafkago.Message{msg}
		for len(batch) < 500 {
			select {
			case msg, ok = <-ap.queue:
				if !ok {
					// Channel closed — flush what we have and exit.
					ap.write(batch)
					return
				}
				batch = append(batch, msg)
			default:
				goto flush
			}
		}

	flush:
		ap.write(batch)
		ap.metrics.setQueueDepth(float64(len(ap.queue)))
	}
}

// write sends a batch of messages to Kafka, logging and counting any errors.
func (ap *AsyncProducer) write(batch []kafkago.Message) {
	if len(batch) == 0 {
		return
	}
	if err := ap.writer.WriteMessages(context.Background(), batch...); err != nil {
		ap.logger.Error("async kafka write failed",
			"batch_size", len(batch),
			"error", err,
		)
		ap.metrics.incProduceErrors()
		return
	}
	ap.logger.Debug("async kafka write completed", "batch_size", len(batch))
}
