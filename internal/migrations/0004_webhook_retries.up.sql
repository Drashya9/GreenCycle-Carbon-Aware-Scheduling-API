-- When a failed webhook delivery may be retried; NULL means "as soon as it's due".
ALTER TABLE scheduled_run ADD COLUMN next_attempt_at TIMESTAMPTZ;

-- The scheduler treats a run claimed as 'fired' with no notification yet as abandoned
-- mid-delivery once its claim is stale, and checks that with NOT EXISTS on this column.
CREATE INDEX idx_notifications_scheduled_run_id ON notifications (scheduled_run_id);
