package algorithm

import (
	"strings"
	"testing"
	"time"

	"github.com/Drashya9/GreenCycle-Carbon-Aware-Scheduling-API/internal/models"
)

func buildForecast(intensities ...float64) []models.GridDataPoint {
	base := time.Now().Truncate(time.Hour)
	forecast := make([]models.GridDataPoint, len(intensities))
	for i, v := range intensities {
		forecast[i] = models.GridDataPoint{
			Timestamp:       base.Add(time.Duration(i) * time.Hour),
			CarbonIntensity: v,
		}
	}
	return forecast
}

func flat(n int, value float64) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = value
	}
	return out
}

func TestOptimalAtStart(t *testing.T) {
	intensities := append([]float64{50, 60}, flat(22, 300)...)
	forecast := buildForecast(intensities...)

	result, err := FindOptimalWindow(forecast, 2.0, "Washer")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.StartTime.Equal(forecast[0].Timestamp) {
		t.Errorf("expected start time %v, got %v", forecast[0].Timestamp, result.StartTime)
	}
	if result.AverageCarbonIntensity != 55.0 {
		t.Errorf("expected avg 55.0, got %v", result.AverageCarbonIntensity)
	}
}

func TestOptimalInMiddle(t *testing.T) {
	intensities := flat(24, 300)
	intensities[10], intensities[11], intensities[12] = 80, 70, 90
	forecast := buildForecast(intensities...)

	result, err := FindOptimalWindow(forecast, 3.0, "Dryer")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.StartTime.Equal(forecast[10].Timestamp) {
		t.Errorf("expected start time %v, got %v", forecast[10].Timestamp, result.StartTime)
	}
}

func TestOptimalAtEnd(t *testing.T) {
	intensities := flat(22, 400)
	intensities = append(intensities, 100, 110)
	forecast := buildForecast(intensities...)

	result, err := FindOptimalWindow(forecast, 2.0, "Dishwasher")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.StartTime.Equal(forecast[22].Timestamp) {
		t.Errorf("expected start time %v, got %v", forecast[22].Timestamp, result.StartTime)
	}
}

func TestAllEqual(t *testing.T) {
	forecast := buildForecast(flat(24, 200)...)

	result, err := FindOptimalWindow(forecast, 3.0, "Pool Pump")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.StartTime.Equal(forecast[0].Timestamp) {
		t.Errorf("expected start time %v, got %v", forecast[0].Timestamp, result.StartTime)
	}
	if result.AverageCarbonIntensity != 200.0 {
		t.Errorf("expected avg 200.0, got %v", result.AverageCarbonIntensity)
	}
}

func TestWindowTooLarge(t *testing.T) {
	forecast := buildForecast(100, 200, 300)

	_, err := FindOptimalWindow(forecast, 5.0, "EV Charger")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "needs") {
		t.Errorf("expected error message to contain 'needs', got: %v", err)
	}
}

func TestFractionalDuration(t *testing.T) {
	intensities := flat(24, 300)
	intensities[5], intensities[6] = 50, 60
	forecast := buildForecast(intensities...)

	// 1.5 hours -> ceil -> 2 slots; cheapest 2-slot window is [50,60] at idx 5
	result, err := FindOptimalWindow(forecast, 1.5, "Water Heater")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.StartTime.Equal(forecast[5].Timestamp) {
		t.Errorf("expected start time %v, got %v", forecast[5].Timestamp, result.StartTime)
	}
	if result.AverageCarbonIntensity != 55.0 {
		t.Errorf("expected avg 55.0, got %v", result.AverageCarbonIntensity)
	}
}

func TestRecommendationContainsName(t *testing.T) {
	forecast := buildForecast(
		100, 200, 300, 100, 200, 300, 100, 200,
		300, 100, 200, 300, 100, 200, 300, 100,
		200, 300, 100, 200, 300, 100, 200, 300,
	)
	result, err := FindOptimalWindow(forecast, 1.0, "Fancy Toaster")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Recommendation, "Fancy Toaster") {
		t.Errorf("expected recommendation to contain appliance name, got: %v", result.Recommendation)
	}
}
