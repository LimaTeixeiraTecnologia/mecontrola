CREATE TABLE outbox_events (
    id              UUID        NOT NULL,
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
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT outbox_events_pkey PRIMARY KEY (id),
    CONSTRAINT outbox_events_status_check CHECK (status IN (1, 2, 3, 4)),
    CONSTRAINT outbox_events_attempts_check CHECK (attempts >= 0),
    CONSTRAINT outbox_events_max_attempts_check CHECK (max_attempts > 0),
    CONSTRAINT outbox_events_attempts_max_check CHECK (attempts <= max_attempts),
    CONSTRAINT outbox_events_published_status_check
        CHECK ((status = 3) = (published_at IS NOT NULL))
);

COMMENT ON COLUMN outbox_events.status IS '1=Pending, 2=Processing, 3=Published, 4=Failed';

CREATE INDEX outbox_events_dispatcher_pending_idx
    ON outbox_events (next_attempt_at)
    WHERE status = 1;

CREATE INDEX outbox_events_reaper_processing_idx
    ON outbox_events (locked_at)
    WHERE status = 2;

CREATE INDEX outbox_events_housekeeping_published_idx
    ON outbox_events (published_at)
    WHERE status = 3;

CREATE INDEX outbox_events_aggregate_idx
    ON outbox_events (aggregate_type, aggregate_id);
