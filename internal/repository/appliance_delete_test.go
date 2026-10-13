package repository

import (
	"context"
	"errors"
	"testing"
	"time"
)

// Runs against a real Postgres with migration 0005 applied (skipped without one; CI provides it).
func TestDeleteRefusesSeededAppliances(t *testing.T) {
	dbx := openTestDB(t)
	repo := NewApplianceRepository(dbx)

	dryer, err := repo.FindByNameIgnoreCase(context.Background(), "Dryer")
	if err != nil {
		t.Fatalf("the seeded Dryer should exist: %v", err)
	}
	if err := repo.Delete(context.Background(), dryer.ID); !errors.Is(err, ErrProtected) {
		t.Errorf("expected ErrProtected deleting a seeded appliance, got %v", err)
	}
	if _, err := repo.FindByNameIgnoreCase(context.Background(), "Dryer"); err != nil {
		t.Errorf("the seeded appliance must still exist: %v", err)
	}
}

func TestDeleteRefusesAppliancesWithPendingRuns(t *testing.T) {
	dbx := openTestDB(t)
	appliances := NewApplianceRepository(dbx)
	runs := NewScheduledRunRepository(dbx)
	appliance := createTestAppliance(t, dbx)

	run := newTestRun(appliance.ID, appliance.Name, time.Now().Add(time.Hour), time.Now().Add(2*time.Hour))
	if err := runs.Create(context.Background(), run); err != nil {
		t.Fatalf("create run: %v", err)
	}

	if err := appliances.Delete(context.Background(), appliance.ID); !errors.Is(err, ErrHasPendingRuns) {
		t.Fatalf("expected ErrHasPendingRuns while a run is pending, got %v", err)
	}

	if err := runs.Cancel(context.Background(), run.ID); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if err := appliances.Delete(context.Background(), appliance.ID); err != nil {
		t.Errorf("expected the delete to succeed once nothing is pending, got %v", err)
	}
}

func TestFindAllReportsProtectedAppliances(t *testing.T) {
	dbx := openTestDB(t)
	all, err := NewApplianceRepository(dbx).FindAll(context.Background())
	if err != nil {
		t.Fatalf("find all: %v", err)
	}
	protected := 0
	for _, a := range all {
		if a.Protected {
			protected++
		}
	}
	if protected != 6 {
		t.Errorf("expected the 6 seeded appliances to be protected, got %d", protected)
	}
}
