SET LOCAL lock_timeout    = '5s';
SET LOCAL statement_timeout = '120s';

CREATE INDEX IF NOT EXISTS outbox_events_user_pending_occurred_idx
    ON mecontrola.outbox_events (aggregate_user_id, occurred_at, created_at, id)
    WHERE status = 1 AND aggregate_user_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS outbox_events_user_inflight_uidx
    ON mecontrola.outbox_events (aggregate_user_id)
    WHERE status = 2 AND aggregate_user_id IS NOT NULL;
