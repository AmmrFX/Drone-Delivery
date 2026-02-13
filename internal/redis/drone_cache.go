package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"drone-delivery/internal/common"
)

type CachedDroneLocation struct {
	Lat       float64   `json:"lat"`
	Lng       float64   `json:"lng"`
	Timestamp time.Time `json:"timestamp"`
}

type DroneLocationCache struct {
	client *goredis.Client
	ttl    time.Duration
}

func NewDroneLocationCache(client *goredis.Client, ttlSeconds int) *DroneLocationCache {
	return &DroneLocationCache{
		client: client,
		ttl:    time.Duration(ttlSeconds) * time.Second,
	}
}

func (c *DroneLocationCache) Set(ctx context.Context, droneID string, loc common.Location) error {
	data := CachedDroneLocation{
		Lat:       loc.Lat,
		Lng:       loc.Lng,
		Timestamp: time.Now(),
	}
	bytes, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal drone location: %w", err)
	}
	return c.client.Set(ctx, droneLocationKey(droneID), bytes, c.ttl).Err()
}

func (c *DroneLocationCache) Get(ctx context.Context, droneID string) (*CachedDroneLocation, error) {
	bytes, err := c.client.Get(ctx, droneLocationKey(droneID)).Bytes()
	if err == goredis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get drone location: %w", err)
	}

	var loc CachedDroneLocation
	if err := json.Unmarshal(bytes, &loc); err != nil {
		return nil, fmt.Errorf("unmarshal drone location: %w", err)
	}
	return &loc, nil
}

func droneLocationKey(droneID string) string {
	return fmt.Sprintf("drone:location:%s", droneID)
}
