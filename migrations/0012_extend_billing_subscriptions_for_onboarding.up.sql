ALTER TABLE billing_subscriptions
    ADD COLUMN IF NOT EXISTS customer_mobile_e164 TEXT NULL,
    ADD COLUMN IF NOT EXISTS customer_email       TEXT NULL,
    ADD COLUMN IF NOT EXISTS external_sale_id     TEXT NULL;

CREATE INDEX IF NOT EXISTS idx_billing_subscriptions_external_sale_id
    ON billing_subscriptions (external_sale_id)
    WHERE external_sale_id IS NOT NULL;
