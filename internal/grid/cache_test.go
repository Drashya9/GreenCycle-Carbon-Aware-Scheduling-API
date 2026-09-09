package grid

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/Drashya9/GreenCycle-Carbon-Aware-Scheduling-API/internal/cache"
)

// TestCached_ConcurrentRepeatedRequests backs the "reducing latency for repeated requests"
// claim: once a forecast is cached, N concurrent callers for the same (provider, zip) all
// return in roughly the time of a single Redis round-trip, not N times the underlying
// provider's real latency.
//
// Needs a reachable Redis (REDIS_URL, default redis://localhost:6379/0) — skipped
// automatically if one isn't available, so this never blocks CI or a Redis-less machine.
func TestCached_ConcurrentRepeatedRequests(t *testing.T) {
	redisURL := getEnvOrDefault("REDIS_URL", "redis://localhost:6379/0")
	redisClient, err := cache.NewClient(redisURL)
	if err != nil {
		t.Skipf("redis not reachable at %s, skipping: %v", redisURL, err)
	}
	defer redisClient.Close()

	zip := fmt.Sprintf("test-%d", time.Now().UnixNano()) // unique key so repeated runs don't collide
	inner := &delayedProvider{name: "slow-real-api", delay: 200 * time.Millisecond, forecast: sampleForecast()}
	cached := WithCache(inner, redisClient, time.Minute)
	defer redisClient.Del(context.Background(), cached.cacheKey(zip))

	// First call: guaranteed cache miss, pays the full simulated API latency.
	missStart := time.Now()
	if _, err := cached.GetForecast(context.Background(), zip); err != nil {
		t.Fatalf("cache-miss call failed: %v", err)
	}
	miss := time.Since(missStart)

	// 10 concurrent callers for the same key — every one should now be a cache hit.
	const concurrency = 10
	var wg sync.WaitGroup
	hitStart := time.Now()
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := cached.GetForecast(context.Background(), zip); err != nil {
				t.Errorf("cache-hit call failed: %v", err)
			}
		}()
	}
	wg.Wait()
	hits := time.Since(hitStart)

	t.Logf("cache miss (1 call, hits the real provider): %v", miss)
	t.Logf("cache hits (%d concurrent calls, served from Redis): %v total", concurrency, hits)

	if hits >= miss {
		t.Errorf("expected %d concurrent cache hits (%v) to be faster than a single cache miss (%v)", concurrency, hits, miss)
	}
}

func getEnvOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
