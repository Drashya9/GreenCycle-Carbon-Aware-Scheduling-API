package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jmoiron/sqlx"

	"github.com/Drashya9/GreenCycle-Carbon-Aware-Scheduling-API/internal/models"
)

var ErrNotFound = errors.New("not found")

type ApplianceRepository struct {
	db *sqlx.DB
}

func NewApplianceRepository(db *sqlx.DB) *ApplianceRepository {
	return &ApplianceRepository{db: db}
}

func (r *ApplianceRepository) FindAll(ctx context.Context) ([]models.Appliance, error) {
	var out []models.Appliance
	err := r.db.SelectContext(ctx, &out, `SELECT id, name, duration_hours FROM appliance ORDER BY id`)
	return out, err
}

func (r *ApplianceRepository) FindByNameIgnoreCase(ctx context.Context, name string) (*models.Appliance, error) {
	var a models.Appliance
	err := r.db.GetContext(ctx, &a,
		`SELECT id, name, duration_hours FROM appliance WHERE LOWER(name) = LOWER($1)`, name)
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
	return r.db.GetContext(ctx, &a.ID,
		`INSERT INTO appliance (name, duration_hours) VALUES ($1, $2) RETURNING id`,
		a.Name, a.DurationHours)
}

func (r *ApplianceRepository) Delete(ctx context.Context, id int64) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM appliance WHERE id = $1`, id)
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
