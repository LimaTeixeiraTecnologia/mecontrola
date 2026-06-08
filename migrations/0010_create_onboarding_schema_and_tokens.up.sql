CREATE SCHEMA IF NOT EXISTS onboarding;

CREATE TABLE onboarding.onboarding_tokens (
    id                       UUID         PRIMARY KEY,
    token_hash               BYTEA        NOT NULL UNIQUE,
    status                   TEXT         NOT NULL CHECK (status IN ('PENDING','PAID','CONSUMED','EXPIRED')),
    plan_id                  UUID         NOT NULL,
    expires_at               TIMESTAMPTZ  NOT NULL,
    created_at               TIMESTAMPTZ  NOT NULL DEFAULT now(),
    paid_at                  TIMESTAMPTZ  NULL,
    consumed_at              TIMESTAMPTZ  NULL,
    outreach_sent_at         TIMESTAMPTZ  NULL,
    activation_token_ciphertext TEXT     NOT NULL,
    subscription_id          UUID         NULL,
    customer_mobile_e164     TEXT         NULL,
    customer_email           TEXT         NULL,
    external_sale_id         TEXT         NULL,
    consumed_by_user_id      UUID         NULL,
    consumed_by_mobile_e164  TEXT         NULL,
    activation_path          TEXT         NULL CHECK (activation_path IN ('direct','fallback_e164','outreach','admin')),
    metadata                 JSONB        NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX idx_onboarding_tokens_status_expires ON onboarding.onboarding_tokens (status, expires_at)
    WHERE status IN ('PENDING','PAID');

CREATE INDEX idx_onboarding_tokens_outreach_pick ON onboarding.onboarding_tokens (status, paid_at)
    WHERE status = 'PAID' AND outreach_sent_at IS NULL;

CREATE INDEX idx_onboarding_tokens_by_mobile_paid ON onboarding.onboarding_tokens (customer_mobile_e164)
    WHERE status = 'PAID' AND outreach_sent_at IS NOT NULL;

CREATE TABLE onboarding.consumer_lookup_attempts (
    event_id       TEXT         PRIMARY KEY,
    attempts       INT          NOT NULL DEFAULT 1,
    last_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_consumer_lookup_attempts_last_attempt ON onboarding.consumer_lookup_attempts (last_attempt_at);
