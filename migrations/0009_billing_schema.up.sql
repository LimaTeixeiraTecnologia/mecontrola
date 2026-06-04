-- migration: 0009_billing_schema.up.sql
-- Cria substrato do módulo billing: event store imutável (webhook_events),
-- agregado Subscription, idempotência de processamento (billing_event_applications)
-- e tabela de planos read-only (billing_plans).

CREATE TABLE IF NOT EXISTS webhook_events (
    id                  TEXT         NOT NULL,
    provider            TEXT         NOT NULL,
    external_event_id   TEXT         NOT NULL,
    event_type          TEXT         NOT NULL,
    signature           TEXT,
    headers             JSONB        NOT NULL DEFAULT '{}'::jsonb,
    payload             JSONB        NOT NULL,
    received_at         TIMESTAMPTZ  NOT NULL DEFAULT now(),
    processed_at        TIMESTAMPTZ,
    anonymized_at       TIMESTAMPTZ,
    CONSTRAINT pk_webhook_events PRIMARY KEY (id),
    CONSTRAINT uq_webhook_events_external
        UNIQUE (provider, external_event_id)
);

CREATE INDEX IF NOT EXISTS idx_webhook_events_received_at
    ON webhook_events (received_at DESC);

CREATE INDEX IF NOT EXISTS idx_webhook_events_pending_anonymization
    ON webhook_events (received_at)
    WHERE anonymized_at IS NULL;

REVOKE ALL ON TABLE webhook_events FROM PUBLIC;

CREATE TABLE IF NOT EXISTS subscriptions (
    id                         TEXT         NOT NULL,
    user_id                    UUID         NOT NULL,
    provider                   TEXT         NOT NULL,
    external_subscription_id   TEXT         NOT NULL,
    plan_code                  TEXT         NOT NULL,
    status                     TEXT         NOT NULL,
    period_start               TIMESTAMPTZ  NOT NULL,
    period_end                 TIMESTAMPTZ  NOT NULL,
    grace_period_end           TIMESTAMPTZ,
    refund_amount_cents        BIGINT       NOT NULL DEFAULT 0,
    last_event_at              TIMESTAMPTZ  NOT NULL,
    last_webhook_event_id      TEXT         NOT NULL,
    created_at                 TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at                 TIMESTAMPTZ  NOT NULL DEFAULT now(),
    deleted_at                 TIMESTAMPTZ,
    CONSTRAINT pk_subscriptions PRIMARY KEY (id),
    CONSTRAINT fk_subscriptions_user_id
        FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE RESTRICT,
    CONSTRAINT fk_subscriptions_last_webhook_event_id
        FOREIGN KEY (last_webhook_event_id) REFERENCES webhook_events (id) ON DELETE RESTRICT,
    CONSTRAINT ck_subscriptions_status CHECK (status IN
        ('TRIALING','ACTIVE','PAST_DUE','CANCELED_PENDING','EXPIRED','REFUNDED')),
    CONSTRAINT ck_subscriptions_period CHECK (period_end > period_start),
    CONSTRAINT ck_subscriptions_refund CHECK (refund_amount_cents >= 0)
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_subscriptions_external_id
    ON subscriptions (provider, external_subscription_id)
    WHERE deleted_at IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS uq_subscriptions_one_active_per_user
    ON subscriptions (user_id)
    WHERE status IN ('TRIALING','ACTIVE','PAST_DUE','CANCELED_PENDING')
      AND deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_subscriptions_status_active
    ON subscriptions (status)
    WHERE status IN ('ACTIVE','PAST_DUE') AND deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS billing_event_applications (
    event_id         TEXT         NOT NULL,
    subscription_id  TEXT         NOT NULL,
    applied_at       TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT pk_billing_event_applications PRIMARY KEY (event_id),
    CONSTRAINT fk_billing_event_applications_subscription_id
        FOREIGN KEY (subscription_id) REFERENCES subscriptions (id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_billing_event_applications_subscription_id
    ON billing_event_applications (subscription_id);

CREATE TABLE IF NOT EXISTS billing_plans (
    plan_code           TEXT         NOT NULL,
    display_name        TEXT         NOT NULL,
    period_length_days  INTEGER      NOT NULL,
    price_brl_cents     BIGINT       NOT NULL,
    kiwify_product_id   TEXT,
    active              BOOLEAN      NOT NULL DEFAULT true,
    created_at          TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT pk_billing_plans PRIMARY KEY (plan_code),
    CONSTRAINT ck_billing_plans_period CHECK (period_length_days > 0),
    CONSTRAINT ck_billing_plans_price CHECK (price_brl_cents > 0)
);
