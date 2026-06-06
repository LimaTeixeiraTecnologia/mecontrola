CREATE TABLE billing_processed_events (
    event_key   TEXT PRIMARY KEY,
    trigger     TEXT NOT NULL,
    recurso_id  TEXT NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL,
    applied_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    status      TEXT NOT NULL CHECK (status IN ('applied', 'superseded'))
);

CREATE INDEX billing_processed_events_recurso_idx
    ON billing_processed_events (recurso_id);
