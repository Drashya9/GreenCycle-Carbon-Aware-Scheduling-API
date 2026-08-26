package grid

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/Drashya9/GreenCycle-Carbon-Aware-Scheduling-API/internal/models"
)

// Cached wraps a Provider with a Redis-backed cache. Forecasts don't change every
// second, so a cache hit skips both the network call and the rate limiter entirely.
// If Redis is unreachable, it logs once per call and falls back to calling the
// underlying provider directly rather than failing the request.
type Cached struct {
	inner Provider
	redis *redis.Client
	ttl   time.Duration
}

func WithCache(inner Provider, redisClient *redis.Client, ttl time.Duration) *Cached {
	return &Cached{inner: inner, redis: redisClient, ttl: ttl}
}

func (c *Cached) Name() string { return c.inner.Name() }

func (c *Cached) cacheKey(zipCode string) string {
	return fmt.Sprintf("forecast:%s:%s", c.inner.Name(), zipCode)
}

func (c *Cached) GetForecast(ctx context.Context, zipCode string) ([]models.GridDataPoint, error) {
	if c.redis == nil {
		return c.inner.GetForecast(ctx, zipCode)
	}

	key := c.cacheKey(zipCode)

	cached, err := c.redis.Get(ctx, key).Result()
	if err == nil {
		var forecast []models.GridDataPoint
		if jsonErr := json.Unmarshal([]byte(cached), &forecast); jsonErr == nil {
			return forecast, nil
		}
	} else if err != redis.Nil {
		log.Printf("grid cache: redis GET %s failed, bypassing cache: %v", key, err)
	}

	forecast, err := c.inner.GetForecast(ctx, zipCode)
	if err != nil {
		return nil, err
	}

	if encoded, jsonErr := json.Marshal(forecast); jsonErr == nil {
		if setErr := c.redis.Set(ctx, key, encoded, c.ttl).Err(); setErr != nil {
			log.Printf("grid cache: redis SET %s failed: %v", key, setErr)
		}
	}

	return forecast, nil
}
