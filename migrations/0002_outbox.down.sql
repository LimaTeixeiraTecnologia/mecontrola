DROP INDEX IF EXISTS ix_outbox_deliveries_claimed_stuck;
DROP INDEX IF EXISTS ix_outbox_deliveries_housekeeping;
DROP INDEX IF EXISTS ix_outbox_deliveries_subscription_status;
DROP INDEX IF EXISTS ix_outbox_deliveries_claim_ready;
DROP TABLE IF EXISTS outbox_deliveries;
DROP INDEX IF EXISTS ix_outbox_events_type_aggregate;
DROP TABLE IF EXISTS outbox_events;
