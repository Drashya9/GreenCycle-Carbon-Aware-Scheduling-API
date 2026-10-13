package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jmoiron/sqlx"

	"github.com/Drashya9/GreenCycle-Carbon-Aware-Scheduling-API/internal/models"
)

var ErrNotFound = errors.New("not found")

// ErrConflict means a write violated a uniqueness constraint, e.g. a duplicate appliance name.
var ErrConflict = errors.New("conflict")

// ErrProtected means the appliance is a seeded demo fixture, which can't be deleted.
var ErrProtected = errors.New("protected")

// ErrHasPendingRuns means the appliance still has scheduled runs waiting to fire.
var ErrHasPendingRuns = errors.New("has pending runs")

// pgUniqueViolation is Postgres's SQLSTATE for unique_violation.
const pgUniqueViolation = "23505"

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation
}

type ApplianceRepository struct {
	db *sqlx.DB
}

func NewApplianceRepository(db *sqlx.DB) *ApplianceRepository {
	return &ApplianceRepository{db: db}
}

func (r *ApplianceRepository) FindAll(ctx context.Context) ([]models.Appliance, error) {
	var out []models.Appliance
	err := r.db.SelectContext(ctx, &out, `SELECT id, name, duration_hours, is_seed FROM appliance ORDER BY id`)
	return out, err
}

func (r *ApplianceRepository) FindByNameIgnoreCase(ctx context.Context, name string) (*models.Appliance, error) {
	var a models.Appliance
	err := r.db.GetContext(ctx, &a,
		`SELECT id, name, duration_hours FROM appliance
		 WHERE LOWER(TRIM(name)) = LOWER(TRIM($1))
		 ORDER BY id LIMIT 1`, name)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *ApplianceRepository) FindByID(ctx context.Context, id int64) (*models.Appliance, error) {
	var a models.Appliance
	err := r.db.GetContext(ctx, &a,
		`SELECT id, name, duration_hours FROM appliance WHERE id = $1`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *ApplianceRepository) Create(ctx context.Context, a *models.Appliance) error {
	err := r.db.GetContext(ctx, &a.ID,
		`INSERT INTO appliance (name, duration_hours) VALUES ($1, $2) RETURNING id`,
		a.Name, a.DurationHours)
	if isUniqueViolation(err) {
		return ErrConflict
	}
	return err
}

// Delete removes a user-created appliance. Seeded demo appliances are refused
// (ErrProtected), and so is one that still has runs waiting to fire (ErrHasPendingRuns) —
// deleting it would otherwise silently cascade-delete those runs.
func (r *ApplianceRepository) Delete(ctx context.Context, id int64) error {
	res, err := r.db.ExecContext(ctx, `
		DELETE FROM appliance a
		WHERE a.id = $1 AND NOT a.is_seed
		  AND NOT EXISTS (SELECT 1 FROM scheduled_run s WHERE s.appliance_id = a.id AND s.status = 'pending')`, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 1 {
		return nil
	}

	// Nothing was deleted; work out why.
	var why struct {
		IsSeed  bool `db:"is_seed"`
		Pending bool `db:"pending"`
	}
	err = r.db.GetContext(ctx, &why, `
		SELECT a.is_seed,
		       EXISTS (SELECT 1 FROM scheduled_run s WHERE s.appliance_id = a.id AND s.status = 'pending') AS pending
		FROM appliance a WHERE a.id = $1`, id)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return ErrNotFound
	case err != nil:
		return err
	case why.IsSeed:
		return ErrProtected
	default:
		// Either it still has pending runs, or it had one at delete time that has fired or
		// been cancelled since; either way the caller should resolve its runs and retry.
		return ErrHasPendingRuns
	}
}
