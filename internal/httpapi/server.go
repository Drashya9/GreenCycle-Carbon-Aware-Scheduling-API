// Package httpapi wires the chi router and HTTP handlers.
package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"

	"github.com/Drashya9/GreenCycle-Carbon-Aware-Scheduling-API/internal/grid"
	"github.com/Drashya9/GreenCycle-Carbon-Aware-Scheduling-API/internal/models"
)

// applianceStore and runStore are the slices of the repositories the handlers need,
// kept as interfaces so handler tests can inject in-memory fakes instead of a live DB.
// *repository.ApplianceRepository / *repository.ScheduledRunRepository satisfy these
// structurally, with no changes needed on that side.
type applianceStore interface {
	FindAll(ctx context.Context) ([]models.Appliance, error)
	FindByNameIgnoreCase(ctx context.Context, name string) (*models.Appliance, error)
	Create(ctx context.Context, a *models.Appliance) error
	Delete(ctx context.Context, id int64) error
}

type runStore interface {
	Create(ctx context.Context, run *models.ScheduledRun) error
	FindAll(ctx context.Context, status string) ([]models.ScheduledRun, error)
	FindByID(ctx context.Context, id int64) (*models.ScheduledRun, error)
	Cancel(ctx context.Context, id int64) error
}

// dbPinger and redisPinger narrow *sqlx.DB / *redis.Client to what /health needs.
type dbPinger interface {
	PingContext(ctx context.Context) error
}

type Server struct {
	applianceRepo     applianceStore
	runRepo           runStore
	providers         []grid.Provider
	raceTimeout       time.Duration
	multiZoneWorkers  int
	defaultWebhookURL string

	db          dbPinger
	redisClient *redis.Client
}

func NewServer(
	applianceRepo applianceStore,
	runRepo runStore,
	providers []grid.Provider,
	raceTimeout time.Duration,
	multiZoneWorkers int,
	defaultWebhookURL string,
	db *sqlx.DB,
	redisClient *redis.Client,
) *Server {
	return &Server{
		applianceRepo:     applianceRepo,
		runRepo:           runRepo,
		providers:         providers,
		raceTimeout:       raceTimeout,
		multiZoneWorkers:  multiZoneWorkers,
		defaultWebhookURL: defaultWebhookURL,
		db:                db,
		redisClient:       redisClient,
	}
}

func (s *Server) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(corsMiddleware)

	r.Get("/health", s.handleHealth)

	r.Get("/appliances", s.handleListAppliances)
	r.Post("/appliances", s.handleCreateAppliance)
	r.Delete("/appliances/{id}", s.handleDeleteAppliance)

	r.Get("/calculate", s.handleCalculate)
	r.Get("/calculate/multi", s.handleCalculateMulti)

	r.Post("/schedules", s.handleCreateSchedule)
	r.Get("/schedules", s.handleListSchedules)
	r.Get("/schedules/{id}", s.handleGetSchedule)
	r.Delete("/schedules/{id}", s.handleCancelSchedule)

	return r
}

// corsMiddleware allows calls from any origin during development, matching the Java
// version's @CrossOrigin(origins = "*").
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	checks := map[string]string{"database": "up", "redis": "up"}
	status := "UP"

	if err := s.db.PingContext(ctx); err != nil {
		checks["database"] = "down"
		status = "DEGRADED"
	}
	if s.redisClient == nil {
		checks["redis"] = "not configured"
	} else if err := s.redisClient.Ping(ctx).Err(); err != nil {
		checks["redis"] = "down"
		if status == "UP" {
			status = "DEGRADED"
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":  status,
		"service": "GreenCycle",
		"checks":  checks,
	})
}
