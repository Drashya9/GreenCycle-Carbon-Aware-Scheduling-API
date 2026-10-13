package grid

import (
	"context"
	"fmt"
	"hash/fnv"
	"math"
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
	// Start at the next whole hour so all 24 slots are still ahead of us.
	now := time.Now().Truncate(time.Hour).Add(time.Hour)
	forecast := make([]models.GridDataPoint, 24)

	for i := 0; i < 24; i++ {
		slotTime := now.Add(time.Duration(i) * time.Hour)
		hourOfDay := slotTime.Hour()

		intensity := math.Max(50, hourlyBase[hourOfDay]+jitter(zipCode, slotTime))

		forecast[i] = models.GridDataPoint{
			Timestamp:       slotTime,
			CarbonIntensity: math.Round(intensity*10) / 10,
		}
	}

	return forecast, nil
}

// jitter is a deterministic ±30 offset per (zip, hour): the same request always gets the
// same answer, and different zips differ in a stable way, so Multi-Zone comparisons mean
// something instead of reflecting fresh random noise on every call.
func jitter(zipCode string, slot time.Time) float64 {
	h := fnv.New64a()
	fmt.Fprintf(h, "%s|%d", zipCode, slot.Unix())
	unit := float64(h.Sum64()>>11) / (1 << 53) // uniform in [0, 1)
	return (unit - 0.5) * 60
}
