package httpapi

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/Drashya9/GreenCycle-Carbon-Aware-Scheduling-API/internal/algorithm"
	"github.com/Drashya9/GreenCycle-Carbon-Aware-Scheduling-API/internal/grid"
	"github.com/Drashya9/GreenCycle-Carbon-Aware-Scheduling-API/internal/models"
	"github.com/Drashya9/GreenCycle-Carbon-Aware-Scheduling-API/internal/repository"
)

var (
	// errInternal marks failures that are the server's fault rather than the caller's, so
	// handlers answer 500 without echoing the underlying error.
	errInternal = errors.New("internal error")
	// errUpstream marks grid-provider failures: the request was fine, the forecast source wasn't.
	errUpstream = errors.New("forecast providers unavailable")
)

// findAppliance resolves an appliance by name for the calculate and schedule endpoints.
func (s *Server) findAppliance(ctx context.Context, name string) (*models.Appliance, error) {
	appliance, err := s.applianceRepo.FindByNameIgnoreCase(ctx, name)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, fmt.Errorf("appliance not found: '%s'. Call GET /appliances to see available options", name)
	}
	if err != nil {
		return nil, fmt.Errorf("%w: look up appliance: %v", errInternal, err)
	}
	return appliance, nil
}

// windowFor is the shared pipeline behind GET /calculate and POST /schedules: race every
// configured grid provider for zip's forecast, then pick the cleanest window in it.
func (s *Server) windowFor(ctx context.Context, appliance *models.Appliance, zip string) (*models.OptimalWindow, error) {
	raceCtx, cancel := context.WithTimeout(ctx, s.raceTimeout)
	defer cancel()

	forecast, provider, err := grid.RaceProviders(raceCtx, s.providers, zip)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errUpstream, err)
	}
	return bestWindow(forecast, provider, appliance)
}

// bestWindow runs the sliding-window algorithm over the part of the forecast that hasn't
// started yet, so a recommendation never begins in the past.
func bestWindow(forecast []models.GridDataPoint, provider string, appliance *models.Appliance) (*models.OptimalWindow, error) {
	upcoming := algorithm.UpcomingSlots(forecast, time.Now())
	window, err := algorithm.FindOptimalWindow(upcoming, appliance.DurationHours, appliance.Name)
	if err != nil {
		return nil, err
	}
	window.Provider = provider
	return window, nil
}

// writeWindowError answers a findAppliance or windowFor failure with the status that says
// whose fault it was.
func writeWindowError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, errInternal):
		writeInternalError(w, "calculate window", err)
	case errors.Is(err, errUpstream):
		log.Printf("httpapi: %v", err)
		writeError(w, http.StatusServiceUnavailable, "carbon forecast providers are unavailable; try again shortly")
	default:
		writeError(w, http.StatusBadRequest, err.Error())
	}
}

func (s *Server) handleCalculate(w http.ResponseWriter, r *http.Request) {
	applianceName := strings.TrimSpace(r.URL.Query().Get("appliance"))
	zip := strings.TrimSpace(r.URL.Query().Get("zip"))

	if applianceName == "" {
		writeError(w, http.StatusBadRequest, "appliance name is required")
		return
	}
	if zip == "" {
		writeError(w, http.StatusBadRequest, "zip code is required")
		return
	}
	if err := validateZip(zip); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	appliance, err := s.findAppliance(r.Context(), applianceName)
	if err != nil {
		writeWindowError(w, err)
		return
	}
	window, err := s.windowFor(r.Context(), appliance, zip)
	if err != nil {
		writeWindowError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, window)
}

// handleCalculateMulti fans out across every zip code concurrently (bounded worker pool),
// racing all configured providers for each one, and reports which zip comes out greenest.
func (s *Server) handleCalculateMulti(w http.ResponseWriter, r *http.Request) {
	applianceName := strings.TrimSpace(r.URL.Query().Get("appliance"))
	if applianceName == "" {
		writeError(w, http.StatusBadRequest, "appliance name is required")
		return
	}

	zips, err := parseZipList(r.URL.Query().Get("zips"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	appliance, err := s.findAppliance(r.Context(), applianceName)
	if err != nil {
		writeWindowError(w, err)
		return
	}

	raceCtx, cancel := context.WithTimeout(r.Context(), s.raceTimeout)
	defer cancel()

	fetchResults := grid.FetchMultiZone(raceCtx, func(ctx context.Context, zip string) ([]models.GridDataPoint, string, error) {
		return grid.RaceProviders(ctx, s.providers, zip)
	}, zips, s.multiZoneWorkers)

	results := make([]models.ZoneResult, 0, len(zips))
	var best *models.OptimalWindow

	for _, zip := range zips {
		res := fetchResults[zip]
		if res.Err != nil {
			results = append(results, models.ZoneResult{Zip: zip, Error: res.Err.Error()})
			continue
		}

		window, err := bestWindow(res.Forecast, res.Provider, appliance)
		if err != nil {
			results = append(results, models.ZoneResult{Zip: zip, Error: err.Error()})
			continue
		}

		results = append(results, models.ZoneResult{Zip: zip, Window: window})
		if best == nil || window.AverageCarbonIntensity < best.AverageCarbonIntensity {
			best = window
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"applianceName": appliance.Name,
		"results":       results,
		"greenest":      best,
	})
}
