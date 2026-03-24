package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/segmentio/kafka-go"

	"github.com/zurek11/pulse-pipeline/services/api/models"
)

// Producer wraps kafka-go Writer with structured logging.
type Producer struct {
	writer *kafka.Writer
	logger *slog.Logger
}

// NewProducer creates a new Kafka producer targeting the given brokers and topic.
// Uses hash partitioning so the same key (customer_id) always goes to the same partition.
func NewProducer(brokers []string, topic string, logger *slog.Logger) *Producer {
	return &Producer{
		writer: &kafka.Writer{
			Addr:         kafka.TCP(brokers...),
			Topic:        topic,
			Balancer:     &kafka.Hash{},
			RequiredAcks: kafka.RequireAll,
			MaxAttempts:  3,
		},
		logger: logger,
	}
}

// Produce serialises value as JSON and writes it to Kafka with the given key.
func (p *Producer) Produce(ctx context.Context, key string, value interface{}) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal value: %w", err)
	}

	msg := kafka.Message{
		Key:   []byte(key),
		Value: data,
	}

	if err := p.writer.WriteMessages(ctx, msg); err != nil {
		return fmt.Errorf("write to kafka: %w", err)
	}

	p.logger.DebugContext(ctx, "event produced", "key", key, "topic", p.writer.Topic)
	return nil
}

// ProduceBatch serialises all events and writes them to Kafka in a single
// WriteMessages call — one round-trip regardless of batch size.
func (p *Producer) ProduceBatch(ctx context.Context, events []*models.Event) error {
	msgs := make([]kafka.Message, len(events))
	for i, e := range events {
		data, err := json.Marshal(e)
		if err != nil {
			return fmt.Errorf("marshal event %d: %w", i, err)
		}
		msgs[i] = kafka.Message{
			Key:   []byte(e.CustomerID),
			Value: data,
		}
	}
	if err := p.writer.WriteMessages(ctx, msgs...); err != nil {
		return fmt.Errorf("write batch to kafka: %w", err)
	}
	p.logger.DebugContext(ctx, "batch produced", "count", len(events), "topic", p.writer.Topic)
	return nil
}

// Close flushes in-flight messages and closes the underlying writer.
func (p *Producer) Close() error {
	if err := p.writer.Close(); err != nil {
		return fmt.Errorf("close kafka writer: %w", err)
	}
	return nil
}
