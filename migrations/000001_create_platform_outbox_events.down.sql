DROP INDEX IF EXISTS outbox_events_aggregate_idx;
DROP INDEX IF EXISTS outbox_events_housekeeping_published_idx;
DROP INDEX IF EXISTS outbox_events_reaper_processing_idx;
DROP INDEX IF EXISTS outbox_events_dispatcher_pending_idx;
DROP TABLE IF EXISTS outbox_events;
