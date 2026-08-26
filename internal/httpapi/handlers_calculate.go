package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/Drashya9/GreenCycle-Carbon-Aware-Scheduling-API/internal/algorithm"
	"github.com/Drashya9/GreenCycle-Carbon-Aware-Scheduling-API/internal/grid"
	"github.com/Drashya9/GreenCycle-Carbon-Aware-Scheduling-API/internal/models"
	"github.com/Drashya9/GreenCycle-Carbon-Aware-Scheduling-API/internal/repository"
)

// calculateWindow is the shared pipeline behind both GET /calculate and POST /schedules:
// look up the appliance, race every configured grid provider for a forecast, then run the
// sliding-window algorithm over it.
func (s *Server) calculateWindow(ctx context.Context, applianceName, zip string) (*models.OptimalWindow, error) {
	appliance, err := s.applianceRepo.FindByNameIgnoreCase(ctx, applianceName)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, fmt.Errorf("appliance not found: '%s'. Call GET /appliances to see available options", applianceName)
		}
		return nil, err
	}

	raceCtx, cancel := context.WithTimeout(ctx, s.raceTimeout)
	defer cancel()

	forecast, provider, err := grid.RaceProviders(raceCtx, s.providers, zip)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch carbon forecast: %w", err)
	}

	window, err := algorithm.FindOptimalWindow(forecast, appliance.DurationHours, appliance.Name)
	if err != nil {
		return nil, err
	}
	window.Provider = provider

	return window, nil
}

func (s *Server) handleCalculate(w http.ResponseWriter, r *http.Request) {
	appliance := strings.TrimSpace(r.URL.Query().Get("appliance"))
	zip := strings.TrimSpace(r.URL.Query().Get("zip"))

	if appliance == "" {
		writeError(w, http.StatusBadRequest, "appliance name is required")
		return
	}
	if zip == "" {
		writeError(w, http.StatusBadRequest, "zip code is required")
		return
	}

	window, err := s.calculateWindow(r.Context(), appliance, zip)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, window)
}

// handleCalculateMulti fans out across every zip code concurrently (bounded worker pool),
// racing all configured providers for each one, and reports which zip comes out greenest.
func (s *Server) handleCalculateMulti(w http.ResponseWriter, r *http.Request) {
	applianceName := strings.TrimSpace(r.URL.Query().Get("appliance"))
	zipsParam := strings.TrimSpace(r.URL.Query().Get("zips"))

	if applianceName == "" {
		writeError(w, http.StatusBadRequest, "appliance name is required")
		return
	}
	if zipsParam == "" {
		writeError(w, http.StatusBadRequest, "zips is required (comma-separated)")
		return
	}

	var zips []string
	for _, z := range strings.Split(zipsParam, ",") {
		if z = strings.TrimSpace(z); z != "" {
			zips = append(zips, z)
		}
	}
	if len(zips) == 0 {
		writeError(w, http.StatusBadRequest, "zips is required (comma-separated)")
		return
	}

	appliance, err := s.applianceRepo.FindByNameIgnoreCase(r.Context(), applianceName)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusBadRequest, fmt.Sprintf(
				"appliance not found: '%s'. Call GET /appliances to see available options", applianceName))
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
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

		window, err := algorithm.FindOptimalWindow(res.Forecast, appliance.DurationHours, appliance.Name)
		if err != nil {
			results = append(results, models.ZoneResult{Zip: zip, Error: err.Error()})
			continue
		}
		window.Provider = res.Provider

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
