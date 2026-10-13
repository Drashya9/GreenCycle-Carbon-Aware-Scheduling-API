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

const (
	// claimLease is how long a run may sit claimed as 'fired' with no notification before
	// it's treated as abandoned mid-delivery (e.g. the process died or the function was cut
	// off) and becomes claimable again. Far longer than one fire takes.
	claimLease = 5 * time.Minute
	// reclaimWindow bounds how far back abandoned claims are recovered, so old rows (e.g.
	// claimed before recovery existed) are never resurrected.
	reclaimWindow = 24 * time.Hour
)

// dueCondition is the one definition of "claimable", shared by FindDuePending and
// ClaimForFiring so they can't disagree: a pending run whose start time and any retry
// backoff have passed, or a claimed run abandoned mid-delivery. Arguments are placeholders.
func dueCondition(now, leaseCutoff, windowStart string) string {
	return `(status = 'pending' AND start_time <= ` + now + `
	         AND (next_attempt_at IS NULL OR next_attempt_at <= ` + now + `))
	     OR (status = 'fired' AND fired_at <= ` + leaseCutoff + ` AND fired_at > ` + windowStart + `
	         AND NOT EXISTS (SELECT 1 FROM notifications n WHERE n.scheduled_run_id = scheduled_run.id))`
}

// FindDuePending returns claimable runs (see dueCondition), oldest first.
func (r *ScheduledRunRepository) FindDuePending(ctx context.Context, now time.Time, limit int) ([]models.ScheduledRun, error) {
	var out []models.ScheduledRun
	err := r.db.SelectContext(ctx, &out, `
		SELECT * FROM scheduled_run
		WHERE `+dueCondition("$1", "$2", "$3")+`
		ORDER BY start_time
		LIMIT $4`, now, now.Add(-claimLease), now.Add(-reclaimWindow), limit)
	return out, err
}

// ClaimForFiring atomically marks a claimable run fired and returns it. The WHERE clause is
// a compare-and-swap: only the one caller whose UPDATE matches the row gets it, so
// concurrent ticks, overlapping cron calls or multiple instances can never fire a run
// twice. A stale reclaim also refreshes fired_at, so a second reclaimer no longer matches.
func (r *ScheduledRunRepository) ClaimForFiring(ctx context.Context, id int64, firedAt time.Time) (*models.ScheduledRun, error) {
	var run models.ScheduledRun
	err := r.db.GetContext(ctx, &run, `
		UPDATE scheduled_run
		SET status = 'fired', fired_at = $2, attempt_count = attempt_count + 1
		WHERE id = $1 AND (`+dueCondition("$2", "$3", "$4")+`)
		RETURNING *`, id, firedAt, firedAt.Add(-claimLease), firedAt.Add(-reclaimWindow))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound // already claimed by another tick/instance
	}
	if err != nil {
		return nil, err
	}
	return &run, nil
}

// MarkRetry returns a claimed run whose delivery failed to pending, to be claimed again
// once nextAttempt has passed. The status guard keeps it from touching a cancelled run.
func (r *ScheduledRunRepository) MarkRetry(ctx context.Context, id int64, lastError string, nextAttempt time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE scheduled_run SET status = 'pending', last_error = $2, next_attempt_at = $3
		WHERE id = $1 AND status = 'fired'`,
		id, lastError, nextAttempt)
	return err
}

// MarkFailed records a delivery that failed for the last time; the run stays terminal
// (status='failed'). The engine still writes a notification, so the failure is never silent.
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
