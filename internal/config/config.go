// Package config loads all runtime configuration from environment variables (12-factor).
package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Port string

	DatabaseURL string

	RedisURL string

	// GridProviders is the ordered list of enabled grid providers, e.g. "mock,electricitymaps".
	// /calculate races every provider in this list that has its required config present.
	GridProviders []string

	ElectricityMapsAPIKey  string
	ElectricityMapsBaseURL string

	WattTimeUsername string
	WattTimePassword string
	WattTimeBaseURL  string

	// ForecastCacheTTL is how long a forecast stays cached in Redis before it's refetched.
	ForecastCacheTTL time.Duration

	// GridRateLimitRPS caps our own outbound calls to each real grid provider.
	GridRateLimitRPS float64

	// RaceTimeout bounds how long /calculate waits for any provider to respond.
	RaceTimeout time.Duration

	// SchedulerPollInterval is how often the scheduler engine checks for due runs.
	SchedulerPollInterval time.Duration

	// SchedulerWebhookURL is the default webhook target; a scheduled run may override it.
	SchedulerWebhookURL string

	// SchedulerWebhookTimeout bounds a single webhook POST attempt.
	SchedulerWebhookTimeout time.Duration

	// SchedulerWebhookMaxAttempts is how many times a run's webhook is tried (1m, 2m, 4m…
	// apart) before the run is marked failed.
	SchedulerWebhookMaxAttempts int

	// MultiZoneWorkers bounds how many zip codes /calculate/multi fetches concurrently.
	MultiZoneWorkers int

	// RunMigrations applies pending migrations on boot. Turn it off where the schema is
	// migrated separately (cmd/migrate), e.g. serverless hosts that cold-start often.
	RunMigrations bool

	// SchedulerEnabled starts the in-process poll loop. Turn it off on hosts that can't keep
	// a background goroutine alive; runs then fire only via GET /api/cron/tick.
	SchedulerEnabled bool

	// CronSecret enables GET /api/cron/tick when set. Callers must send it as a Bearer token.
	CronSecret string
}

func Load() Config {
	return Config{
		Port: getEnv("PORT", "8080"),

		DatabaseURL: getEnv("DATABASE_URL", "postgres://greencycle:greencycle@localhost:5432/greencycle?sslmode=disable"),

		// KV_URL is the rediss:// URL Vercel's Upstash integration injects.
		RedisURL: getEnv("REDIS_URL", getEnv("KV_URL", "redis://localhost:6379/0")),

		GridProviders: splitCSV(getEnv("GRID_PROVIDERS", "mock")),

		ElectricityMapsAPIKey:  os.Getenv("ELECTRICITYMAPS_API_KEY"),
		ElectricityMapsBaseURL: getEnv("ELECTRICITYMAPS_BASE_URL", "https://api.electricitymap.org/v3"),

		WattTimeUsername: os.Getenv("WATTTIME_USERNAME"),
		WattTimePassword: os.Getenv("WATTTIME_PASSWORD"),
		WattTimeBaseURL:  getEnv("WATTTIME_BASE_URL", "https://api2.watttime.org/v2"),

		ForecastCacheTTL: getDuration("FORECAST_CACHE_TTL", 15*time.Minute),

		GridRateLimitRPS: getFloat("GRID_RATE_LIMIT_RPS", 2.0),

		RaceTimeout: getDuration("GRID_RACE_TIMEOUT", 8*time.Second),

		SchedulerPollInterval: getDuration("SCHEDULER_POLL_INTERVAL", 15*time.Second),

		SchedulerWebhookURL: os.Getenv("SCHEDULER_WEBHOOK_URL"),

		SchedulerWebhookTimeout: getDuration("SCHEDULER_WEBHOOK_TIMEOUT", 5*time.Second),

		SchedulerWebhookMaxAttempts: getInt("SCHEDULER_WEBHOOK_MAX_ATTEMPTS", 3),

		MultiZoneWorkers: getInt("MULTIZONE_WORKERS", 5),

		RunMigrations:    getBool("RUN_MIGRATIONS", true),
		SchedulerEnabled: getBool("SCHEDULER_ENABLED", true),
		CronSecret:       os.Getenv("CRON_SECRET"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}

func getFloat(key string, fallback float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return fallback
}

func getInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}

func getBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return fallback
}

func splitCSV(v string) []string {
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
