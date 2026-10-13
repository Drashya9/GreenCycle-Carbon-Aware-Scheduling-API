package algorithm

import (
	"time"

	"github.com/Drashya9/GreenCycle-Carbon-Aware-Scheduling-API/internal/models"
)

// UpcomingSlots drops slots that start before now, so a recommendation can never begin in
// the past. Forecasts usually start at the current hour rounded down, and cached ones age
// further, so without this the cleanest "window" could be one that started 59 minutes ago.
func UpcomingSlots(forecast []models.GridDataPoint, now time.Time) []models.GridDataPoint {
	for i, slot := range forecast {
		if !slot.Timestamp.Before(now) {
			return forecast[i:]
		}
	}
	return nil
}
