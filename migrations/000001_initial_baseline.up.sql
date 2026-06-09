SET LOCAL lock_timeout    = '5s';
SET LOCAL statement_timeout = '120s';

CREATE SCHEMA IF NOT EXISTS mecontrola;

-- ============================================================
-- Platform: Transactional Outbox
-- ============================================================

CREATE TABLE mecontrola.outbox_events (
    id              UUID        NOT NULL,
    event_type      TEXT        NOT NULL,
    aggregate_type  TEXT        NOT NULL,
    aggregate_id    TEXT        NOT NULL,
    payload         JSONB       NOT NULL,
    metadata        JSONB       NOT NULL DEFAULT '{}'::jsonb,
    status          SMALLINT    NOT NULL,
    attempts        INT         NOT NULL DEFAULT 0,
    max_attempts    INT         NOT NULL,
    next_attempt_at TIMESTAMPTZ NOT NULL,
    last_error      TEXT,
    locked_at       TIMESTAMPTZ,
    locked_by       TEXT,
    published_at    TIMESTAMPTZ,
    occurred_at     TIMESTAMPTZ NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT outbox_events_pkey                  PRIMARY KEY (id),
    CONSTRAINT outbox_events_status_check          CHECK (status IN (1, 2, 3, 4)),
    CONSTRAINT outbox_events_attempts_check        CHECK (attempts >= 0),
    CONSTRAINT outbox_events_max_attempts_check    CHECK (max_attempts > 0),
    CONSTRAINT outbox_events_attempts_max_check    CHECK (attempts <= max_attempts),
    CONSTRAINT outbox_events_published_status_check
        CHECK ((status = 3) = (published_at IS NOT NULL))
) WITH (fillfactor = 70);

COMMENT ON COLUMN mecontrola.outbox_events.status IS '1=Pending, 2=Processing, 3=Published, 4=Failed';

ALTER TABLE mecontrola.outbox_events SET (
    autovacuum_vacuum_scale_factor  = 0.05,
    autovacuum_analyze_scale_factor = 0.02,
    autovacuum_vacuum_cost_delay    = 2
);

CREATE INDEX outbox_events_dispatcher_pending_idx
    ON mecontrola.outbox_events (next_attempt_at)
    WHERE status = 1;

CREATE INDEX outbox_events_reaper_processing_idx
    ON mecontrola.outbox_events (locked_at)
    WHERE status = 2;

CREATE INDEX outbox_events_housekeeping_published_idx
    ON mecontrola.outbox_events (published_at)
    WHERE status = 3;

CREATE INDEX outbox_events_aggregate_idx
    ON mecontrola.outbox_events (aggregate_type, aggregate_id);

-- ============================================================
-- Identity
-- ============================================================

CREATE TABLE mecontrola.users (
    id              UUID        NOT NULL,
    whatsapp_number TEXT        NOT NULL,
    email           TEXT        NULL,
    display_name    TEXT        NULL,
    status          TEXT        NOT NULL DEFAULT 'ACTIVE',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at      TIMESTAMPTZ NULL,

    CONSTRAINT users_pkey PRIMARY KEY (id),
    CONSTRAINT users_status_check CHECK (status IN ('ACTIVE', 'DELETED')),
    CONSTRAINT users_status_deleted_at_check
        CHECK ((status = 'DELETED') = (deleted_at IS NOT NULL))
);

CREATE UNIQUE INDEX users_whatsapp_number_active_uniq_idx
    ON mecontrola.users (whatsapp_number)
    WHERE deleted_at IS NULL;

CREATE INDEX users_whatsapp_number_deleted_idx
    ON mecontrola.users (whatsapp_number)
    WHERE deleted_at IS NOT NULL;

CREATE UNIQUE INDEX users_email_active_uniq_idx
    ON mecontrola.users (email)
    WHERE email IS NOT NULL AND deleted_at IS NULL;

