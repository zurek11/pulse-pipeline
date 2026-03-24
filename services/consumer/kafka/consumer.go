package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/segmentio/kafka-go"

	"github.com/zurek11/pulse-pipeline/services/consumer/metrics"
	"github.com/zurek11/pulse-pipeline/services/consumer/models"
)

const maxRetries = 3

// Writer is the interface used to buffer and flush events to storage.
// Defined at point of use for testability.
type Writer interface {
	Add(event interface{}, eventID string)
	Flush(ctx context.Context) (int64, error)
	Len() int
	Reset()
}

// dlqSender is the interface for writing messages to the Dead Letter Queue.
type dlqSender interface {
	WriteMessages(ctx context.Context, msgs ...kafka.Message) error
	Close() error
}

// Consumer reads events from a Kafka topic, batches them, and writes to MongoDB.
// On flush failure it retries up to maxRetries times, then routes the batch to DLQ.
type Consumer struct {
	reader     *kafka.Reader
	dlq        dlqSender
	writer     Writer
	logger     *slog.Logger
	metrics    *metrics.Consumer
	batchSize  int
	flushEvery time.Duration
}

// NewConsumer creates a Consumer wired to the given topic, DLQ, writer, and metrics.
// Pass nil for metrics to disable instrumentation (e.g. in tests).
// batchSize controls how many events to accumulate before flushing.
// flushEvery controls the maximum time between flushes regardless of batch size.
func NewConsumer(
	brokers []string,
	topic, groupID, dlqTopic string,
	writer Writer,
	logger *slog.Logger,
	m *metrics.Consumer,
	batchSize int,
	flushEvery time.Duration,
) *Consumer {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:  brokers,
		Topic:    topic,
		GroupID:  groupID,
		MinBytes: 1e3,
		MaxBytes: 10e6,
	})

	dlqWriter := &kafka.Writer{
		Addr:         kafka.TCP(brokers...),
		Topic:        dlqTopic,
		RequiredAcks: kafka.RequireAll,
		MaxAttempts:  3,
	}

	return &Consumer{
		reader:     reader,
		dlq:        dlqWriter,
		writer:     writer,
		logger:     logger,
		metrics:    m,
		batchSize:  batchSize,
		flushEvery: flushEvery,
	}
}

// batchItem pairs a Kafka message with its deserialized event.
type batchItem struct {
	msg   kafka.Message
	event models.Event
}

// Start runs the consumer loop until ctx is cancelled.
// On shutdown it flushes any remaining buffered events before returning.
func (c *Consumer) Start(ctx context.Context) error {
	ticker := time.NewTicker(c.flushEvery)
	defer ticker.Stop()

	var batch []batchItem

	flush := func() {
		if len(batch) == 0 {
			return
		}

		if c.metrics != nil {
			c.metrics.BufferSize.Set(float64(len(batch)))
		}

		if flushWithRetry(ctx, c.writer, c.logger, c.metrics) {
			// All retries exhausted — route batch to DLQ and clear writer buffer.
			c.logger.Error("flush failed after all retries, routing to DLQ",
				"batch_size", len(batch),
			)
			c.writer.Reset()
			for _, item := range batch {
				if err := c.sendToDLQ(ctx, item.msg); err != nil {
					c.logger.Error("failed to send message to DLQ",
						"partition", item.msg.Partition,
						"offset", item.msg.Offset,
						"error", err,
					)
				} else if c.metrics != nil {
					c.metrics.DLQTotal.Inc()
				}
			}
		}

		// Commit all Kafka offsets regardless of outcome (success or DLQ).
		msgs := make([]kafka.Message, len(batch))
		for i, item := range batch {
			msgs[i] = item.msg
		}
		if err := c.reader.CommitMessages(ctx, msgs...); err != nil {
			c.logger.Error("commit offsets failed", "error", err)
		}

		if c.metrics != nil {
			c.metrics.BufferSize.Set(0)
		}
		batch = batch[:0]
	}

	for {
		select {
		case <-ctx.Done():
			flush()
			return nil

		case <-ticker.C:
			flush()

		default:
			fetchCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
			msg, err := c.reader.FetchMessage(fetchCtx)
			cancel()

			if err != nil {
				if ctx.Err() != nil {
					flush()
					return nil
				}
				// Timeout or transient error — loop again.
				continue
			}

			event, err := parseMessage(msg)
			if err != nil {
				c.logger.Error("failed to parse message — routing to DLQ",
					"partition", msg.Partition,
					"offset", msg.Offset,
					"error", err,
				)
				if dlqErr := c.sendToDLQ(ctx, msg); dlqErr != nil {
					c.logger.Error("failed to send to DLQ", "error", dlqErr)
				} else if c.metrics != nil {
					c.metrics.DLQTotal.Inc()
				}
				if commitErr := c.reader.CommitMessages(ctx, msg); commitErr != nil {
					c.logger.Error("commit offset failed", "error", commitErr)
				}
				continue
			}

			c.writer.Add(event, event.EventID)
			batch = append(batch, batchItem{msg: msg, event: event})

			if c.metrics != nil {
				c.metrics.EventsConsumed.WithLabelValues(event.EventType).Inc()
				c.metrics.BufferSize.Set(float64(len(batch)))
			}

			c.logger.DebugContext(ctx, "event buffered",
				"event_id", event.EventID,
				"event_type", event.EventType,
				"buffer_size", len(batch),
			)

			if len(batch) >= c.batchSize {
				flush()
			}
		}
	}
}

// Close shuts down the Kafka reader and DLQ writer.
func (c *Consumer) Close() error {
	if err := c.reader.Close(); err != nil {
		return fmt.Errorf("close reader: %w", err)
	}
	if err := c.dlq.Close(); err != nil {
		return fmt.Errorf("close dlq writer: %w", err)
	}
	return nil
}

// flushWithRetry attempts to flush the writer up to maxRetries times.
// Returns true if all attempts failed.
func flushWithRetry(ctx context.Context, writer Writer, logger *slog.Logger, m *metrics.Consumer) bool {
	for attempt := 1; attempt <= maxRetries; attempt++ {
		_, err := writer.Flush(ctx)
		if err == nil {
			if m != nil {
				m.WritesTotal.WithLabelValues("success").Inc()
			}
			return false
		}
		logger.Warn("flush failed, retrying",
			"attempt", attempt,
			"max_attempts", maxRetries,
			"error", err,
		)
	}
	if m != nil {
		m.WritesTotal.WithLabelValues("error").Inc()
	}
	return true
}

// parseMessage deserialises a Kafka message into an Event and stamps ProcessedAt.
func parseMessage(msg kafka.Message) (models.Event, error) {
	var event models.Event
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		return models.Event{}, fmt.Errorf("unmarshal event: %w", err)
	}
	event.ProcessedAt = time.Now().UTC()
	return event, nil
}

func (c *Consumer) sendToDLQ(ctx context.Context, msg kafka.Message) error {
	dlqMsg := kafka.Message{
		Key:   msg.Key,
		Value: msg.Value,
	}
	if err := c.dlq.WriteMessages(ctx, dlqMsg); err != nil {
		return fmt.Errorf("write to DLQ: %w", err)
	}
	return nil
}
