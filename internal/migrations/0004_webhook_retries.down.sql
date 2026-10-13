DROP INDEX IF EXISTS idx_notifications_scheduled_run_id;
ALTER TABLE scheduled_run DROP COLUMN IF EXISTS next_attempt_at;
