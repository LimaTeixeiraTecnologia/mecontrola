CREATE TABLE billing_kiwify_events (
    envelope_id      TEXT PRIMARY KEY,
    trigger          TEXT NOT NULL,
    raw_body         JSONB NOT NULL,
    received_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    processed_at     TIMESTAMPTZ NULL,
    signature_status TEXT NOT NULL CHECK (signature_status IN ('valid', 'invalid', 'rotated'))
);

CREATE INDEX billing_kiwify_events_received_at_idx
    ON billing_kiwify_events (received_at);

CREATE INDEX billing_kiwify_events_trigger_idx
    ON billing_kiwify_events (trigger);
