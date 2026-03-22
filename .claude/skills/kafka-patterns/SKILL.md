---
name: kafka-patterns
description: 'REQUIRED when writing Kafka producer or consumer code. Invoke this skill before any Kafka-related implementation.'
allowed-tools: Read, Write, Edit, Glob, Grep, Bash(go:*), Bash(docker:*)
---

# Kafka Patterns

## When to Use

- Implementing Kafka producer (API service)
- Implementing Kafka consumer (Consumer service)
- Configuring Kafka topics
- Handling offsets, retries, or DLQ

## Producer Pattern

```go
package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/segmentio/kafka-go"
)

type Producer struct {
	writer *kafka.Writer
	logger *slog.Logger
}

func NewProducer(brokers []string, topic string, logger *slog.Logger) *Producer {
	return &Producer{
		writer: &kafka.Writer{
			Addr:         kafka.TCP(brokers...),
			Topic:        topic,
			Balancer:     &kafka.Hash{}, // Hash by key = customer_id partitioning
			RequiredAcks: kafka.RequireAll,
			MaxAttempts:  3,
		},
		logger: logger,
	}
}

func (p *Producer) Produce(ctx context.Context, key string, value interface{}) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	msg := kafka.Message{
		Key:   []byte(key),
		Value: data,
	}

	if err := p.writer.WriteMessages(ctx, msg); err != nil {
		return fmt.Errorf("write to kafka: %w", err)
	}

	return nil
}

func (p *Producer) Close() error {
	return p.writer.Close()
}
```

## Consumer Pattern

```go
package kafka

import (
	"context"
	"log/slog"

	"github.com/segmentio/kafka-go"
)

type Consumer struct {
	reader  *kafka.Reader
	logger  *slog.Logger
	handler MessageHandler
}

type MessageHandler func(ctx context.Context, msg kafka.Message) error

func NewConsumer(brokers []string, topic, groupID string, handler MessageHandler, logger *slog.Logger) *Consumer {
	return &Consumer{
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers:  brokers,
			Topic:    topic,
			GroupID:  groupID,
			MinBytes: 1e3,    // 1KB
			MaxBytes: 10e6,   // 10MB
		}),
		logger:  logger,
		handler: handler,
	}
}

func (c *Consumer) Start(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			c.logger.Info("consumer stopping — context cancelled")
			return nil
		default:
			msg, err := c.reader.FetchMessage(ctx)
			if err != nil {
				if ctx.Err() != nil {
					return nil // graceful shutdown
				}
				c.logger.Error("fetch message failed", "error", err)
				continue
			}

			if err := c.handler(ctx, msg); err != nil {
				c.logger.Error("handle message failed",
					"error", err,
					"topic", msg.Topic,
					"partition", msg.Partition,
					"offset", msg.Offset,
				)
				// Don't commit — will retry on next fetch
				continue
			}

			if err := c.reader.CommitMessages(ctx, msg); err != nil {
				c.logger.Error("commit offset failed", "error", err)
			}
		}
	}
}

func (c *Consumer) Close() error {
	return c.reader.Close()
}
```

## Topic Configuration

```yaml
# In docker-compose.yml, topics are created via kafka-init container
# Topic: pulse.events.v1
#   Partitions: 3
#   Replication: 1
#   Retention: 7 days (604800000 ms)
#   Key: customer_id (hash partitioning)
#
# Topic: pulse.events.dlq.v1
#   Partitions: 1
#   Replication: 1
#   Retention: 30 days
```

## Rules

1. Use `segmentio/kafka-go` — pure Go, no CGo dependency (easier Docker builds than confluent-kafka-go)
2. Producer: key = `customer_id` (guarantees ordering per customer within partition)
3. Producer: `RequireAll` acks (all replicas confirm write)
4. Consumer: explicit offset commit AFTER successful processing
5. Consumer: if processing fails, do NOT commit offset (message will be redelivered)
6. DLQ: after 3 failed attempts, produce to `pulse.events.dlq.v1` and commit original offset
7. Graceful shutdown: close reader/writer, wait for in-flight messages
8. Always log: topic, partition, offset on errors for debugging
9. JSON serialization for values (human-readable, good for learning)
10. Consumer group ID: `pulse-consumer-group` (allows multiple consumer instances)
