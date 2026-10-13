package scheduler

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Drashya9/GreenCycle-Carbon-Aware-Scheduling-API/internal/models"
	"github.com/Drashya9/GreenCycle-Carbon-Aware-Scheduling-API/internal/repository"
)

// MarkRetry completes engine_test.go's fakeRepo for the runRepository interface.
func (f *fakeRepo) MarkRetry(ctx context.Context, id int64, lastError string, nextAttempt time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r, ok := f.runs[id]; ok {
		r.Status = models.StatusPending
		r.LastError = models.NewNull(lastError)
		r.NextAttemptAt = models.NewNull(nextAttempt)
	}
	return nil
}

// retryRepo mirrors the real repository's retry semantics: the claim increments
// attempt_count, and a run waiting on its backoff isn't due until NextAttemptAt passes.
type retryRepo struct {
	mu            sync.Mutex
	run           models.ScheduledRun
	notifications []models.Notification
}

func (r *retryRepo) FindDuePending(_ context.Context, now time.Time, _ int) ([]models.ScheduledRun, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	backoffOver := !r.run.NextAttemptAt.Valid || !r.run.NextAttemptAt.V.After(now)
	if r.run.Status == models.StatusPending && !r.run.StartTime.After(now) && backoffOver {
		return []models.ScheduledRun{r.run}, nil
	}
	return nil, nil
}

func (r *retryRepo) ClaimForFiring(_ context.Context, _ int64, firedAt time.Time) (*models.ScheduledRun, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.run.Status != models.StatusPending {
		return nil, repository.ErrNotFound
	}
	r.run.Status = models.StatusFired
	r.run.FiredAt = models.NewNull(firedAt)
	r.run.AttemptCount++
	claimed := r.run
	return &claimed, nil
}

func (r *retryRepo) MarkRetry(_ context.Context, _ int64, lastError string, nextAttempt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.run.Status = models.StatusPending
	r.run.LastError = models.NewNull(lastError)
	r.run.NextAttemptAt = models.NewNull(nextAttempt)
	return nil
}

func (r *retryRepo) MarkFailed(_ context.Context, _ int64, lastError string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.run.Status = models.StatusFailed
	r.run.LastError = models.NewNull(lastError)
	return nil
}

func (r *retryRepo) CreateNotification(_ context.Context, n *models.Notification) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.notifications = append(r.notifications, *n)
	return nil
}

// skipBackoff makes a run waiting to retry due now, standing in for time passing.
func (r *retryRepo) skipBackoff() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.run.NextAttemptAt = models.NewNull(time.Now().Add(-time.Second))
}

// flakySender fails its first `failures` calls, then succeeds.
type flakySender struct {
	failures int32
	calls    int32
}

func (f *flakySender) Do(*http.Request) (*http.Response, error) {
	if atomic.AddInt32(&f.calls, 1) <= f.failures {
		return nil, errors.New("connection refused")
	}
	return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(""))}, nil
}

func newRetryRepo() *retryRepo {
	return &retryRepo{run: models.ScheduledRun{
		ID: 30, ApplianceName: "Dryer", ZipCode: "85281",
		StartTime: time.Now().Add(-time.Minute), EndTime: time.Now().Add(time.Hour),
		Status: models.StatusPending,
	}}
}

func TestFailedWebhookIsRetriedThenDelivered(t *testing.T) {
	repo := newRetryRepo()
	sender := &flakySender{failures: 1}
	e := NewEngine(repo, "https://example.invalid/webhook", time.Minute, time.Second)
	e.client = sender
	e.SetMaxAttempts(3)

	e.RunOnce(context.Background())
	if repo.run.Status != models.StatusPending || !repo.run.LastError.Valid || !repo.run.NextAttemptAt.Valid {
		t.Fatalf("after a failed attempt: expected pending with lastError and nextAttemptAt, got %+v", repo.run)
	}
	if wait := time.Until(repo.run.NextAttemptAt.V); wait < 50*time.Second || wait > 70*time.Second {
		t.Errorf("expected a ~1m backoff before attempt 2, got %v", wait)
	}
	if len(repo.notifications) != 0 {
		t.Errorf("expected no notification before a final outcome, got %d", len(repo.notifications))
	}

	if n := e.RunOnce(context.Background()); n != 0 {
		t.Errorf("expected the run to wait out its backoff, but %d run(s) were due", n)
	}

	repo.skipBackoff()
	e.RunOnce(context.Background())

	if repo.run.Status != models.StatusFired || repo.run.AttemptCount != 2 {
		t.Errorf("expected fired on attempt 2, got status %q after %d attempts", repo.run.Status, repo.run.AttemptCount)
	}
	if len(repo.notifications) != 1 || !repo.notifications[0].DeliveredWebhook {
		t.Fatalf("expected exactly one delivered notification, got %+v", repo.notifications)
	}
	if !strings.Contains(repo.notifications[0].Message, "attempt 2") {
		t.Errorf("expected the notification to mention attempt 2, got %q", repo.notifications[0].Message)
	}
}

func TestRetriesStopAtMaxAttempts(t *testing.T) {
	repo := newRetryRepo()
	sender := &flakySender{failures: 100}
	e := NewEngine(repo, "https://example.invalid/webhook", time.Minute, time.Second)
	e.client = sender
	e.SetMaxAttempts(2)

	e.RunOnce(context.Background())
	repo.skipBackoff()
	e.RunOnce(context.Background())
	repo.skipBackoff()
	e.RunOnce(context.Background()) // nothing left to do: the run is terminal

	if got := atomic.LoadInt32(&sender.calls); got != 2 {
		t.Errorf("expected exactly 2 delivery attempts, got %d", got)
	}
	if repo.run.Status != models.StatusFailed {
		t.Errorf("expected failed after exhausting attempts, got %q", repo.run.Status)
	}
	if len(repo.notifications) != 1 || repo.notifications[0].DeliveredWebhook {
		t.Errorf("expected one undelivered notification, got %+v", repo.notifications)
	}
}

func TestRetryDelayBacksOffExponentiallyWithCap(t *testing.T) {
	for attempt, want := range map[int]time.Duration{
		1: time.Minute, 2: 2 * time.Minute, 3: 4 * time.Minute, 6: 32 * time.Minute,
		7: time.Hour, 100: time.Hour,
	} {
		if got := retryDelay(attempt); got != want {
			t.Errorf("retryDelay(%d) = %v, want %v", attempt, got, want)
		}
	}
}
