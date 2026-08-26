// Package scheduler runs a background loop that fires a webhook + notification the
// moment a scheduled appliance run's window starts.
package scheduler

import (
	"context"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/Drashya9/GreenCycle-Carbon-Aware-Scheduling-API/internal/models"
)

// runRepository is the slice of ScheduledRunRepository the engine depends on — kept as
// an interface so tests can exercise the claim/catch-up logic without a live Postgres.
type runRepository interface {
	FindDuePending(ctx context.Context, now time.Time, limit int) ([]models.ScheduledRun, error)
	ClaimForFiring(ctx context.Context, id int64, firedAt time.Time) (*models.ScheduledRun, error)
	MarkFailed(ctx context.Context, id int64, lastError string) error
	CreateNotification(ctx context.Context, n *models.Notification) error
}

// batchLimit bounds how many due runs a single poll tick claims, so one slow tick can't
// starve the next.
const batchLimit = 50

type Engine struct {
	repo              runRepository
	client            webhookSender
	pollInterval      time.Duration
	defaultWebhookURL string
	webhookTimeout    time.Duration

	wg sync.WaitGroup
}

func NewEngine(repo runRepository, defaultWebhookURL string, pollInterval, webhookTimeout time.Duration) *Engine {
	return &Engine{
		repo:              repo,
		client:            &http.Client{},
		pollInterval:      pollInterval,
		defaultWebhookURL: defaultWebhookURL,
		webhookTimeout:    webhookTimeout,
	}
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
	now := time.Now()
	due, err := e.repo.FindDuePending(ctx, now, batchLimit)
	if err != nil {
		log.Printf("scheduler: find due runs: %v", err)
		return
	}

	for _, run := range due {
		e.wg.Add(1)
		go func(run models.ScheduledRun) {
			defer e.wg.Done()
			e.fire(ctx, run, now)
		}(run)
	}
}

func (e *Engine) fire(ctx context.Context, run models.ScheduledRun, tickTime time.Time) {
	claimed, err := e.repo.ClaimForFiring(ctx, run.ID, tickTime)
	if err != nil {
		// ErrNotFound just means another tick (or another instance) already claimed it —
		// the atomic UPDATE...WHERE status='pending' is what makes that safe to ignore.
		return
	}

	// More than one poll interval late suggests the process was down or stalled through
	// the scheduled start time, not just normal poll-cycle granularity.
	firedLate := tickTime.Sub(claimed.StartTime) > e.pollInterval

	message := formatMessage(*claimed, firedLate)

	webhookURL := e.defaultWebhookURL
	if claimed.WebhookURL.Valid && claimed.WebhookURL.V != "" {
		webhookURL = claimed.WebhookURL.V
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
		}
		if err := dispatchWebhook(ctx, e.client, webhookURL, payload, e.webhookTimeout); err != nil {
			log.Printf("scheduler: webhook delivery failed for run %d: %v", claimed.ID, err)
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

func formatMessage(run models.ScheduledRun, firedLate bool) string {
	suffix := ""
	if firedLate {
		suffix = " (fired late — process may have been down at start time)"
	}
	return run.ApplianceName + " window started" + suffix
}
