package models

// Appliance is a household device that consumes electricity in a single cycle.
type Appliance struct {
	ID            int64   `db:"id" json:"id"`
	Name          string  `db:"name" json:"name"`
	DurationHours float64 `db:"duration_hours" json:"durationHours"`
}
