package models

import "time"

// GridDataPoint is one hourly slot in a carbon-intensity forecast.
//
// CarbonIntensity is grams of CO2-equivalent per kilowatt-hour (gCO2eq/kWh).
// Lower is cleaner (more solar/wind on the grid), higher is dirtier (more coal/gas).
type GridDataPoint struct {
	Timestamp       time.Time `json:"timestamp"`
	CarbonIntensity float64   `json:"carbonIntensity"`
}
