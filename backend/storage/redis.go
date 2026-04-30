package storage

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/jainambarbhaya1509/ims/models"
)

type Redis struct {
	client *redis.Client
}

func NewRedis() *Redis {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	// Retry
	for i := 0; i < 10; i++ {
		if err := client.Ping(context.Background()).Err(); err == nil {
			break
		}
		log.Printf("Redis not ready, retrying (%d/10)...", i+1)
		time.Sleep(2 * time.Second)
	}

	log.Println("Redis connected")
	return &Redis{client: client}
}

// SetWorkItem caches a single work item (called every time a work item changes)
func (r *Redis) SetWorkItem(ctx context.Context, wi models.WorkItem) error {
	data, err := json.Marshal(wi)
	if err != nil {
		return err
	}
	// TTL of 5 minutes — if Redis dies, UI falls back to Postgres
	return r.client.Set(ctx, "wi:"+wi.ID, data, 5*time.Minute).Err()
}

// GetAllWorkItems returns the cached dashboard state
func (r *Redis) GetAllWorkItems(ctx context.Context) ([]models.WorkItem, error) {
	keys, err := r.client.Keys(ctx, "wi:*").Result()
	if err != nil || len(keys) == 0 {
		return nil, err
	}

	vals, err := r.client.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, err
	}

	var items []models.WorkItem
	for _, v := range vals {
		if v == nil {
			continue
		}
		var wi models.WorkItem
		if err := json.Unmarshal([]byte(v.(string)), &wi); err == nil {
			items = append(items, wi)
		}
	}
	return items, nil
}

// InvalidateWorkItem removes a specific work item from cache (forces re-fetch)
func (r *Redis) InvalidateWorkItem(ctx context.Context, workItemID string) error {
	return r.client.Del(ctx, "wi:"+workItemID).Err()
}