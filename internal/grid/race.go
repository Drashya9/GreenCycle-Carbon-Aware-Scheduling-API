package grid

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/Drashya9/GreenCycle-Carbon-Aware-Scheduling-API/internal/models"
)

// raceResult carries one provider's outcome back to the racer.
type raceResult struct {
	provider string
	forecast []models.GridDataPoint
	err      error
}

// FetchResult is one zip code's forecast outcome from a concurrent multi-zone fetch.
type FetchResult struct {
	Provider string
	Forecast []models.GridDataPoint
	Err      error
}

// RaceProviders calls every provider concurrently and returns whichever succeeds first.
// Once a winner is picked, ctx is cancelled so slower in-flight HTTP calls (made with
// http.NewRequestWithContext) are abandoned rather than left running to completion.
// If every provider fails, all errors are joined together.
func RaceProviders(ctx context.Context, providers []Provider, zipCode string) ([]models.GridDataPoint, string, error) {
	if len(providers) == 0 {
		return nil, "", errors.New("no grid providers configured")
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	results := make(chan raceResult, len(providers))

	for _, p := range providers {
		go func(p Provider) {
			forecast, err := p.GetForecast(ctx, zipCode)
			results <- raceResult{provider: p.Name(), forecast: forecast, err: err}
		}(p)
	}

	var errs []error
	for i := 0; i < len(providers); i++ {
		select {
		case res := <-results:
			if res.err == nil {
				return res.forecast, res.provider, nil
			}
			errs = append(errs, fmt.Errorf("%s: %w", res.provider, res.err))
		case <-ctx.Done():
			errs = append(errs, ctx.Err())
			return nil, "", errors.Join(append([]error{errors.New("grid provider race cancelled")}, errs...)...)
		}
	}

	return nil, "", errors.Join(append([]error{errors.New("all grid providers failed")}, errs...)...)
}

// FetchFunc fetches one zip code's forecast, returning which provider answered.
// RaceProviders (bound to a provider list) is the fetch function /calculate/multi uses,
// so each zip in the worker pool below also races every configured provider.
type FetchFunc func(ctx context.Context, zipCode string) ([]models.GridDataPoint, string, error)

// FetchMultiZone fetches a forecast for every zip code concurrently, using a bounded
// worker pool so a large zip list can't spawn unbounded goroutines or outbound requests.
func FetchMultiZone(ctx context.Context, fetch FetchFunc, zipCodes []string, workers int) map[string]FetchResult {
	if workers < 1 {
		workers = 1
	}

	jobs := make(chan string)
	out := make(map[string]FetchResult, len(zipCodes))
	var mu sync.Mutex
	var wg sync.WaitGroup

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for zip := range jobs {
				forecast, provider, err := fetch(ctx, zip)
				mu.Lock()
				out[zip] = FetchResult{Provider: provider, Forecast: forecast, Err: err}
				mu.Unlock()
			}
		}()
	}

	for _, zip := range zipCodes {
		jobs <- zip
	}
	close(jobs)

	wg.Wait()
	return out
}
