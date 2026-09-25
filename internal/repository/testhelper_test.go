package repository

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/Drashya9/GreenCycle-Carbon-Aware-Scheduling-API/internal/db"
	"github.com/Drashya9/GreenCycle-Carbon-Aware-Scheduling-API/internal/models"
)

// openTestDB connects to a real Postgres and migrates it (idempotent, same path the app
// takes at boot), so these tests exercise the actual SQL — not a fake — against a real
// database. Skipped automatically if no Postgres is reachable, so it never blocks CI or a
// machine without one running.
func openTestDB(t *testing.T) *sqlx.DB {
	t.Helper()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://greencycle:greencycle@localhost:5432/greencycle?sslmode=disable"
	}

	dbx, err := db.Connect(dbURL)
	if err != nil {
		t.Skipf("postgres not reachable at %s, skipping: %v", dbURL, err)
	}
	if err := db.Migrate(dbx); err != nil {
		t.Skipf("could not migrate test database, skipping: %v", err)
	}

	t.Cleanup(func() { dbx.Close() })
	return dbx
}

// createTestAppliance inserts a uniquely-named appliance (scheduled_run.appliance_id has a
// foreign key to this table) and cleans it up when the test finishes.
func createTestAppliance(t *testing.T, dbx *sqlx.DB) *models.Appliance {
	t.Helper()

	repo := NewApplianceRepository(dbx)
	a := &models.Appliance{
		Name:          fmt.Sprintf("test-appliance-%d", time.Now().UnixNano()),
		DurationHours: 1.0,
	}
	if err := repo.Create(context.Background(), a); err != nil {
		t.Fatalf("create test appliance: %v", err)
	}
	t.Cleanup(func() { _ = repo.Delete(context.Background(), a.ID) })
	return a
}
