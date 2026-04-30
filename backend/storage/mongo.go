package storage

import (
	"context"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"github.com/jainambarbhaya1509/ims/models"
)

type Mongo struct {
	signals *mongo.Collection
}

func NewMongo() *Mongo {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		log.Fatal("Mongo connect failed:", err)
	}

	// Retry ping
	for i := 0; i < 10; i++ {
		if err = client.Ping(ctx, nil); err == nil {
			break
		}
		log.Printf("Mongo not ready, retrying (%d/10)...", i+1)
		time.Sleep(2 * time.Second)
	}

	db := client.Database("ims")
	col := db.Collection("signals")

	// Index on component_id + received_at for fast debounce queries
	col.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{
			{Key: "component_id", Value: 1},
			{Key: "received_at", Value: -1},
		},
	})

	log.Println("Mongo connected")
	return &Mongo{signals: col}
}

func (m *Mongo) InsertSignal(ctx context.Context, s models.Signal) error {
	_, err := m.signals.InsertOne(ctx, s)
	return err
}

// LinkSignalToWorkItem updates raw signals with their WorkItem ID after grouping
func (m *Mongo) LinkSignalToWorkItem(ctx context.Context, signalID, workItemID string) error {
	_, err := m.signals.UpdateOne(ctx,
		bson.M{"_id": signalID},
		bson.M{"$set": bson.M{"work_item_id": workItemID}},
	)
	return err
}

// GetSignalsForWorkItem fetches all raw signals for an incident detail view
func (m *Mongo) GetSignalsForWorkItem(ctx context.Context, workItemID string) ([]models.Signal, error) {
	cursor, err := m.signals.Find(ctx,
		bson.M{"work_item_id": workItemID},
		options.Find().SetSort(bson.D{{Key: "received_at", Value: -1}}),
	)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var signals []models.Signal
	cursor.All(ctx, &signals)
	return signals, nil
}

// GetRecentSignalsForComponent — used by debounce worker to check 10s window
func (m *Mongo) GetRecentSignalsForComponent(ctx context.Context, componentID string, since time.Time) ([]models.Signal, error) {
	cursor, err := m.signals.Find(ctx, bson.M{
		"component_id": componentID,
		"received_at":  bson.M{"$gte": since},
	})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var signals []models.Signal
	cursor.All(ctx, &signals)
	return signals, nil
}