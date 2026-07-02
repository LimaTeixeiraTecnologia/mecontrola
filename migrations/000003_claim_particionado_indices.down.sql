SET LOCAL lock_timeout    = '5s';
SET LOCAL statement_timeout = '120s';

DROP INDEX IF EXISTS mecontrola.outbox_events_user_inflight_uidx;
DROP INDEX IF EXISTS mecontrola.outbox_events_user_pending_occurred_idx;
