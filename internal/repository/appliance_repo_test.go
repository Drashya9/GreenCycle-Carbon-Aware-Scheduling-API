package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Drashya9/GreenCycle-Carbon-Aware-Scheduling-API/internal/models"
)

func TestApplianceRepository_CreateAndFindByNameIgnoreCase(t *testing.T) {
	dbx := openTestDB(t)
	repo := NewApplianceRepository(dbx)

	name := fmt.Sprintf("Test Appliance %d", time.Now().UnixNano())
	a := &models.Appliance{Name: name, DurationHours: 2.5}
	if err := repo.Create(context.Background(), a); err != nil {
		t.Fatalf("create: %v", err)
	}
	defer repo.Delete(context.Background(), a.ID)

	if a.ID == 0 {
		t.Fatal("expected a non-zero id after create")
	}

	// FindByNameIgnoreCase is queried with LOWER(name) = LOWER($1) — verify that actually
	// works against real Postgres, not just in an in-memory fake.
	found, err := repo.FindByNameIgnoreCase(context.Background(), strings.ToUpper(name))
	if err != nil {
		t.Fatalf("expected case-insensitive lookup to succeed: %v", err)
	}
	if found.ID != a.ID {
		t.Errorf("expected id %d, got %d", a.ID, found.ID)
	}
	if found.DurationHours != 2.5 {
		t.Errorf("expected durationHours 2.5, got %v", found.DurationHours)
	}
}

func TestApplianceRepository_FindByNameIgnoreCase_NotFound(t *testing.T) {
	dbx := openTestDB(t)
	repo := NewApplianceRepository(dbx)

	_, err := repo.FindByNameIgnoreCase(context.Background(), "Definitely Not A Real Appliance")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got: %v", err)
	}
}

func TestApplianceRepository_Delete_NotFound(t *testing.T) {
	dbx := openTestDB(t)
	repo := NewApplianceRepository(dbx)

	err := repo.Delete(context.Background(), -1) // an id that can never exist
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound deleting a nonexistent appliance, got: %v", err)
	}
}
