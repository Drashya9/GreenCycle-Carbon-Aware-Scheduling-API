// Package cache configures the shared Redis client used for forecast caching.
package cache

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// NewClient parses redisURL and returns a connected client, verified with a short PING.
func NewClient(redisURL string) (*redis.Client, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, err
	}
	client := redis.NewClient(opts)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}
	return client, nil
}
