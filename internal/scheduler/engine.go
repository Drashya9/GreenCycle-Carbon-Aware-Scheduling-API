// Package scheduler runs a background loop that fires a webhook + notification the
// moment a scheduled appliance run's window starts.
package scheduler

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/Drashya9/GreenCycle-Carbon-Aware-Scheduling-API/internal/models"
	"github.com/Drashya9/GreenCycle-Carbon-Aware-Scheduling-API/internal/netguard"
)

// runRepository is the slice of ScheduledRunRepository the engine depends on — kept as
// an interface so tests can exercise the claim/catch-up logic without a live Postgres.
type runRepository interface {
	FindDuePending(ctx context.Context, now time.Time, limit int) ([]models.ScheduledRun, error)
	ClaimForFiring(ctx context.Context, id int64, firedAt time.Time) (*models.ScheduledRun, error)
	MarkRetry(ctx context.Context, id int64, lastError string, nextAttempt time.Time) error
	MarkFailed(ctx context.Context, id int64, lastError string) error
	CreateNotification(ctx context.Context, n *models.Notification) error
}

// batchLimit bounds how many due runs a single poll tick claims, so one slow tick can't
// starve the next.
const batchLimit = 50

type Engine struct {
	repo runRepository
	// client sends to the operator-configured default webhook; userClient sends to webhook
	// URLs supplied per schedule, which are untrusted and so restricted to public addresses.
	client            webhookSender
	userClient        webhookSender
	pollInterval      time.Duration
	defaultWebhookURL string
	webhookTimeout    time.Duration
	// maxAttempts is the delivery budget per run, counting the first attempt.
	maxAttempts int

	wg sync.WaitGroup
}

// NewEngine makes a single delivery attempt per run; call SetMaxAttempts to allow retries.
func NewEngine(repo runRepository, defaultWebhookURL string, pollInterval, webhookTimeout time.Duration) *Engine {
	return &Engine{
		repo:              repo,
		client:            &http.Client{CheckRedirect: netguard.NoRedirects},
		userClient:        netguard.PublicOnlyClient(),
		pollInterval:      pollInterval,
		defaultWebhookURL: defaultWebhookURL,
		webhookTimeout:    webhookTimeout,
		maxAttempts:       1,
	}
}

// SetMaxAttempts sets how many times a webhook is attempted before the run is marked
// failed. Values below 1 are ignored.
func (e *Engine) SetMaxAttempts(n int) {
	if n >= 1 {
		e.maxAttempts = n
	}
}

// retryDelay is the backoff before the next attempt after `attempt` failed ones:
// 1m, 2m, 4m, … capped at an hour.
func retryDelay(attempt int) time.Duration {
	const maxDelay = time.Hour
	if attempt >= 7 { // 2^6 minutes already exceeds the cap; also avoids shift overflow
		return maxDelay
	}
	return min(time.Minute<<(attempt-1), maxDelay)
}

// Run blocks until ctx is cancelled. It fires an immediate catch-up pass on startup —
// covering any run whose window started while the process was down — then polls on a
// ticker. On shutdown it stops the ticker and waits for the in-flight tick to finish
// before returning, so a fire in progress isn't abandoned mid-webhook.
func (e *Engine) Run(ctx context.Context) {
	e.tick(ctx) // startup catch-up pass

	ticker := time.NewTicker(e.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			e.wg.Wait()
			return
		case <-ticker.C:
			e.tick(ctx)
		}
	}
}

func (e *Engine) tick(ctx context.Context) {
	e.dispatch(ctx, &e.wg)
}

// RunOnce does a single poll-and-fire pass and waits for every fire it started before
// returning the number of due runs it picked up. It exists for hosts where a background
// ticker can't be relied on (a cron-invoked endpoint). It uses its own WaitGroup so
// overlapping calls can't interfere with each other or with Run's shutdown wait.
func (e *Engine) RunOnce(ctx context.Context) int {
	var wg sync.WaitGroup
	n := e.dispatch(ctx, &wg)
	wg.Wait()
	return n
}

func (e *Engine) dispatch(ctx context.Context, wg *sync.WaitGroup) int {
	now := time.Now()
	due, err := e.repo.FindDuePending(ctx, now, batchLimit)
	if err != nil {
		log.Printf("scheduler: find due runs: %v", err)
		return 0
	}

	for _, run := range due {
		wg.Add(1)
		go func(run models.ScheduledRun) {
			defer wg.Done()
			e.fire(ctx, run, now)
		}(run)
	}
	return len(due)
}

func (e *Engine) fire(ctx context.Context, run models.ScheduledRun, tickTime time.Time) {
	claimed, err := e.repo.ClaimForFiring(ctx, run.ID, tickTime)
	if err != nil {
		// ErrNotFound just means another tick (or another instance) already claimed it —
		// the atomic compare-and-swap in the claim is what makes that safe to ignore.
		return
	}

	// The claim increments attempt_count, so this is the attempt being made now.
	attempt := max(claimed.AttemptCount, 1)

	// More than one poll interval late suggests the process was down or stalled through
	// the scheduled start time, not just normal poll-cycle granularity.
	firedLate := tickTime.Sub(claimed.StartTime) > e.pollInterval

	message := formatMessage(*claimed, firedLate, attempt)

	webhookURL, client := e.defaultWebhookURL, e.client
	if claimed.WebhookURL.Valid && claimed.WebhookURL.V != "" {
		webhookURL, client = claimed.WebhookURL.V, e.userClient
	}

	delivered := false
	if webhookURL != "" {
		payload := WebhookPayload{
			ScheduledRunID: claimed.ID,
			ApplianceName:  claimed.ApplianceName,
			ZipCode:        claimed.ZipCode,
			StartTime:      claimed.StartTime,
			EndTime:        claimed.EndTime,
			FiredLate:      firedLate,
			Attempt:        attempt,
		}
		if err := dispatchWebhook(ctx, client, webhookURL, payload, e.webhookTimeout); err != nil {
			if attempt < e.maxAttempts {
				next := time.Now().Add(retryDelay(attempt))
				log.Printf("scheduler: webhook attempt %d/%d failed for run %d, retrying at %s: %v",
					attempt, e.maxAttempts, claimed.ID, next.Format(time.RFC3339), err)
				if markErr := e.repo.MarkRetry(ctx, claimed.ID, err.Error(), next); markErr != nil {
					log.Printf("scheduler: schedule retry for run %d: %v", claimed.ID, markErr)
				}
				return // notify once the run reaches a final outcome, not on every attempt
			}
			log.Printf("scheduler: webhook delivery failed for run %d after %d attempt(s): %v", claimed.ID, attempt, err)
			if markErr := e.repo.MarkFailed(ctx, claimed.ID, err.Error()); markErr != nil {
				log.Printf("scheduler: mark run %d failed: %v", claimed.ID, markErr)
			}
		} else {
			delivered = true
		}
	}

	if err := e.repo.CreateNotification(ctx, &models.Notification{
		ScheduledRunID:   claimed.ID,
		Message:          message,
		DeliveredWebhook: delivered,
	}); err != nil {
		log.Printf("scheduler: create notification for run %d: %v", claimed.ID, err)
	}
}

func formatMessage(run models.ScheduledRun, firedLate bool, attempt int) string {
	suffix := ""
	switch {
	case attempt > 1:
		suffix = fmt.Sprintf(" (attempt %d — earlier deliveries failed or were interrupted)", attempt)
	case firedLate:
		suffix = " (fired late — process may have been down at start time)"
	}
	return run.ApplianceName + " window started" + suffix
}
