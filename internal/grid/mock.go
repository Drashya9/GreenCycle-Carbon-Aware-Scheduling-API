package grid

import (
	"context"
	"math"
	"math/rand"
	"time"

	"github.com/Drashya9/GreenCycle-Carbon-Aware-Scheduling-API/internal/models"
)

// hourlyBase holds base carbon intensities (gCO2eq/kWh) per hour of day (index 0 = midnight),
// mimicking a typical US grid day: low overnight, rising in the morning, dipping at midday
// solar peak, spiking in the evening.
var hourlyBase = [24]float64{
	180, 160, 145, 135, 130, 140, // 0-5 AM
	180, 220, 260, 280, 300, 310, // 6-11 AM
	280, 240, 210, 230, 270, 320, // 12-17 PM
	380, 400, 370, 320, 260, 210, // 18-23 PM
}

// MockProvider generates a synthetic 24-hour forecast — no API key required.
type MockProvider struct{}

func NewMockProvider() *MockProvider { return &MockProvider{} }

func (m *MockProvider) Name() string { return "mock" }

func (m *MockProvider) GetForecast(ctx context.Context, zipCode string) ([]models.GridDataPoint, error) {
	now := time.Now().Truncate(time.Hour)
	forecast := make([]models.GridDataPoint, 24)

	for i := 0; i < 24; i++ {
		slotTime := now.Add(time.Duration(i) * time.Hour)
		hourOfDay := slotTime.Hour()

		jitter := (rand.Float64() - 0.5) * 60
		intensity := math.Max(50, hourlyBase[hourOfDay]+jitter)

		forecast[i] = models.GridDataPoint{
			Timestamp:       slotTime,
			CarbonIntensity: math.Round(intensity*10) / 10,
		}
	}

	return forecast, nil
}
