package mongodb

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// Connect establishes a MongoDB connection, verifies it with a ping,
// ensures all required indexes exist, and returns the client and collection.
func Connect(ctx context.Context, uri, database, collection string) (*mongo.Client, *mongo.Collection, error) {
	// v2: Connect no longer takes a context as the first argument.
	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		return nil, nil, fmt.Errorf("connect to mongodb: %w", err)
	}

	if err := client.Ping(ctx, nil); err != nil {
		return nil, nil, fmt.Errorf("ping mongodb: %w", err)
	}

	coll := client.Database(database).Collection(collection)

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
		return nil, nil, fmt.Errorf("create indexes: %w", err)
	}

	return client, coll, nil
}
