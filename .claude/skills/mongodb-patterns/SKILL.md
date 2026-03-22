---
name: mongodb-patterns
description: 'REQUIRED when writing MongoDB read/write operations. Invoke this skill before any MongoDB-related code.'
allowed-tools: Read, Write, Edit, Glob, Grep, Bash(go:*), Bash(docker:*)
---

# MongoDB Patterns

## When to Use

- Implementing MongoDB client connection
- Writing bulk insert/upsert operations
- Creating indexes
- Querying events

## Bulk Writer Pattern (Idempotent)

```go
package mongodb

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type BulkWriter struct {
	collection    *mongo.Collection
	logger        *slog.Logger
	buffer        []mongo.WriteModel
	mu            sync.Mutex
	batchSize     int
	flushInterval time.Duration
}

func NewBulkWriter(collection *mongo.Collection, batchSize int, flushInterval time.Duration, logger *slog.Logger) *BulkWriter {
	return &BulkWriter{
		collection:    collection,
		logger:        logger,
		buffer:        make([]mongo.WriteModel, 0, batchSize),
		batchSize:     batchSize,
		flushInterval: flushInterval,
	}
}

// Add adds an event to the buffer. Flushes automatically when batch size is reached.
func (w *BulkWriter) Add(ctx context.Context, event interface{}, eventID string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Idempotent upsert — if event_id exists, skip (setOnInsert)
	model := mongo.NewUpdateOneModel().
		SetFilter(bson.M{"event_id": eventID}).
		SetUpdate(bson.M{"$setOnInsert": event}).
		SetUpsert(true)

	w.buffer = append(w.buffer, model)

	if len(w.buffer) >= w.batchSize {
		return w.flushLocked(ctx)
	}
	return nil
}

// Flush writes all buffered events to MongoDB.
func (w *BulkWriter) Flush(ctx context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.flushLocked(ctx)
}

func (w *BulkWriter) flushLocked(ctx context.Context) error {
	if len(w.buffer) == 0 {
		return nil
	}

	opts := options.BulkWrite().SetOrdered(false) // Unordered = faster, continues on error
	result, err := w.collection.BulkWrite(ctx, w.buffer, opts)
	if err != nil {
		return fmt.Errorf("bulk write: %w", err)
	}

	w.logger.Info("bulk write completed",
		"inserted", result.UpsertedCount,
		"matched", result.MatchedCount, // matched = duplicates (already existed)
		"buffer_size", len(w.buffer),
	)

	w.buffer = w.buffer[:0] // Reset buffer
	return nil
}
```

## Connection + Index Setup

```go
func Connect(ctx context.Context, uri, database, collection string) (*mongo.Collection, error) {
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return nil, fmt.Errorf("connect to mongodb: %w", err)
	}

	if err := client.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("ping mongodb: %w", err)
	}

	coll := client.Database(database).Collection(collection)

	// Ensure indexes exist
	indexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "event_id", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{
				{Key: "customer_id", Value: 1},
				{Key: "timestamp", Value: -1},
			},
		},
		{
			Keys: bson.D{
				{Key: "event_type", Value: 1},
				{Key: "timestamp", Value: -1},
			},
		},
	}

	if _, err := coll.Indexes().CreateMany(ctx, indexes); err != nil {
		return nil, fmt.Errorf("create indexes: %w", err)
	}

	return coll, nil
}
```

## Rules

1. Use official Go driver: `go.mongodb.org/mongo-driver`
2. Always use `context.Context` for operations (timeout, cancellation)
3. Bulk writes with `ordered: false` — faster, doesn't stop on single failure
4. Idempotent upserts with `$setOnInsert` — safe for at-least-once Kafka delivery
5. Create indexes on startup (idempotent — CreateMany is safe to call repeatedly)
6. Unique index on `event_id` for deduplication guarantee
7. Compound indexes for query patterns: `(customer_id, timestamp)`, `(event_type, timestamp)`
8. Connection URI from environment variable: `MONGODB_URI`
9. Always `Disconnect()` on shutdown
10. Log bulk write results: inserted count, matched count (duplicates)
