-- migration: 0011_billing_fk_deferred.up.sql
-- Torna a FK billing_event_applications → subscriptions DEFERRABLE INITIALLY DEFERRED.
-- Necessário porque ProcessBillingEventUseCase insere billing_event_applications antes de
-- fazer upsert da subscription dentro da mesma transação (CA-02 idempotência).
-- Referência: ADR-012 (pessimistic locking), CA-02.

ALTER TABLE billing_event_applications
    DROP CONSTRAINT IF EXISTS fk_billing_event_applications_subscription_id;

ALTER TABLE billing_event_applications
    ADD CONSTRAINT fk_billing_event_applications_subscription_id
        FOREIGN KEY (subscription_id) REFERENCES subscriptions (id)
        ON DELETE CASCADE
        DEFERRABLE INITIALLY DEFERRED;
