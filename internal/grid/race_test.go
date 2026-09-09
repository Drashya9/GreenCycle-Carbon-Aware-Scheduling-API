package grid

import (
	"context"
	"testing"
	"time"

	"github.com/Drashya9/GreenCycle-Carbon-Aware-Scheduling-API/internal/models"
)

// delayedProvider simulates a real grid API's network latency, so the test below measures
// genuine wall-clock behavior instead of asserting against an invented number.
type delayedProvider struct {
	name     string
	delay    time.Duration
	forecast []models.GridDataPoint
}

func (d *delayedProvider) Name() string { return d.name }

func (d *delayedProvider) GetForecast(ctx context.Context, zip string) ([]models.GridDataPoint, error) {
	select {
	case <-time.After(d.delay):
		return d.forecast, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func sampleForecast() []models.GridDataPoint {
	now := time.Now().Truncate(time.Hour)
	out := make([]models.GridDataPoint, 24)
	for i := range out {
		out[i] = models.GridDataPoint{Timestamp: now.Add(time.Duration(i) * time.Hour), CarbonIntensity: 200}
	}
	return out
}

// TestRaceProviders_ConcurrentIsFasterThanSequential backs the "goroutine-based concurrency
// to complete lookups" claim: racing N providers concurrently costs roughly the SLOWEST
// one's latency, not the SUM of all of them. Delays (120/140/160ms) approximate real grid
// API response times without making an actual network call.
func TestRaceProviders_ConcurrentIsFasterThanSequential(t *testing.T) {
	providers := []Provider{
		&delayedProvider{name: "p1", delay: 120 * time.Millisecond, forecast: sampleForecast()},
		&delayedProvider{name: "p2", delay: 140 * time.Millisecond, forecast: sampleForecast()},
		&delayedProvider{name: "p3", delay: 160 * time.Millisecond, forecast: sampleForecast()},
	}

	// Sequential baseline: what it would cost to call each provider one after another,
	// i.e. the "before concurrency" case.
	seqStart := time.Now()
	for _, p := range providers {
		if _, err := p.GetForecast(context.Background(), "85281"); err != nil {
			t.Fatalf("sequential call to %s failed: %v", p.Name(), err)
		}
	}
	sequential := time.Since(seqStart)

	// Concurrent: RaceProviders, the actual production code path used by /calculate.
	concStart := time.Now()
	_, winner, err := RaceProviders(context.Background(), providers, "85281")
	concurrent := time.Since(concStart)
	if err != nil {
		t.Fatalf("RaceProviders failed: %v", err)
	}

	t.Logf("sequential (3 providers, one after another): %v", sequential)
	t.Logf("concurrent (RaceProviders, actual code path): %v, winner=%s", concurrent, winner)

	if concurrent >= sequential {
		t.Errorf("expected concurrent race (%v) to be faster than sequential calls (%v)", concurrent, sequential)
	}
	// Should land close to the fastest provider (120ms), not anywhere near the sum (420ms).
	if concurrent > 200*time.Millisecond {
		t.Errorf("expected concurrent race to land close to the fastest provider (~120ms), got %v", concurrent)
	}
}
