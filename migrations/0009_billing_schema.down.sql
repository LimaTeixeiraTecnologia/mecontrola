-- migration: 0009_billing_schema.down.sql
-- Reverte o substrato do módulo billing na ordem inversa de criação.

DROP INDEX IF EXISTS idx_billing_event_applications_subscription_id;
DROP TABLE IF EXISTS billing_event_applications;

DROP INDEX IF EXISTS idx_subscriptions_status_active;
DROP INDEX IF EXISTS uq_subscriptions_one_active_per_user;
DROP INDEX IF EXISTS uq_subscriptions_external_id;
DROP TABLE IF EXISTS subscriptions;

DROP INDEX IF EXISTS idx_webhook_events_pending_anonymization;
DROP INDEX IF EXISTS idx_webhook_events_received_at;
DROP TABLE IF EXISTS webhook_events;

DROP TABLE IF EXISTS billing_plans;
