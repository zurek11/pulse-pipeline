package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/segmentio/kafka-go"

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

// Consumer reads events from a Kafka topic, batches them, and writes to MongoDB.
// On flush failure it retries up to maxRetries times, then routes the batch to DLQ.
type Consumer struct {
	reader     *kafka.Reader
	dlq        *kafka.Writer
	writer     Writer
	logger     *slog.Logger
	batchSize  int
	flushEvery time.Duration
}

// NewConsumer creates a Consumer wired to the given topic, DLQ, and writer.
func NewConsumer(
	brokers []string,
	topic, groupID, dlqTopic string,
	writer Writer,
	logger *slog.Logger,
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
		batchSize:  100,
		flushEvery: time.Second,
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

		var lastErr error
		for attempt := 1; attempt <= maxRetries; attempt++ {
			_, err := c.writer.Flush(ctx)
			if err == nil {
				lastErr = nil
				break
			}
			lastErr = err
			c.logger.Warn("flush failed, retrying",
				"attempt", attempt,
				"max_attempts", maxRetries,
				"error", err,
			)
		}

		if lastErr != nil {
			// All retries exhausted — route batch to DLQ and clear writer buffer.
			c.logger.Error("flush failed after all retries, routing to DLQ",
				"batch_size", len(batch),
				"error", lastErr,
			)
			c.writer.Reset()
			for _, item := range batch {
				if err := c.sendToDLQ(ctx, item.msg); err != nil {
					c.logger.Error("failed to send message to DLQ",
						"partition", item.msg.Partition,
						"offset", item.msg.Offset,
						"error", err,
					)
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
				}
				if commitErr := c.reader.CommitMessages(ctx, msg); commitErr != nil {
					c.logger.Error("commit offset failed", "error", commitErr)
				}
				continue
			}

			c.writer.Add(event, event.EventID)
			batch = append(batch, batchItem{msg: msg, event: event})

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
