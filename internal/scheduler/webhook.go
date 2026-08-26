package scheduler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// WebhookPayload is POSTed to the configured webhook URL when a scheduled run fires.
type WebhookPayload struct {
	ScheduledRunID int64     `json:"scheduledRunId"`
	ApplianceName  string    `json:"applianceName"`
	ZipCode        string    `json:"zip"`
	StartTime      time.Time `json:"startTime"`
	EndTime        time.Time `json:"endTime"`
	// FiredLate is true when the process was down at StartTime and this fired during
	// the engine's startup catch-up pass instead of exactly on time.
	FiredLate bool `json:"firedLate"`
}

// webhookSender is the subset of *http.Client the dispatcher needs — narrow enough to
// fake in tests without spinning up a real HTTP server.
type webhookSender interface {
	Do(req *http.Request) (*http.Response, error)
}

func dispatchWebhook(ctx context.Context, client webhookSender, url string, payload WebhookPayload, timeout time.Duration) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal webhook payload: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build webhook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("send webhook: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}
	return nil
}
