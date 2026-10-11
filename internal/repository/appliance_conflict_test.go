package repository

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Drashya9/GreenCycle-Carbon-Aware-Scheduling-API/internal/models"
)

func TestIsUniqueViolation(t *testing.T) {
	wrapped := fmt.Errorf("insert: %w", &pgconn.PgError{Code: pgUniqueViolation})
	if !isUniqueViolation(wrapped) {
		t.Error("expected a wrapped 23505 to be recognized")
	}
	if isUniqueViolation(&pgconn.PgError{Code: "23503"}) {
		t.Error("a foreign-key violation (23503) is not a unique violation")
	}
	if isUniqueViolation(errors.New("connection refused")) || isUniqueViolation(nil) {
		t.Error("non-Postgres errors must not be treated as conflicts")
	}
}

// TestApplianceRepository_CreateDuplicateReturnsErrConflict runs against a real Postgres
// (skipped without one; CI provides it), proving the driver's actual error is mapped.
func TestApplianceRepository_CreateDuplicateReturnsErrConflict(t *testing.T) {
	dbx := openTestDB(t)
	repo := NewApplianceRepository(dbx)

	name := fmt.Sprintf("Duplicate Test %d", time.Now().UnixNano())
	first := &models.Appliance{Name: name, DurationHours: 1}
	if err := repo.Create(context.Background(), first); err != nil {
		t.Fatalf("create first: %v", err)
	}
	defer repo.Delete(context.Background(), first.ID)

	err := repo.Create(context.Background(), &models.Appliance{Name: name, DurationHours: 2})
	if !errors.Is(err, ErrConflict) {
		t.Errorf("expected ErrConflict for a duplicate name, got: %v", err)
	}
}
