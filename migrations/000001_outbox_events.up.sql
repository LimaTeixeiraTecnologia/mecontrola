CREATE TABLE outbox_events (
    id              UUID PRIMARY KEY,
    event_type      TEXT NOT NULL,
    aggregate_type  TEXT NOT NULL,
    aggregate_id    TEXT NOT NULL,
    payload         JSONB NOT NULL,
    metadata        JSONB NOT NULL DEFAULT '{}'::jsonb,
    status          SMALLINT NOT NULL,
    attempts        INT NOT NULL DEFAULT 0,
    max_attempts    INT NOT NULL,
    next_attempt_at TIMESTAMPTZ NOT NULL,
    last_error      TEXT,
    locked_at       TIMESTAMPTZ,
    locked_by       TEXT,
    published_at    TIMESTAMPTZ,
    occurred_at     TIMESTAMPTZ NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

COMMENT ON COLUMN outbox_events.status IS '1=Pending, 2=Processing, 3=Published, 4=Failed';

CREATE INDEX idx_outbox_dispatcher_pending
    ON outbox_events (next_attempt_at)
 WHERE status = 1;

CREATE INDEX idx_outbox_reaper_processing
    ON outbox_events (locked_at)
 WHERE status = 2;

CREATE INDEX idx_outbox_housekeeping_published
    ON outbox_events (published_at)
 WHERE status = 3;

CREATE INDEX idx_outbox_aggregate
    ON outbox_events (aggregate_type, aggregate_id);
