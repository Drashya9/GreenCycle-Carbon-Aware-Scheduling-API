package scheduler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Drashya9/GreenCycle-Carbon-Aware-Scheduling-API/internal/models"
)

// TestUserWebhookCannotReachInternalAddresses covers a run whose stored webhook URL points at
// the server's own network (e.g. one saved before validation existed). The engine's
// public-only client must refuse it, and the run must be recorded as failed, not delivered.
func TestUserWebhookCannotReachInternalAddresses(t *testing.T) {
	reached := false
	internal := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { reached = true }))
	defer internal.Close()

	run := models.ScheduledRun{
		ID: 20, ApplianceName: "Dryer", ZipCode: "85281",
		StartTime: time.Now().Add(-time.Minute), EndTime: time.Now(),
		Status:     models.StatusPending,
		WebhookURL: models.NewNull(internal.URL), // 127.0.0.1
	}
	repo := newFakeRepo(run)
	e := NewEngine(repo, "", time.Minute, time.Second) // real clients, no fakes

	e.RunOnce(context.Background())

	if reached {
		t.Fatal("the scheduler POSTed to a loopback address")
	}
	repo.mu.Lock()
	got := repo.runs[20]
	repo.mu.Unlock()
	if got.Status != models.StatusFailed || !strings.Contains(got.LastError.V, "non-public address") {
		t.Errorf("expected status failed with a non-public-address error, got %q / %q", got.Status, got.LastError.V)
	}
	if len(repo.notifications) != 1 || repo.notifications[0].DeliveredWebhook {
		t.Errorf("expected one undelivered notification, got %+v", repo.notifications)
	}
}
