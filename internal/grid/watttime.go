package grid

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/Drashya9/GreenCycle-Carbon-Aware-Scheduling-API/internal/models"
)

// zipToBA is a minimal ZIP-prefix -> WattTime balancing-authority lookup table.
var zipToBA = map[string]string{
	"900": "CAISO_NORTH", "901": "CAISO_NORTH", "902": "CAISO_NORTH",
	"940": "CAISO_NORTH", "941": "CAISO_NORTH", "945": "CAISO_NORTH",
	"852": "PNM", "853": "PNM",
	"750": "ERCOT_NORTH", "770": "ERCOT_NORTH",
	"100": "NYIS_NYC", "110": "NYIS_NYC",
	"331": "FPL", "332": "FPL",
	"606": "PJM_CHICAGO", "607": "PJM_CHICAGO",
}

// WattTimeProvider talks to https://api2.watttime.org/v2 — login once, cache the token
// for its lifetime, then fetch a marginal-emissions forecast per balancing authority.
// Deactivated (never raced) unless both username and password are configured.
type WattTimeProvider struct {
	username   string
	password   string
	baseURL    string
	httpClient *http.Client

	mu       sync.Mutex
	token    string
	tokenExp time.Time
}

func NewWattTimeProvider(username, password, baseURL string) *WattTimeProvider {
	return &WattTimeProvider{
		username:   username,
		password:   password,
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (w *WattTimeProvider) Name() string { return "watttime" }

type wtLoginResponse struct {
	Token string `json:"token"`
}

// authToken returns a cached token if it's still valid, otherwise logs in for a new one.
func (w *WattTimeProvider) authToken(ctx context.Context) (string, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.token != "" && time.Now().Before(w.tokenExp) {
		return w.token, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, w.baseURL+"/login", nil)
	if err != nil {
		return "", err
	}
	req.SetBasicAuth(w.username, w.password)

	resp, err := w.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("watttime: login request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("watttime: login failed with status %d", resp.StatusCode)
	}

	var body wtLoginResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", fmt.Errorf("watttime: decode login response: %w", err)
	}

	w.token = body.Token
	// WattTime tokens are valid for ~30 minutes; refresh a little early.
	w.tokenExp = time.Now().Add(25 * time.Minute)
	return w.token, nil
}

type wtForecastPoint struct {
	PointTime string  `json:"point_time"`
	Value     float64 `json:"value"` // MOER, in lbs CO2/MWh
}

const lbsPerMWhToGramsPerKWh = 0.453592

func (w *WattTimeProvider) GetForecast(ctx context.Context, zipCode string) ([]models.GridDataPoint, error) {
	token, err := w.authToken(ctx)
	if err != nil {
		return nil, err
	}

	ba := zipToBAFor(zipCode)
	url := fmt.Sprintf("%s/forecast?ba=%s", w.baseURL, ba)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := w.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("watttime: unexpected status %d", resp.StatusCode)
	}

	var points []wtForecastPoint
	if err := json.NewDecoder(resp.Body).Decode(&points); err != nil {
		return nil, fmt.Errorf("watttime: decode response: %w", err)
	}

	return hourlyAverage(points)
}

// hourlyAverage downsamples WattTime's ~5-minute-resolution points into 24 hourly slots
// by averaging every point that falls within each hour, and converts MOER (lbs/MWh) to
// gCO2eq/kWh so the result is comparable across providers.
func hourlyAverage(points []wtForecastPoint) ([]models.GridDataPoint, error) {
	type bucket struct {
		sum   float64
		count int
		hour  time.Time
	}
	buckets := map[time.Time]*bucket{}

	for _, p := range points {
		ts, err := time.Parse(time.RFC3339, p.PointTime)
		if err != nil {
			continue
		}
		hourKey := ts.Truncate(time.Hour)
		b, ok := buckets[hourKey]
		if !ok {
			b = &bucket{hour: hourKey}
			buckets[hourKey] = b
		}
		b.sum += p.Value
		b.count++
	}

	if len(buckets) == 0 {
		return nil, fmt.Errorf("watttime: no forecast points returned")
	}

	result := make([]models.GridDataPoint, 0, len(buckets))
	for _, b := range buckets {
		avgMOER := b.sum / float64(b.count)
		result = append(result, models.GridDataPoint{
			Timestamp:       b.hour,
			CarbonIntensity: avgMOER * lbsPerMWhToGramsPerKWh,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp.Before(result[j].Timestamp)
	})

	if len(result) > 24 {
		result = result[:24]
	}

	return result, nil
}

func zipToBAFor(zip string) string {
	if len(zip) < 3 {
		return "CAISO_NORTH"
	}
	if ba, ok := zipToBA[zip[:3]]; ok {
		return ba
	}
	return "CAISO_NORTH"
}
