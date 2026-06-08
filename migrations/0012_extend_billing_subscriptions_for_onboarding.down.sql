DROP INDEX IF EXISTS idx_billing_subscriptions_external_sale_id;
ALTER TABLE billing_subscriptions
    DROP COLUMN IF EXISTS external_sale_id,
    DROP COLUMN IF EXISTS customer_email,
    DROP COLUMN IF EXISTS customer_mobile_e164;
