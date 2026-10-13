package models

// Appliance is a household device that consumes electricity in a single cycle.
type Appliance struct {
	ID            int64   `db:"id" json:"id"`
	Name          string  `db:"name" json:"name"`
	DurationHours float64 `db:"duration_hours" json:"durationHours"`
	// Protected marks the seeded demo appliances, which can't be deleted.
	Protected bool `db:"is_seed" json:"protected"`
}
