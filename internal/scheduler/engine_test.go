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

// fakeRepo mimics the real ScheduledRunRepository's compare-and-swap claim semantics
// with an in-memory map, so the engine's concurrency guarantees can be tested without a
// live Postgres.
type fakeRepo struct {
	mu            sync.Mutex
	runs          map[int64]*models.ScheduledRun
	claimAttempts int32
	notifications []models.Notification
}

func newFakeRepo(runs ...models.ScheduledRun) *fakeRepo {
	m := make(map[int64]*models.ScheduledRun, len(runs))
	for i := range runs {
		r := runs[i]
		m[r.ID] = &r
	}
	return &fakeRepo{runs: m}
}

func (f *fakeRepo) FindDuePending(ctx context.Context, now time.Time, limit int) ([]models.ScheduledRun, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []models.ScheduledRun
	for _, r := range f.runs {
		if r.Status == models.StatusPending && !r.StartTime.After(now) {
			out = append(out, *r)
		}
	}
	return out, nil
}

func (f *fakeRepo) ClaimForFiring(ctx context.Context, id int64, firedAt time.Time) (*models.ScheduledRun, error) {
	atomic.AddInt32(&f.claimAttempts, 1)
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.runs[id]
	if !ok || r.Status != models.StatusPending {
		return nil, repository.ErrNotFound
	}
	r.Status = models.StatusFired
	r.FiredAt.V = firedAt
	r.FiredAt.Valid = true
	claimed := *r
	return &claimed, nil
}

func (f *fakeRepo) MarkFailed(ctx context.Context, id int64, lastError string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r, ok := f.runs[id]; ok {
		r.Status = models.StatusFailed
		r.LastError.V = lastError
		r.LastError.Valid = true
	}
	return nil
}

func (f *fakeRepo) CreateNotification(ctx context.Context, n *models.Notification) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.notifications = append(f.notifications, *n)
	return nil
}

// fakeSender always succeeds without making a real network call.
type fakeSender struct{ calls int32 }

func (f *fakeSender) Do(req *http.Request) (*http.Response, error) {
	atomic.AddInt32(&f.calls, 1)
	return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(""))}, nil
}

// failingSender always errors, to exercise the MarkFailed path.
type failingSender struct{}

func (failingSender) Do(req *http.Request) (*http.Response, error) {
	return nil, errors.New("connection refused")
}

func TestClaimIsAtomicUnderConcurrentTicks(t *testing.T) {
	due := models.ScheduledRun{
		ID: 1, ApplianceName: "Dryer", ZipCode: "85281",
		StartTime: time.Now().Add(-time.Minute), EndTime: time.Now(),
		Status: models.StatusPending,
	}
	repo := newFakeRepo(due)
	sender := &fakeSender{}

	e := NewEngine(repo, "https://example.invalid/webhook", 15*time.Second, time.Second)
	e.client = sender

	// Simulate two poll ticks racing on the exact same due row, as could happen if a
	// tick runs long and overlaps the next one.
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			e.tick(context.Background())
		}()
	}
	wg.Wait()
	e.wg.Wait() // drain the fire goroutines each tick spawned

	if got := atomic.LoadInt32(&sender.calls); got != 1 {
		t.Errorf("expected exactly 1 webhook delivery despite 5 concurrent ticks, got %d", got)
	}
	if len(repo.notifications) != 1 {
		t.Errorf("expected exactly 1 notification, got %d", len(repo.notifications))
	}
}

func TestStartupCatchUpFiresOverdueRunImmediately(t *testing.T) {
	overdue := models.ScheduledRun{
		ID: 2, ApplianceName: "EV Charger", ZipCode: "85281",
		StartTime: time.Now().Add(-1 * time.Hour), // the process was "down" through this
		EndTime:   time.Now(),
		Status:    models.StatusPending,
	}
	repo := newFakeRepo(overdue)
	sender := &fakeSender{}

	e := NewEngine(repo, "https://example.invalid/webhook", time.Minute, time.Second)
	e.client = sender

	// Run's first action (before the ticker starts) is a catch-up tick — exercise it
	// directly rather than waiting on a real ticker.
	e.tick(context.Background())
	e.wg.Wait()

	if got := atomic.LoadInt32(&sender.calls); got != 1 {
		t.Fatalf("expected the overdue run to fire immediately, got %d webhook calls", got)
	}
	if len(repo.notifications) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(repo.notifications))
	}
	if !strings.Contains(repo.notifications[0].Message, "fired late") {
		t.Errorf("expected notification to flag the run as fired late, got: %q", repo.notifications[0].Message)
	}
}

func TestOnTimeFireIsNotFlaggedLate(t *testing.T) {
	onTime := models.ScheduledRun{
		ID: 3, ApplianceName: "Washing Machine", ZipCode: "85281",
		StartTime: time.Now().Add(-time.Second),
		EndTime:   time.Now().Add(time.Hour),
		Status:    models.StatusPending,
	}
	repo := newFakeRepo(onTime)
	sender := &fakeSender{}

	e := NewEngine(repo, "https://example.invalid/webhook", time.Minute, time.Second)
	e.client = sender

	e.tick(context.Background())
	e.wg.Wait()

	if strings.Contains(repo.notifications[0].Message, "fired late") {
		t.Errorf("run fired within one poll interval of start time should not be flagged late, got: %q",
			repo.notifications[0].Message)
	}
}

func TestWebhookFailureMarksRunFailedButStillNotifies(t *testing.T) {
	due := models.ScheduledRun{
		ID: 4, ApplianceName: "Pool Pump", ZipCode: "85281",
		StartTime: time.Now().Add(-time.Minute), EndTime: time.Now(),
		Status: models.StatusPending,
	}
	repo := newFakeRepo(due)

	e := NewEngine(repo, "https://example.invalid/webhook", time.Minute, time.Second)
	e.client = failingSender{}

	e.tick(context.Background())
	e.wg.Wait()

	repo.mu.Lock()
	status := repo.runs[4].Status
	repo.mu.Unlock()

	if status != models.StatusFailed {
		t.Errorf("expected status 'failed' after webhook error, got %q", status)
	}
	if len(repo.notifications) != 1 {
		t.Fatalf("expected a notification even on webhook failure, got %d", len(repo.notifications))
	}
	if repo.notifications[0].DeliveredWebhook {
		t.Errorf("notification should record delivery failure, got DeliveredWebhook=true")
	}
}
