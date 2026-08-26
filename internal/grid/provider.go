// Package grid fetches carbon-intensity forecasts, and provides concurrency, caching,
// and rate-limiting decorators around that fetch.
package grid

import (
	"context"

	"github.com/Drashya9/GreenCycle-Carbon-Aware-Scheduling-API/internal/models"
)

// Provider fetches a 24-hour carbon-intensity forecast for a US zip code.
type Provider interface {
	// Name identifies the provider, e.g. "mock", "electricitymaps", "watttime".
	// Returned to the caller so they know which source answered a race.
	Name() string
	GetForecast(ctx context.Context, zipCode string) ([]models.GridDataPoint, error)
}