CREATE TABLE mecontrola.user_whatsapp_history (
    id          UUID        NOT NULL,
    user_id     UUID        NOT NULL,
    number      TEXT        NOT NULL,
    active      BOOLEAN     NOT NULL,
    linked_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    unlinked_at TIMESTAMPTZ NULL,
    reason      TEXT        NULL,

    CONSTRAINT user_whatsapp_history_pkey PRIMARY KEY (id),
    CONSTRAINT user_whatsapp_history_user_id_fkey
        FOREIGN KEY (user_id) REFERENCES mecontrola.users (id) ON DELETE CASCADE,
    CONSTRAINT user_whatsapp_history_active_unlinked_at_check
        CHECK ((active = TRUE) = (unlinked_at IS NULL))
);

CREATE INDEX user_whatsapp_history_user_active_idx
    ON mecontrola.user_whatsapp_history (user_id, active);

CREATE INDEX user_whatsapp_history_number_idx
    ON mecontrola.user_whatsapp_history (number);

CREATE TABLE mecontrola.identity_entitlements (
    user_id         UUID        NOT NULL,
    subscription_id UUID        NOT NULL,
    status          TEXT        NOT NULL,
    period_end      TIMESTAMPTZ NOT NULL,
    grace_end       TIMESTAMPTZ NULL,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT identity_entitlements_pkey         PRIMARY KEY (user_id),
    CONSTRAINT identity_entitlements_status_check
        CHECK (status IN ('TRIALING', 'ACTIVE', 'PAST_DUE', 'CANCELED_PENDING', 'EXPIRED', 'REFUNDED')),
    CONSTRAINT identity_entitlements_user_id_fkey
        FOREIGN KEY (user_id) REFERENCES mecontrola.users (id)
) WITH (fillfactor = 80);

CREATE INDEX identity_entitlements_subscription_id_idx
    ON mecontrola.identity_entitlements (subscription_id);

CREATE TABLE mecontrola.identity_entitlements_pending (
    subscription_id UUID        NOT NULL,
    funnel_token    TEXT        NOT NULL,
    payload         JSONB       NOT NULL,
    received_at     TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT identity_entitlements_pending_pkey PRIMARY KEY (subscription_id)
);

CREATE INDEX identity_entitlements_pending_funnel_token_idx
    ON mecontrola.identity_entitlements_pending (funnel_token);

CREATE TABLE mecontrola.auth_events (
    id          UUID        NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    user_id     UUID        NULL,
    kind        TEXT        NOT NULL,
    source      TEXT        NOT NULL,
    reason      TEXT        NULL,

    CONSTRAINT auth_events_pkey         PRIMARY KEY (id),
    CONSTRAINT auth_events_kind_check
        CHECK (kind IN ('principal_established', 'failed', 'unknown_user')),
    CONSTRAINT auth_events_source_check
        CHECK (source = 'whatsapp'),
    CONSTRAINT auth_events_reason_check
        CHECK (
            (kind = 'failed' AND reason IN ('invalid_signature', 'unknown_wa_id', 'invalid_country', 'invalid_payload', 'rate_limited', 'db_unavailable'))
            OR (kind <> 'failed' AND reason IS NULL)
        )
);

CREATE INDEX auth_events_user_id_occurred_at_idx
    ON mecontrola.auth_events (user_id, occurred_at DESC)
    WHERE user_id IS NOT NULL;

CREATE INDEX auth_events_failed_occurred_at_idx
    ON mecontrola.auth_events (occurred_at DESC, reason)
    WHERE kind = 'failed';

-- ============================================================
-- Billing
-- ============================================================

CREATE TABLE mecontrola.billing_plans (
    kiwify_product_id TEXT    NOT NULL,
    code              TEXT    NOT NULL,
    duration_days     INTEGER NOT NULL,
    currency          TEXT    NOT NULL DEFAULT 'BRL',

    CONSTRAINT billing_plans_pkey                PRIMARY KEY (kiwify_product_id),
    CONSTRAINT billing_plans_code_uniq           UNIQUE (code),
    CONSTRAINT billing_plans_code_check          CHECK (code IN ('MONTHLY', 'QUARTERLY', 'ANNUAL')),
    CONSTRAINT billing_plans_duration_days_check CHECK (duration_days > 0),
    CONSTRAINT billing_plans_currency_check      CHECK (currency <> '')
);

