DROP INDEX IF EXISTS mecontrola.outbox_events_aggregate_user_id_idx;

ALTER TABLE mecontrola.outbox_events
    DROP COLUMN IF EXISTS aggregate_user_id;
