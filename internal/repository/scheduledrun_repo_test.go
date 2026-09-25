package repository

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Drashya9/GreenCycle-Carbon-Aware-Scheduling-API/internal/models"
)

func newTestRun(applianceID int64, name string, start, end time.Time) *models.ScheduledRun {
	return &models.ScheduledRun{
		ApplianceID:            applianceID,
		ApplianceName:          name,
		ZipCode:                "85281",
		StartTime:              start,
		EndTime:                end,
		AverageCarbonIntensity: 123.4,
	}
}

func TestScheduledRunRepository_CreateAndFindByID(t *testing.T) {
	dbx := openTestDB(t)
	appliance := createTestAppliance(t, dbx)
	repo := NewScheduledRunRepository(dbx)

	run := newTestRun(appliance.ID, appliance.Name, time.Now().Add(time.Hour), time.Now().Add(2*time.Hour))
	if err := repo.Create(context.Background(), run); err != nil {
		t.Fatalf("create: %v", err)
	}
	defer dbx.Exec(`DELETE FROM scheduled_run WHERE id = $1`, run.ID)

	if run.ID == 0 {
		t.Fatal("expected a non-zero id after create")
	}
	if run.Status != models.StatusPending {
		t.Errorf("expected status pending, got %q", run.Status)
	}

	found, err := repo.FindByID(context.Background(), run.ID)
	if err != nil {
		t.Fatalf("find by id: %v", err)
	}
	if found.ZipCode != "85281" {
		t.Errorf("expected zip 85281, got %q", found.ZipCode)
	}
}

func TestScheduledRunRepository_Cancel_OnlyWhilePending(t *testing.T) {
	dbx := openTestDB(t)
	appliance := createTestAppliance(t, dbx)
	repo := NewScheduledRunRepository(dbx)

	run := newTestRun(appliance.ID, appliance.Name, time.Now().Add(time.Hour), time.Now().Add(2*time.Hour))
	if err := repo.Create(context.Background(), run); err != nil {
		t.Fatalf("create: %v", err)
	}
	defer dbx.Exec(`DELETE FROM scheduled_run WHERE id = $1`, run.ID)

	if err := repo.Cancel(context.Background(), run.ID); err != nil {
		t.Fatalf("expected cancel of a pending run to succeed, got: %v", err)
	}

	// Same compare-and-swap shape as the claim below: cancelling an already-cancelled
	// run must fail, not silently succeed twice.
	if err := repo.Cancel(context.Background(), run.ID); err != ErrNotFound {
		t.Errorf("expected ErrNotFound cancelling an already-cancelled run, got: %v", err)
	}
}

func TestScheduledRunRepository_FindDuePending(t *testing.T) {
	dbx := openTestDB(t)
	appliance := createTestAppliance(t, dbx)
	repo := NewScheduledRunRepository(dbx)

	due := newTestRun(appliance.ID, appliance.Name, time.Now().Add(-time.Minute), time.Now())
	notDue := newTestRun(appliance.ID, appliance.Name, time.Now().Add(time.Hour), time.Now().Add(2*time.Hour))
	for _, r := range []*models.ScheduledRun{due, notDue} {
		if err := repo.Create(context.Background(), r); err != nil {
			t.Fatalf("create: %v", err)
		}
		id := r.ID
		defer dbx.Exec(`DELETE FROM scheduled_run WHERE id = $1`, id)
	}

	results, err := repo.FindDuePending(context.Background(), time.Now(), 50)
	if err != nil {
		t.Fatalf("find due pending: %v", err)
	}

	var foundDue, foundNotDue bool
	for _, r := range results {
		if r.ID == due.ID {
			foundDue = true
		}
		if r.ID == notDue.ID {
			foundNotDue = true
		}
	}
	if !foundDue {
		t.Error("expected the overdue run to be returned")
	}
	if foundNotDue {
		t.Error("expected the future run NOT to be returned")
	}
}

// TestScheduledRunRepository_ClaimForFiring_AtomicUnderConcurrency is the test that actually
// closes the coverage gap: internal/scheduler/engine_test.go proves the claim logic is
// exactly-once against a FAKE repo. This proves the real SQL — UPDATE ... WHERE
// status='pending' RETURNING * — gives that same guarantee against a REAL Postgres
// connection pool under genuine concurrent access, not simulated concurrency.
func TestScheduledRunRepository_ClaimForFiring_AtomicUnderConcurrency(t *testing.T) {
	dbx := openTestDB(t)
	appliance := createTestAppliance(t, dbx)
	repo := NewScheduledRunRepository(dbx)

	run := newTestRun(appliance.ID, appliance.Name, time.Now().Add(-time.Minute), time.Now())
	if err := repo.Create(context.Background(), run); err != nil {
		t.Fatalf("create: %v", err)
	}
	defer dbx.Exec(`DELETE FROM scheduled_run WHERE id = $1`, run.ID)

	const attempts = 20
	var successes int32
	var wg sync.WaitGroup
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := repo.ClaimForFiring(context.Background(), run.ID, time.Now()); err == nil {
				atomic.AddInt32(&successes, 1)
			}
		}()
	}
	wg.Wait()

	if successes != 1 {
		t.Errorf("expected exactly 1 successful claim out of %d concurrent attempts against real Postgres, got %d", attempts, successes)
	}

	final, err := repo.FindByID(context.Background(), run.ID)
	if err != nil {
		t.Fatalf("find by id after claims: %v", err)
	}
	if final.Status != models.StatusFired {
		t.Errorf("expected final status 'fired', got %q", final.Status)
	}
	if final.AttemptCount != 1 {
		t.Errorf("expected attempt_count to be exactly 1 (one successful claim), got %d", final.AttemptCount)
	}
}
