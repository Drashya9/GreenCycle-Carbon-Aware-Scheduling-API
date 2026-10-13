package grid

import (
	"context"
	"testing"
	"time"
)

func TestMockForecastIsDeterministicPerZip(t *testing.T) {
	m := NewMockProvider()
	a1, _ := m.GetForecast(context.Background(), "85281")
	a2, _ := m.GetForecast(context.Background(), "85281")
	b, _ := m.GetForecast(context.Background(), "10001")

	same, differs := true, false
	for i := range a1 {
		if a1[i] != a2[i] {
			same = false
		}
		if a1[i].CarbonIntensity != b[i].CarbonIntensity {
			differs = true
		}
	}
	if !same {
		t.Error("two forecasts for the same zip and hour should be identical")
	}
	if !differs {
		t.Error("different zips should get different forecasts")
	}
}

func TestMockForecastStartsAtTheNextHour(t *testing.T) {
	forecast, _ := NewMockProvider().GetForecast(context.Background(), "85281")
	if len(forecast) != 24 {
		t.Fatalf("expected 24 slots, got %d", len(forecast))
	}
	if !forecast[0].Timestamp.After(time.Now()) {
		t.Errorf("expected every slot to be in the future, first starts at %v", forecast[0].Timestamp)
	}
}

func TestCacheTTLStopsAtTheHourBoundary(t *testing.T) {
	at := func(hh, mm int) time.Time { return time.Date(2026, 10, 13, hh, mm, 0, 0, time.UTC) }

	if got := cacheTTL(at(4, 58), 15*time.Minute); got != 2*time.Minute {
		t.Errorf("at 04:58 a 15m TTL should be cut to 2m, got %v", got)
	}
	if got := cacheTTL(at(4, 10), 15*time.Minute); got != 15*time.Minute {
		t.Errorf("at 04:10 the full 15m TTL fits before the hour, got %v", got)
	}
}
