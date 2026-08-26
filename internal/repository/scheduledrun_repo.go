package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/Drashya9/GreenCycle-Carbon-Aware-Scheduling-API/internal/models"
)

type ScheduledRunRepository struct {
	db *sqlx.DB
}

func NewScheduledRunRepository(db *sqlx.DB) *ScheduledRunRepository {
	return &ScheduledRunRepository{db: db}
}

func (r *ScheduledRunRepository) Create(ctx context.Context, run *models.ScheduledRun) error {
	row := r.db.QueryRowxContext(ctx, `
		INSERT INTO scheduled_run
			(appliance_id, appliance_name, zip_code, start_time, end_time,
			 average_carbon_intensity, webhook_url, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, 'pending')
		RETURNING id, created_at, status, attempt_count`,
		run.ApplianceID, run.ApplianceName, run.ZipCode, run.StartTime, run.EndTime,
		run.AverageCarbonIntensity, run.WebhookURL)

	return row.Scan(&run.ID, &run.CreatedAt, &run.Status, &run.AttemptCount)
}

func (r *ScheduledRunRepository) FindAll(ctx context.Context, status string) ([]models.ScheduledRun, error) {
	var out []models.ScheduledRun
	if status == "" {
		err := r.db.SelectContext(ctx, &out, `SELECT * FROM scheduled_run ORDER BY start_time`)
		return out, err
	}
	err := r.db.SelectContext(ctx, &out,
		`SELECT * FROM scheduled_run WHERE status = $1 ORDER BY start_time`, status)
	return out, err
}

func (r *ScheduledRunRepository) FindByID(ctx context.Context, id int64) (*models.ScheduledRun, error) {
	var run models.ScheduledRun
	err := r.db.GetContext(ctx, &run, `SELECT * FROM scheduled_run WHERE id = $1`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &run, nil
}

// Cancel marks a still-pending run as cancelled. It's a no-op error (ErrNotFound) if the
// run doesn't exist or has already fired/failed/been cancelled — same compare-and-swap
// shape as the engine's claim, so a cancel racing a fire can never leave the row ambiguous.
func (r *ScheduledRunRepository) Cancel(ctx context.Context, id int64) error {
	res, err := r.db.ExecContext(ctx,
		`UPDATE scheduled_run SET status = 'cancelled' WHERE id = $1 AND status = 'pending'`, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// FindDuePending returns pending runs whose window has already started, oldest first.
func (r *ScheduledRunRepository) FindDuePending(ctx context.Context, now time.Time, limit int) ([]models.ScheduledRun, error) {
	var out []models.ScheduledRun
	err := r.db.SelectContext(ctx, &out, `
		SELECT * FROM scheduled_run
		WHERE status = 'pending' AND start_time <= $1
		ORDER BY start_time
		LIMIT $2`, now, limit)
	return out, err
}

// ClaimForFiring atomically flips a pending run to fired, returning the claimed row.
// The WHERE status = 'pending' guard is a compare-and-swap: only the one caller whose
// UPDATE actually matches a row gets it, so concurrent poll ticks (or a restart racing
// the previous process's still-draining tick) can never fire the same run twice.
func (r *ScheduledRunRepository) ClaimForFiring(ctx context.Context, id int64, firedAt time.Time) (*models.ScheduledRun, error) {
	var run models.ScheduledRun
	err := r.db.GetContext(ctx, &run, `
		UPDATE scheduled_run
		SET status = 'fired', fired_at = $2, attempt_count = attempt_count + 1
		WHERE id = $1 AND status = 'pending'
		RETURNING *`, id, firedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound // already claimed by another tick/instance
	}
	if err != nil {
		return nil, err
	}
	return &run, nil
}

// MarkFailed records a delivery failure. The row stays terminal (status='failed') rather
// than retried — the notification row is still written so the failure is never silent.
func (r *ScheduledRunRepository) MarkFailed(ctx context.Context, id int64, lastError string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE scheduled_run SET status = 'failed', last_error = $2 WHERE id = $1`,
		id, lastError)
	return err
}

func (r *ScheduledRunRepository) CreateNotification(ctx context.Context, n *models.Notification) error {
	row := r.db.QueryRowxContext(ctx, `
		INSERT INTO notifications (scheduled_run_id, message, delivered_webhook)
		VALUES ($1, $2, $3)
		RETURNING id, created_at`,
		n.ScheduledRunID, n.Message, n.DeliveredWebhook)
	return row.Scan(&n.ID, &n.CreatedAt)
}
