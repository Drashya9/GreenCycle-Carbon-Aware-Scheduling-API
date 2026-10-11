package scheduler

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Drashya9/GreenCycle-Carbon-Aware-Scheduling-API/internal/models"
)

func TestRunOnceFiresDueRunsBeforeReturning(t *testing.T) {
	due := models.ScheduledRun{
		ID: 10, ApplianceName: "Dryer", ZipCode: "85281",
		StartTime: time.Now().Add(-time.Minute), EndTime: time.Now(),
		Status: models.StatusPending,
	}
	repo := newFakeRepo(due)
	sender := &fakeSender{}

	e := NewEngine(repo, "https://example.invalid/webhook", time.Minute, time.Second)
	e.client = sender

	if got := e.RunOnce(context.Background()); got != 1 {
		t.Errorf("expected RunOnce to report 1 due run, got %d", got)
	}

	// Deliberately no e.wg.Wait(): RunOnce itself must not return until the fire has
	// finished, since a cron-invoked request ends as soon as it does.
	if got := atomic.LoadInt32(&sender.calls); got != 1 {
		t.Errorf("expected the webhook to have been delivered by the time RunOnce returned, got %d calls", got)
	}
	if len(repo.notifications) != 1 {
		t.Errorf("expected 1 notification, got %d", len(repo.notifications))
	}
}

func TestRunOnceWithNothingDue(t *testing.T) {
	future := models.ScheduledRun{
		ID: 11, ApplianceName: "Dryer", ZipCode: "85281",
		StartTime: time.Now().Add(time.Hour), EndTime: time.Now().Add(2 * time.Hour),
		Status: models.StatusPending,
	}
	repo := newFakeRepo(future)
	sender := &fakeSender{}

	e := NewEngine(repo, "https://example.invalid/webhook", time.Minute, time.Second)
	e.client = sender

	if got := e.RunOnce(context.Background()); got != 0 {
		t.Errorf("expected 0 due runs, got %d", got)
	}
	if got := atomic.LoadInt32(&sender.calls); got != 0 {
		t.Errorf("expected no webhook calls, got %d", got)
	}
}
