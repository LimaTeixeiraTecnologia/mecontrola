ALTER TABLE mecontrola.outbox_events
    ADD COLUMN aggregate_user_id UUID NULL;

CREATE INDEX IF NOT EXISTS outbox_events_aggregate_user_id_idx
    ON mecontrola.outbox_events (aggregate_user_id)
    WHERE aggregate_user_id IS NOT NULL;
