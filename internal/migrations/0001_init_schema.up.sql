CREATE TABLE appliance (
    id              BIGSERIAL PRIMARY KEY,
    name            TEXT NOT NULL UNIQUE,
    duration_hours  DOUBLE PRECISION NOT NULL
);

CREATE TABLE scheduled_run (
    id                          BIGSERIAL PRIMARY KEY,
    appliance_id                BIGINT NOT NULL REFERENCES appliance(id) ON DELETE CASCADE,
    appliance_name              TEXT NOT NULL,
    zip_code                    TEXT NOT NULL,
    start_time                  TIMESTAMPTZ NOT NULL,
    end_time                    TIMESTAMPTZ NOT NULL,
    average_carbon_intensity    DOUBLE PRECISION NOT NULL,
    webhook_url                 TEXT,
    status                      TEXT NOT NULL DEFAULT 'pending'
                                    CHECK (status IN ('pending', 'fired', 'failed', 'cancelled')),
    created_at                  TIMESTAMPTZ NOT NULL DEFAULT now(),
    fired_at                    TIMESTAMPTZ,
    attempt_count                INT NOT NULL DEFAULT 0,
    last_error                  TEXT
);

-- The scheduler engine's poll loop selects pending rows whose start_time is due;
-- this partial index keeps that query fast as the table grows.
CREATE INDEX idx_scheduled_run_pending_due ON scheduled_run (start_time) WHERE status = 'pending';

CREATE TABLE notifications (
    id                  BIGSERIAL PRIMARY KEY,
    scheduled_run_id    BIGINT NOT NULL REFERENCES scheduled_run(id) ON DELETE CASCADE,
    message             TEXT NOT NULL,
    delivered_webhook    BOOLEAN NOT NULL DEFAULT false,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);
