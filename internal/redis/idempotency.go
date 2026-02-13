package redis

import (
	"context"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

type IdempotencyStore struct {
	client *goredis.Client
	ttl    time.Duration
}

func NewIdempotencyStore(client *goredis.Client, ttlSeconds int) *IdempotencyStore {
	return &IdempotencyStore{
		client: client,
		ttl:    time.Duration(ttlSeconds) * time.Second,
	}
}

func (s *IdempotencyStore) Check(ctx context.Context, userID, key string) ([]byte, bool, error) {
	k := idempotencyKey(userID, key)
	bytes, err := s.client.Get(ctx, k).Bytes()
	if err == goredis.Nil {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("check idempotency key: %w", err)
	}
	return bytes, true, nil
}

func (s *IdempotencyStore) Set(ctx context.Context, userID, key string, response []byte) error {
	k := idempotencyKey(userID, key)
	_, err := s.client.SetNX(ctx, k, response, s.ttl).Result()
	if err != nil {
		return fmt.Errorf("set idempotency key: %w", err)
	}
	return nil
}

func idempotencyKey(userID, key string) string {
	return fmt.Sprintf("idempotency:%s:%s", userID, key)
}
