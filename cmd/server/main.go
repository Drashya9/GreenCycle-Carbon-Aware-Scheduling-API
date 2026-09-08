// Command server is the GreenCycle entry point: it wires config, Postgres, Redis, grid
// providers, the HTTP API, and the background scheduler engine together, then runs until
// SIGINT/SIGTERM.
package main

import (
	"context"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"

	"github.com/Drashya9/GreenCycle-Carbon-Aware-Scheduling-API/internal/cache"
	"github.com/Drashya9/GreenCycle-Carbon-Aware-Scheduling-API/internal/config"
	"github.com/Drashya9/GreenCycle-Carbon-Aware-Scheduling-API/internal/db"
	"github.com/Drashya9/GreenCycle-Carbon-Aware-Scheduling-API/internal/grid"
	"github.com/Drashya9/GreenCycle-Carbon-Aware-Scheduling-API/internal/httpapi"
	"github.com/Drashya9/GreenCycle-Carbon-Aware-Scheduling-API/internal/repository"
	"github.com/Drashya9/GreenCycle-Carbon-Aware-Scheduling-API/internal/scheduler"
	"github.com/Drashya9/GreenCycle-Carbon-Aware-Scheduling-API/web"
)

func main() {
	// .env is optional local-dev convenience; real deployments set real env vars.
	_ = godotenv.Load()

	cfg := config.Load()

	dbx, err := db.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("connect to postgres: %v", err)
	}
	defer dbx.Close()

	if err := db.Migrate(dbx); err != nil {
		log.Fatalf("run migrations: %v", err)
	}
	log.Println("migrations up to date")

	redisClient, err := cache.NewClient(cfg.RedisURL)
	if err != nil {
		log.Printf("redis unavailable, forecasts will not be cached: %v", err)
		redisClient = nil
	}

	providers := buildProviders(cfg, redisClient)
	log.Printf("grid providers enabled: %d", len(providers))

	applianceRepo := repository.NewApplianceRepository(dbx)
	runRepo := repository.NewScheduledRunRepository(dbx)

	server := httpapi.NewServer(
		applianceRepo,
		runRepo,
		providers,
		cfg.RaceTimeout,
		cfg.MultiZoneWorkers,
		cfg.SchedulerWebhookURL,
		dbx,
		redisClient,
		web.FS,
	)

	httpServer := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: server.Router(),
	}

	engine := scheduler.NewEngine(runRepo, cfg.SchedulerWebhookURL, cfg.SchedulerPollInterval, cfg.SchedulerWebhookTimeout)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Println("scheduler engine started")
		engine.Run(ctx)
		log.Println("scheduler engine stopped")
	}()

	go func() {
		log.Printf("listening on :%s", cfg.Port)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http server: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("http server shutdown: %v", err)
	}
}

// buildProviders constructs every grid provider named in GRID_PROVIDERS that has its
// required config present, each wrapped Cached(RateLimited(base)) so cache hits skip
// the limiter entirely and only real outbound calls are throttled.
func buildProviders(cfg config.Config, redisClient *redis.Client) []grid.Provider {
	var providers []grid.Provider

	wrap := func(base grid.Provider) grid.Provider {
		limited := grid.WithRateLimit(base, cfg.GridRateLimitRPS)
		return grid.WithCache(limited, redisClient, cfg.ForecastCacheTTL)
	}

	for _, name := range cfg.GridProviders {
		switch name {
		case "mock":
			providers = append(providers, wrap(grid.NewMockProvider()))
		case "electricitymaps":
			if cfg.ElectricityMapsAPIKey == "" {
				log.Println("skipping electricitymaps: ELECTRICITYMAPS_API_KEY not set")
				continue
			}
			providers = append(providers, wrap(grid.NewElectricityMapsProvider(cfg.ElectricityMapsAPIKey, cfg.ElectricityMapsBaseURL)))
		case "watttime":
			if cfg.WattTimeUsername == "" || cfg.WattTimePassword == "" {
				log.Println("skipping watttime: WATTTIME_USERNAME/WATTTIME_PASSWORD not set")
				continue
			}
			providers = append(providers, wrap(grid.NewWattTimeProvider(cfg.WattTimeUsername, cfg.WattTimePassword, cfg.WattTimeBaseURL)))
		default:
			log.Printf("unknown grid provider %q, ignoring", name)
		}
	}

	if len(providers) == 0 {
		log.Println("no grid providers configured, falling back to mock")
		providers = append(providers, wrap(grid.NewMockProvider()))
	}

	return providers
}
