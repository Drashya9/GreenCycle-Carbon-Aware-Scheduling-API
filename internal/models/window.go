package models

import "time"

// OptimalWindow is the algorithm's answer: the cleanest contiguous time block found.
type OptimalWindow struct {
	StartTime              time.Time `json:"startTime"`
	EndTime                time.Time `json:"endTime"`
	AverageCarbonIntensity float64   `json:"averageCarbonIntensity"`
	ApplianceName          string    `json:"applianceName"`
	Recommendation         string    `json:"recommendation"`
	// Provider is the grid data source that answered first (e.g. "mock", "electricitymaps").
	Provider string `json:"provider,omitempty"`
}

// ZoneResult is one zip code's OptimalWindow, produced by a concurrent multi-zone fetch.
type ZoneResult struct {
	Zip    string         `json:"zip"`
	Window *OptimalWindow `json:"window,omitempty"`
	Error  string         `json:"error,omitempty"`
}
