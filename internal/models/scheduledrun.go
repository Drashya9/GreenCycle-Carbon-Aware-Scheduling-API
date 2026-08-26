package models

import (
	"database/sql"
	"time"
)

type RunStatus string

const (
	StatusPending   RunStatus = "pending"
	StatusFired     RunStatus = "fired"
	StatusFailed    RunStatus = "failed"
	StatusCancelled RunStatus = "cancelled"
)

// ScheduledRun is a persisted commitment to run an appliance during a specific window.
// The scheduler engine polls for due rows and fires a webhook + notification when the
// window starts.
//
// WebhookURL/FiredAt/LastError use sql.Null[T] rather than sql.NullString/NullTime: it
// still scans NULL correctly via sqlx, but (unlike the older Null types) also marshals
// to JSON as the plain value or `null` instead of leaking its {V, Valid} struct shape.
type ScheduledRun struct {
	ID                     int64               `db:"id" json:"id"`
	ApplianceID            int64               `db:"appliance_id" json:"applianceId"`
	ApplianceName          string              `db:"appliance_name" json:"applianceName"`
	ZipCode                string              `db:"zip_code" json:"zip"`
	StartTime              time.Time           `db:"start_time" json:"startTime"`
	EndTime                time.Time           `db:"end_time" json:"endTime"`
	AverageCarbonIntensity float64             `db:"average_carbon_intensity" json:"averageCarbonIntensity"`
	WebhookURL             sql.Null[string]    `db:"webhook_url" json:"webhookUrl"`
	Status                 RunStatus           `db:"status" json:"status"`
	CreatedAt              time.Time           `db:"created_at" json:"createdAt"`
	FiredAt                sql.Null[time.Time] `db:"fired_at" json:"firedAt"`
	AttemptCount           int                 `db:"attempt_count" json:"attemptCount"`
	LastError              sql.Null[string]    `db:"last_error" json:"lastError"`
}

// Notification is a record of a scheduled run firing, kept regardless of whether the
// webhook delivery itself succeeded.
type Notification struct {
	ID               int64     `db:"id" json:"id"`
	ScheduledRunID   int64     `db:"scheduled_run_id" json:"scheduledRunId"`
	Message          string    `db:"message" json:"message"`
	DeliveredWebhook bool      `db:"delivered_webhook" json:"deliveredWebhook"`
	CreatedAt        time.Time `db:"created_at" json:"createdAt"`
}
