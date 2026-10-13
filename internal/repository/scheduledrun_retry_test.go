package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/Drashya9/GreenCycle-Carbon-Aware-Scheduling-API/internal/models"
)

// These run against a real Postgres (skipped without one; CI provides it) and pass explicit
// "now" values to the queries, which is how they simulate backoffs and leases expiring.

func createDueRun(t *testing.T, dbx *sqlx.DB, repo *ScheduledRunRepository, now time.Time) *models.ScheduledRun {
	t.Helper()
	appliance := createTestAppliance(t, dbx)
	run := newTestRun(appliance.ID, appliance.Name, now.Add(-time.Minute), now.Add(time.Hour))
	if err := repo.Create(context.Background(), run); err != nil {
		t.Fatalf("create run: %v", err)
	}
	t.Cleanup(func() { dbx.Exec(`DELETE FROM scheduled_run WHERE id = $1`, run.ID) })
	return run
}

func isDue(t *testing.T, repo *ScheduledRunRepository, id int64, at time.Time) bool {
	t.Helper()
	runs, err := repo.FindDuePending(context.Background(), at, 1000)
	if err != nil {
		t.Fatalf("find due: %v", err)
	}
	for _, r := range runs {
		if r.ID == id {
			return true
		}
	}
	return false
}

func TestMarkRetryDefersTheRunUntilItsBackoffPasses(t *testing.T) {
	dbx := openTestDB(t)
	repo := NewScheduledRunRepository(dbx)
	ctx, now := context.Background(), time.Now()
	run := createDueRun(t, dbx, repo, now)

	if _, err := repo.ClaimForFiring(ctx, run.ID, now); err != nil {
		t.Fatalf("first claim: %v", err)
	}
	if err := repo.MarkRetry(ctx, run.ID, "connection refused", now.Add(10*time.Minute)); err != nil {
		t.Fatalf("mark retry: %v", err)
	}

	if isDue(t, repo, run.ID, now.Add(time.Minute)) {
		t.Error("run was due again before its backoff passed")
	}
	if _, err := repo.ClaimForFiring(ctx, run.ID, now.Add(time.Minute)); !errors.Is(err, ErrNotFound) {
		t.Errorf("expected the claim to be refused during backoff, got %v", err)
	}

	later := now.Add(11 * time.Minute)
	if !isDue(t, repo, run.ID, later) {
		t.Fatal("run was not due once its backoff passed")
	}
	claimed, err := repo.ClaimForFiring(ctx, run.ID, later)
	if err != nil {
		t.Fatalf("second claim: %v", err)
	}
	if claimed.AttemptCount != 2 || claimed.LastError.V != "connection refused" {
		t.Errorf("expected attempt 2 carrying the previous error, got attempt %d / %q", claimed.AttemptCount, claimed.LastError.V)
	}
}

func TestAbandonedClaimIsReclaimedAfterItsLease(t *testing.T) {
	dbx := openTestDB(t)
	repo := NewScheduledRunRepository(dbx)
	ctx, now := context.Background(), time.Now()
	run := createDueRun(t, dbx, repo, now)

	// Claimed, but the process "died" before delivering or writing a notification.
	if _, err := repo.ClaimForFiring(ctx, run.ID, now); err != nil {
		t.Fatalf("claim: %v", err)
	}

	if isDue(t, repo, run.ID, now.Add(time.Minute)) {
		t.Error("a fresh claim must not be reclaimable before its lease expires")
	}

	stale := now.Add(claimLease + time.Minute)
	if !isDue(t, repo, run.ID, stale) {
		t.Fatal("an abandoned claim should be due again once its lease expires")
	}
	reclaimed, err := repo.ClaimForFiring(ctx, run.ID, stale)
	if err != nil {
		t.Fatalf("reclaim: %v", err)
	}
	if reclaimed.AttemptCount != 2 {
		t.Errorf("expected the reclaim to count as attempt 2, got %d", reclaimed.AttemptCount)
	}

	// The reclaim refreshed fired_at, so a racing second reclaimer must lose.
	if _, err := repo.ClaimForFiring(ctx, run.ID, stale); !errors.Is(err, ErrNotFound) {
		t.Errorf("expected a second reclaim at the same instant to be refused, got %v", err)
	}
}

func TestNotifiedRunIsNeverReclaimed(t *testing.T) {
	dbx := openTestDB(t)
	repo := NewScheduledRunRepository(dbx)
	ctx, now := context.Background(), time.Now()
	run := createDueRun(t, dbx, repo, now)

	if _, err := repo.ClaimForFiring(ctx, run.ID, now); err != nil {
		t.Fatalf("claim: %v", err)
	}
	if err := repo.CreateNotification(ctx, &models.Notification{ScheduledRunID: run.ID, Message: "done", DeliveredWebhook: true}); err != nil {
		t.Fatalf("notify: %v", err)
	}

	stale := now.Add(claimLease + time.Minute)
	if isDue(t, repo, run.ID, stale) {
		t.Error("a run that reached a final outcome must never be due again")
	}
	if _, err := repo.ClaimForFiring(ctx, run.ID, stale); !errors.Is(err, ErrNotFound) {
		t.Errorf("expected the claim to be refused, got %v", err)
	}
}
