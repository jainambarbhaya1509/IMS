package storage

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// SignalBucket represents signal count for a time window
type SignalBucket struct {
	Hour       time.Time `json:"hour" bson:"hour"`
	Count      int       `json:"count" bson:"count"`
	ComponentID string   `json:"component_id" bson:"component_id"`
}

// GetSignalsPerHour returns signal volume grouped by hour and component.
// This is a MongoDB aggregation pipeline — think of it as SQL GROUP BY
// but for documents.
//
// Pipeline stages:
// 1. $match  — filter to last 24 hours only
// 2. $group  — group by component + hour, count signals
// 3. $sort   — newest first
func (m *Mongo) GetSignalsPerHour(ctx context.Context, since time.Time) ([]SignalBucket, error) {
	pipeline := mongo.Pipeline{
		// Stage 1: Only look at signals from the last N hours
		// $gte means "greater than or equal to"
		{{Key: "$match", Value: bson.M{
			"received_at": bson.M{"$gte": since},
		}}},

		// Stage 2: Group documents together
		// _id defines the grouping key — component + truncated hour
		// $sum: 1 means "add 1 for each document in this group" = count
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: bson.D{
				{Key: "component_id", Value: "$component_id"},
				{Key: "hour", Value: bson.D{
					// $dateTrunc rounds the timestamp down to the nearest hour
					// e.g. 10:47:23 becomes 10:00:00
					{Key: "$dateTrunc", Value: bson.D{
						{Key: "date", Value: "$received_at"},
						{Key: "unit", Value: "hour"},
					}},
				}},
			}},
			{Key: "count", Value: bson.D{{Key: "$sum", Value: 1}}},
		}}},

		// Stage 3: Sort by hour descending (newest first)
		{{Key: "$sort", Value: bson.D{
			{Key: "_id.hour", Value: -1},
		}}},
	}

	cursor, err := m.signals.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	// Decode results into our struct
	var results []struct {
		ID struct {
			ComponentID string    `bson:"component_id"`
			Hour        time.Time `bson:"hour"`
		} `bson:"_id"`
		Count int `bson:"count"`
	}

	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}

	// Reshape into clean output structs
	buckets := make([]SignalBucket, len(results))
	for i, r := range results {
		buckets[i] = SignalBucket{
			Hour:        r.ID.Hour,
			Count:       r.Count,
			ComponentID: r.ID.ComponentID,
		}
	}
	return buckets, nil
}