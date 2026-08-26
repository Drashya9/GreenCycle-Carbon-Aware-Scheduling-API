package grid

import (
	"context"

	"golang.org/x/time/rate"

	"github.com/Drashya9/GreenCycle-Carbon-Aware-Scheduling-API/internal/models"
)

// RateLimited wraps a Provider so that outbound calls are throttled to at most rps
// requests per second (burst 1), preventing us from getting rate-limited by the real
// grid API. It waits (respecting ctx) rather than rejecting, since a delayed forecast
// is preferable to a failed request.
type RateLimited struct {
	inner   Provider
	limiter *rate.Limiter
}

func WithRateLimit(inner Provider, rps float64) *RateLimited {
	return &RateLimited{
		inner:   inner,
		limiter: rate.NewLimiter(rate.Limit(rps), 1),
	}
}

func (r *RateLimited) Name() string { return r.inner.Name() }

func (r *RateLimited) GetForecast(ctx context.Context, zipCode string) ([]models.GridDataPoint, error) {
	if err := r.limiter.Wait(ctx); err != nil {
		return nil, err
	}
	return r.inner.GetForecast(ctx, zipCode)
}
