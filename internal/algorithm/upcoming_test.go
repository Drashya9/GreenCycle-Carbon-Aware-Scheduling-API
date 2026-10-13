package algorithm

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/Drashya9/GreenCycle-Carbon-Aware-Scheduling-API/internal/models"
)

func TestUpcomingSlotsDropsStartedHours(t *testing.T) {
	base := time.Date(2026, 10, 12, 3, 0, 0, 0, time.UTC)
	forecast := []models.GridDataPoint{
		{Timestamp: base},                    // 03:00, already started at 03:35
		{Timestamp: base.Add(time.Hour)},     // 04:00
		{Timestamp: base.Add(2 * time.Hour)}, // 05:00
	}

	got := UpcomingSlots(forecast, base.Add(35*time.Minute))
	if len(got) != 2 || !got[0].Timestamp.Equal(base.Add(time.Hour)) {
		t.Fatalf("expected the 04:00 and 05:00 slots, got %+v", got)
	}

	// A slot starting exactly now hasn't started yet, so it's kept.
	if got := UpcomingSlots(forecast, base); len(got) != 3 {
		t.Errorf("expected all 3 slots when now is exactly the first slot's start, got %d", len(got))
	}
	if got := UpcomingSlots(forecast, base.Add(3*time.Hour)); len(got) != 0 {
		t.Errorf("expected no slots once the whole forecast is in the past, got %d", len(got))
	}
}

func TestRecommendationNeverStartsInThePast(t *testing.T) {
	now := time.Now()
	start := now.Truncate(time.Hour) // the current, already-started hour
	forecast := make([]models.GridDataPoint, 24)
	for i := range forecast {
		v := 300.0
		if i == 0 {
			v = 10 // by far the cleanest hour — but it started up to 59 minutes ago
		}
		forecast[i] = models.GridDataPoint{Timestamp: start.Add(time.Duration(i) * time.Hour), CarbonIntensity: v}
	}

	window, err := FindOptimalWindow(UpcomingSlots(forecast, now), 1, "Dryer")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if window.StartTime.Before(now) {
		t.Errorf("recommended a window starting at %v, before now (%v)", window.StartTime, now)
	}
}

func TestHugeOrNaNDurationIsRejectedNotWrapped(t *testing.T) {
	forecast := buildForecast(flat(24, 200)...)

	for _, d := range []float64{1e308, math.Inf(1), math.NaN(), 25} {
		_, err := FindOptimalWindow(forecast, d, "Bad Appliance")
		if err == nil || !strings.Contains(err.Error(), "needs") {
			t.Errorf("duration %v: expected a 'needs' error, got %v", d, err)
		}
	}
}
