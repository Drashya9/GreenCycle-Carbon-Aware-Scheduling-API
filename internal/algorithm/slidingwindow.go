// Package algorithm finds the cleanest contiguous block of hours in a carbon forecast.
package algorithm

import (
	"fmt"
	"math"
	"time"

	"github.com/Drashya9/GreenCycle-Carbon-Aware-Scheduling-API/internal/models"
)

// FindOptimalWindow finds the contiguous block of hours with the lowest average carbon
// intensity, using a classic O(n) sliding window: compute the first window's sum, then
// slide right one slot at a time, subtracting the slot that leaves and adding the slot
// that enters.
//
// durationHours is rounded up to a whole number of slots (e.g. 1.5h -> 2 slots).
func FindOptimalWindow(forecast []models.GridDataPoint, durationHours float64, applianceName string) (*models.OptimalWindow, error) {
	windowSize := int(math.Ceil(durationHours))
	if windowSize < 1 {
		windowSize = 1
	}

	if len(forecast) < windowSize {
		return nil, fmt.Errorf("forecast has %d hours but appliance needs %d hours", len(forecast), windowSize)
	}

	var windowSum float64
	for i := 0; i < windowSize; i++ {
		windowSum += forecast[i].CarbonIntensity
	}

	minSum := windowSum
	minStartIdx := 0

	for i := 1; i <= len(forecast)-windowSize; i++ {
		windowSum -= forecast[i-1].CarbonIntensity
		windowSum += forecast[i+windowSize-1].CarbonIntensity

		if windowSum < minSum {
			minSum = windowSum
			minStartIdx = i
		}
	}

	avgIntensity := math.Round((minSum/float64(windowSize))*10) / 10

	startSlot := forecast[minStartIdx]
	endSlot := forecast[minStartIdx+windowSize-1].Timestamp.Add(time.Hour)

	recommendation := fmt.Sprintf(
		"Run your %s at %s — avg. %.0f gCO₂/kWh (done by %s).",
		applianceName,
		startSlot.Timestamp.Format("3:04 PM MST"),
		avgIntensity,
		endSlot.Format("3:04 PM MST"),
	)

	return &models.OptimalWindow{
		StartTime:              startSlot.Timestamp,
		EndTime:                endSlot,
		AverageCarbonIntensity: avgIntensity,
		ApplianceName:          applianceName,
		Recommendation:         recommendation,
	}, nil
}