INSERT INTO mecontrola.billing_plans (kiwify_product_id, code, duration_days) VALUES
    ('__PLACEHOLDER_MONTHLY__',   'MONTHLY',   30),
    ('__PLACEHOLDER_QUARTERLY__', 'QUARTERLY', 90),
    ('__PLACEHOLDER_ANNUAL__',    'ANNUAL',    365)
ON CONFLICT (kiwify_product_id) DO NOTHING;

CREATE TABLE mecontrola.billing_subscriptions (
    id                     UUID        NOT NULL,
    funnel_token           TEXT        NOT NULL,
    user_id                UUID        NULL,
    kiwify_order_id        TEXT        NOT NULL,
    kiwify_subscription_id TEXT        NULL,
    plan_code              TEXT        NOT NULL,
    status                 TEXT        NOT NULL,
    period_start           TIMESTAMPTZ NOT NULL,
    period_end             TIMESTAMPTZ NOT NULL,
    grace_end              TIMESTAMPTZ NULL,
    last_event_at          TIMESTAMPTZ NOT NULL,
    customer_mobile_e164   TEXT        NULL,
    customer_email         TEXT        NULL,
    external_sale_id       TEXT        NULL,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT billing_subscriptions_pkey           PRIMARY KEY (id),
    CONSTRAINT billing_subscriptions_status_check
        CHECK (status IN ('TRIALING', 'ACTIVE', 'PAST_DUE', 'CANCELED_PENDING', 'EXPIRED', 'REFUNDED')),
    CONSTRAINT billing_subscriptions_plan_code_fkey
        FOREIGN KEY (plan_code) REFERENCES mecontrola.billing_plans (code)
) WITH (fillfactor = 80);

CREATE UNIQUE INDEX billing_subscriptions_user_active_uniq_idx
    ON mecontrola.billing_subscriptions (user_id)
    WHERE user_id IS NOT NULL
      AND status IN ('ACTIVE', 'PAST_DUE', 'CANCELED_PENDING');

CREATE UNIQUE INDEX billing_subscriptions_kiwify_order_uniq_idx
    ON mecontrola.billing_subscriptions (kiwify_order_id);

CREATE INDEX billing_subscriptions_funnel_token_idx
    ON mecontrola.billing_subscriptions (funnel_token);

CREATE INDEX billing_subscriptions_external_sale_id_idx
    ON mecontrola.billing_subscriptions (external_sale_id)
    WHERE external_sale_id IS NOT NULL;

CREATE TABLE mecontrola.billing_processed_events (
    event_key   TEXT        NOT NULL,
    trigger     TEXT        NOT NULL,
    recurso_id  TEXT        NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL,
    applied_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    status      TEXT        NOT NULL,

    CONSTRAINT billing_processed_events_pkey         PRIMARY KEY (event_key),
    CONSTRAINT billing_processed_events_status_check CHECK (status IN ('applied', 'superseded'))
);

CREATE INDEX billing_processed_events_recurso_idx
    ON mecontrola.billing_processed_events (recurso_id);

CREATE TABLE mecontrola.billing_kiwify_events (
    envelope_id      TEXT        NOT NULL,
    trigger          TEXT        NOT NULL,
    raw_body         JSONB       NOT NULL,
    received_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    processed_at     TIMESTAMPTZ NULL,
    signature_status TEXT        NOT NULL,

    CONSTRAINT billing_kiwify_events_pkey                    PRIMARY KEY (envelope_id),
    CONSTRAINT billing_kiwify_events_signature_status_check
        CHECK (signature_status IN ('valid', 'invalid', 'rotated'))
) WITH (fillfactor = 85);

CREATE INDEX billing_kiwify_events_received_at_idx
    ON mecontrola.billing_kiwify_events (received_at);

CREATE INDEX billing_kiwify_events_trigger_idx
    ON mecontrola.billing_kiwify_events (trigger);

