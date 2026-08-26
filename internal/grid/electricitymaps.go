package grid

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Drashya9/GreenCycle-Carbon-Aware-Scheduling-API/internal/models"
)

// zipToZoneEM is a minimal ZIP-prefix -> Electricity Maps grid-zone lookup table.
// A production app would geocode ZIP -> lat/lon -> zone; this covers a handful of
// populous states, same as the original Java implementation.
var zipToZoneEM = map[string]string{
	"900": "US-CAL-CISO", "901": "US-CAL-CISO", "902": "US-CAL-CISO",
	"940": "US-CAL-CISO", "941": "US-CAL-CISO", "945": "US-CAL-CISO",
	"852": "US-SW-PNM", "853": "US-SW-PNM",
	"750": "US-TEX-ERCOT", "770": "US-TEX-ERCOT",
	"100": "US-NY-NYIS", "110": "US-NY-NYIS",
	"331": "US-FLA-FPL", "332": "US-FLA-FPL",
	"606": "US-MIDW-AMMO", "607": "US-MIDW-AMMO",
}

type ElectricityMapsProvider struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

func NewElectricityMapsProvider(apiKey, baseURL string) *ElectricityMapsProvider {
	return &ElectricityMapsProvider{
		apiKey:     apiKey,
		baseURL:    baseURL,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (e *ElectricityMapsProvider) Name() string { return "electricitymaps" }

type emForecastResponse struct {
	Forecast []struct {
		Datetime        string  `json:"datetime"`
		CarbonIntensity float64 `json:"carbonIntensity"`
	} `json:"forecast"`
}

func (e *ElectricityMapsProvider) GetForecast(ctx context.Context, zipCode string) ([]models.GridDataPoint, error) {
	zone := zipToZone(zipCode)
	url := fmt.Sprintf("%s/carbon-intensity/forecast?zone=%s", e.baseURL, zone)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("auth-token", e.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("electricitymaps: unexpected status %d", resp.StatusCode)
	}

	var body emForecastResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("electricitymaps: decode response: %w", err)
	}

	n := len(body.Forecast)
	if n > 24 {
		n = 24
	}

	result := make([]models.GridDataPoint, 0, n)
	for i := 0; i < n; i++ {
		slot := body.Forecast[i]
		ts, err := time.Parse(time.RFC3339, slot.Datetime)
		if err != nil {
			return nil, fmt.Errorf("electricitymaps: parse timestamp %q: %w", slot.Datetime, err)
		}
		result = append(result, models.GridDataPoint{
			Timestamp:       ts,
			CarbonIntensity: slot.CarbonIntensity,
		})
	}

	return result, nil
}

func zipToZone(zip string) string {
	if len(zip) < 3 {
		return "US-CAL-CISO"
	}
	if zone, ok := zipToZoneEM[zip[:3]]; ok {
		return zone
	}
	return "US-CAL-CISO"
}
