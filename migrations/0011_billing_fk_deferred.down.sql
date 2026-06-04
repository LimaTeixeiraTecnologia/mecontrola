-- migration: 0011_billing_fk_deferred.down.sql
-- Reverte a FK billing_event_applications → subscriptions para IMMEDIATE.

ALTER TABLE billing_event_applications
    DROP CONSTRAINT IF EXISTS fk_billing_event_applications_subscription_id;

ALTER TABLE billing_event_applications
    ADD CONSTRAINT fk_billing_event_applications_subscription_id
        FOREIGN KEY (subscription_id) REFERENCES subscriptions (id)
        ON DELETE CASCADE;