CREATE TABLE mecontrola.billing_reconciliation_checkpoints (
    name       TEXT        NOT NULL,
    watermark  TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT billing_reconciliation_checkpoints_pkey PRIMARY KEY (name)
);

-- ============================================================
-- Onboarding
-- ============================================================

CREATE TABLE mecontrola.onboarding_tokens (
    id                          UUID        NOT NULL,
    token_hash                  BYTEA       NOT NULL,
    status                      TEXT        NOT NULL,
    plan_id                     UUID        NOT NULL,
    expires_at                  TIMESTAMPTZ NOT NULL,
    created_at                  TIMESTAMPTZ NOT NULL DEFAULT now(),
    paid_at                     TIMESTAMPTZ NULL,
    consumed_at                 TIMESTAMPTZ NULL,
    outreach_sent_at            TIMESTAMPTZ NULL,
    activation_token_ciphertext TEXT        NOT NULL,
    subscription_id             UUID        NULL,
    customer_mobile_e164        TEXT        NULL,
    customer_email              TEXT        NULL,
    external_sale_id            TEXT        NULL,
    consumed_by_user_id         UUID        NULL,
    consumed_by_mobile_e164     TEXT        NULL,
    activation_path             TEXT        NULL,
    metadata                    JSONB       NOT NULL DEFAULT '{}'::jsonb,

    CONSTRAINT onboarding_tokens_pkey              PRIMARY KEY (id),
    CONSTRAINT onboarding_tokens_token_hash_uniq   UNIQUE (token_hash),
    CONSTRAINT onboarding_tokens_status_check
        CHECK (status IN ('PENDING', 'PAID', 'CONSUMED', 'EXPIRED')),
    CONSTRAINT onboarding_tokens_activation_path_check
        CHECK (activation_path IN ('direct', 'fallback_e164', 'outreach', 'admin'))
);

CREATE INDEX onboarding_tokens_status_expires_idx
    ON mecontrola.onboarding_tokens (status, expires_at)
    WHERE status IN ('PENDING', 'PAID');

CREATE INDEX onboarding_tokens_outreach_pick_idx
    ON mecontrola.onboarding_tokens (status, paid_at)
    WHERE status = 'PAID' AND outreach_sent_at IS NULL;

CREATE INDEX onboarding_tokens_by_mobile_paid_idx
    ON mecontrola.onboarding_tokens (customer_mobile_e164)
    WHERE status = 'PAID' AND outreach_sent_at IS NOT NULL;

CREATE TABLE mecontrola.consumer_lookup_attempts (
    event_id        TEXT        NOT NULL,
    attempts        INT         NOT NULL DEFAULT 1,
    last_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT consumer_lookup_attempts_pkey           PRIMARY KEY (event_id),
    CONSTRAINT consumer_lookup_attempts_attempts_check CHECK (attempts > 0)
);

CREATE INDEX consumer_lookup_attempts_last_attempt_idx
    ON mecontrola.consumer_lookup_attempts (last_attempt_at);

CREATE TABLE mecontrola.support_signals (
    id          UUID        NOT NULL,
    kind        TEXT        NOT NULL,
    payload     JSONB       NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    resolved_at TIMESTAMPTZ NULL,
    resolved_by TEXT        NULL,
    notes       TEXT        NULL,

    CONSTRAINT support_signals_pkey       PRIMARY KEY (id),
    CONSTRAINT support_signals_kind_check
        CHECK (kind IN ('orphan_expired_subscription', 'paid_without_token', 'token_reuse_attempt'))
);

CREATE INDEX support_signals_kind_open_idx
    ON mecontrola.support_signals (kind, occurred_at)
    WHERE resolved_at IS NULL;

-- ============================================================
-- Platform: WhatsApp deduplication
-- ============================================================

CREATE TABLE mecontrola.meta_processed_messages (
    wamid        TEXT        NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT meta_processed_messages_pkey PRIMARY KEY (wamid)
);

CREATE INDEX meta_processed_messages_processed_at_idx
    ON mecontrola.meta_processed_messages (processed_at);
