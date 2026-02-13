package redis

import (
	"context"
	"fmt"
	"strconv"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

type RateLimiter struct {
	client        *goredis.Client
	maxRequests   int
	windowSeconds int
}

func NewRateLimiter(client *goredis.Client, maxRequests, windowSeconds int) *RateLimiter {
	return &RateLimiter{
		client:        client,
		maxRequests:   maxRequests,
		windowSeconds: windowSeconds,
	}
}

func (r *RateLimiter) Allow(ctx context.Context, ip string) (bool, error) {
	key := fmt.Sprintf("ratelimit:%s", ip)
	now := time.Now()
	windowStart := now.Add(-time.Duration(r.windowSeconds) * time.Second)

	pipe := r.client.Pipeline()
	pipe.ZRemRangeByScore(ctx, key, "0", strconv.FormatInt(windowStart.UnixMilli(), 10))
	countCmd := pipe.ZCard(ctx, key)
	pipe.ZAdd(ctx, key, goredis.Z{Score: float64(now.UnixMilli()), Member: now.UnixNano()})
	pipe.Expire(ctx, key, time.Duration(r.windowSeconds)*time.Second)

	_, err := pipe.Exec(ctx)
	if err != nil {
		return false, fmt.Errorf("rate limiter pipeline: %w", err)
	}

	count := countCmd.Val()
	return count < int64(r.maxRequests), nil
}
