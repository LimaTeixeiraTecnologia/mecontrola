DROP INDEX IF EXISTS idx_outbox_aggregate;
DROP INDEX IF EXISTS idx_outbox_housekeeping_published;
DROP INDEX IF EXISTS idx_outbox_reaper_processing;
DROP INDEX IF EXISTS idx_outbox_dispatcher_pending;
DROP TABLE IF EXISTS outbox_events;
