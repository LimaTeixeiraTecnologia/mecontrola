SET LOCAL lock_timeout    = '5s';
SET LOCAL statement_timeout = '120s';

CREATE SCHEMA IF NOT EXISTS mecontrola;

-- ============================================================
-- Platform: Transactional Outbox
-- ============================================================

CREATE TABLE IF NOT EXISTS mecontrola.outbox_events (
    id              UUID        NOT NULL,
    event_type      TEXT        NOT NULL,
    aggregate_type  TEXT        NOT NULL,
    aggregate_id    TEXT        NOT NULL,
    aggregate_user_id UUID      NULL,
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

CREATE INDEX IF NOT EXISTS outbox_events_dispatcher_pending_idx
    ON mecontrola.outbox_events (next_attempt_at)
    WHERE status = 1;

CREATE INDEX IF NOT EXISTS outbox_events_reaper_processing_idx
    ON mecontrola.outbox_events (locked_at)
    WHERE status = 2;

CREATE INDEX IF NOT EXISTS outbox_events_housekeeping_published_idx
    ON mecontrola.outbox_events (published_at)
    WHERE status = 3;

CREATE INDEX IF NOT EXISTS outbox_events_aggregate_idx
    ON mecontrola.outbox_events (aggregate_type, aggregate_id);

CREATE INDEX IF NOT EXISTS outbox_events_aggregate_user_id_idx
    ON mecontrola.outbox_events (aggregate_user_id)
    WHERE aggregate_user_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS outbox_events_user_pending_occurred_idx
    ON mecontrola.outbox_events (aggregate_user_id, occurred_at, created_at, id)
    WHERE status = 1 AND aggregate_user_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS outbox_events_user_inflight_uidx
    ON mecontrola.outbox_events (aggregate_user_id)
    WHERE status = 2 AND aggregate_user_id IS NOT NULL;

-- ============================================================
-- Identity
-- ============================================================

CREATE TABLE IF NOT EXISTS mecontrola.users (
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

CREATE UNIQUE INDEX IF NOT EXISTS users_whatsapp_number_active_uniq_idx
    ON mecontrola.users (whatsapp_number)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS users_whatsapp_number_deleted_idx
    ON mecontrola.users (whatsapp_number)
    WHERE deleted_at IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS users_email_active_uniq_idx
    ON mecontrola.users (email)
    WHERE email IS NOT NULL AND deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS mecontrola.user_whatsapp_history (
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

CREATE INDEX IF NOT EXISTS user_whatsapp_history_user_active_idx
    ON mecontrola.user_whatsapp_history (user_id, active);

CREATE INDEX IF NOT EXISTS user_whatsapp_history_number_idx
    ON mecontrola.user_whatsapp_history (number);

CREATE TABLE IF NOT EXISTS mecontrola.identity_entitlements (
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

CREATE INDEX IF NOT EXISTS identity_entitlements_subscription_id_idx
    ON mecontrola.identity_entitlements (subscription_id);

CREATE TABLE IF NOT EXISTS mecontrola.identity_entitlements_pending (
    subscription_id UUID        NOT NULL,
    funnel_token    TEXT        NOT NULL,
    payload         JSONB       NOT NULL,
    received_at     TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT identity_entitlements_pending_pkey PRIMARY KEY (subscription_id)
);

CREATE INDEX IF NOT EXISTS identity_entitlements_pending_funnel_token_idx
    ON mecontrola.identity_entitlements_pending (funnel_token);

CREATE TABLE IF NOT EXISTS mecontrola.auth_events (
    id          UUID        NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    user_id     UUID        NULL,
    kind        TEXT        NOT NULL,
    source      TEXT        NOT NULL,
    reason      TEXT        NULL,
    request_id  TEXT        NULL,
    client_ip   INET        NULL,

    CONSTRAINT auth_events_pkey         PRIMARY KEY (id),
    CONSTRAINT auth_events_kind_check
        CHECK (kind IN ('principal_established', 'failed', 'unknown_user')),
    CONSTRAINT auth_events_source_check
        CHECK (source IN ('whatsapp', 'gateway')),
    CONSTRAINT auth_events_reason_check
        CHECK (
            (kind = 'failed' AND reason IN (
                'invalid_signature',
                'unknown_wa_id',
                'invalid_country',
                'invalid_payload',
                'rate_limited',
                'db_unavailable',
                'gateway_missing_header',
                'gateway_invalid_timestamp',
                'gateway_stale_timestamp',
                'gateway_invalid_signature',
                'stale_webhook',
                'invalid_webhook_timestamp'
            ))
            OR (kind <> 'failed' AND reason IS NULL)
        )
);

CREATE INDEX IF NOT EXISTS auth_events_user_id_occurred_at_idx
    ON mecontrola.auth_events (user_id, occurred_at DESC)
    WHERE user_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS auth_events_failed_occurred_at_idx
    ON mecontrola.auth_events (occurred_at DESC, reason)
    WHERE kind = 'failed';

CREATE INDEX IF NOT EXISTS auth_events_request_id_idx
    ON mecontrola.auth_events (request_id)
    WHERE request_id IS NOT NULL;

-- ============================================================
-- Billing
-- ============================================================

CREATE TABLE IF NOT EXISTS mecontrola.billing_plans (
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




CREATE TABLE IF NOT EXISTS mecontrola.billing_subscriptions (
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
        FOREIGN KEY (plan_code) REFERENCES mecontrola.billing_plans (code),
    CONSTRAINT billing_subscriptions_user_id_fkey
        FOREIGN KEY (user_id) REFERENCES mecontrola.users(id) ON DELETE RESTRICT
) WITH (fillfactor = 80);

CREATE UNIQUE INDEX IF NOT EXISTS billing_subscriptions_user_active_uniq_idx
    ON mecontrola.billing_subscriptions (user_id)
    WHERE user_id IS NOT NULL
      AND status IN ('ACTIVE', 'PAST_DUE', 'CANCELED_PENDING');

CREATE UNIQUE INDEX IF NOT EXISTS billing_subscriptions_kiwify_order_uniq_idx
    ON mecontrola.billing_subscriptions (kiwify_order_id);

CREATE INDEX IF NOT EXISTS billing_subscriptions_funnel_token_idx
    ON mecontrola.billing_subscriptions (funnel_token);

CREATE INDEX IF NOT EXISTS billing_subscriptions_external_sale_id_idx
    ON mecontrola.billing_subscriptions (external_sale_id)
    WHERE external_sale_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS mecontrola.billing_processed_events (
    event_key   TEXT        NOT NULL,
    trigger     TEXT        NOT NULL,
    recurso_id  TEXT        NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL,
    applied_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    status      TEXT        NOT NULL,

    CONSTRAINT billing_processed_events_pkey         PRIMARY KEY (event_key),
    CONSTRAINT billing_processed_events_status_check CHECK (status IN ('applied', 'superseded'))
);

CREATE INDEX IF NOT EXISTS billing_processed_events_recurso_idx
    ON mecontrola.billing_processed_events (recurso_id);

CREATE TABLE IF NOT EXISTS mecontrola.billing_kiwify_events (
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

CREATE INDEX IF NOT EXISTS billing_kiwify_events_received_at_idx
    ON mecontrola.billing_kiwify_events (received_at);

CREATE INDEX IF NOT EXISTS billing_kiwify_events_trigger_idx
    ON mecontrola.billing_kiwify_events (trigger);

CREATE TABLE IF NOT EXISTS mecontrola.billing_reconciliation_checkpoints (
    name       TEXT        NOT NULL,
    watermark  TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT billing_reconciliation_checkpoints_pkey PRIMARY KEY (name)
);

-- ============================================================
-- Onboarding
-- ============================================================

CREATE TABLE IF NOT EXISTS mecontrola.onboarding_tokens (
    id                          UUID        NOT NULL,
    token_hash                  BYTEA       NOT NULL,
    status                      TEXT        NOT NULL,
    plan_id                     TEXT        NOT NULL,
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

CREATE INDEX IF NOT EXISTS onboarding_tokens_status_expires_idx
    ON mecontrola.onboarding_tokens (status, expires_at)
    WHERE status IN ('PENDING', 'PAID');

CREATE INDEX IF NOT EXISTS onboarding_tokens_outreach_pick_idx
    ON mecontrola.onboarding_tokens (status, paid_at)
    WHERE status = 'PAID' AND outreach_sent_at IS NULL;

CREATE INDEX IF NOT EXISTS onboarding_tokens_by_mobile_paid_idx
    ON mecontrola.onboarding_tokens (customer_mobile_e164)
    WHERE status = 'PAID' AND outreach_sent_at IS NOT NULL;

CREATE TABLE IF NOT EXISTS mecontrola.consumer_lookup_attempts (
    event_id        TEXT        NOT NULL,
    attempts        INT         NOT NULL DEFAULT 1,
    last_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT consumer_lookup_attempts_pkey           PRIMARY KEY (event_id),
    CONSTRAINT consumer_lookup_attempts_attempts_check CHECK (attempts > 0)
);

CREATE INDEX IF NOT EXISTS consumer_lookup_attempts_last_attempt_idx
    ON mecontrola.consumer_lookup_attempts (last_attempt_at);

CREATE TABLE IF NOT EXISTS mecontrola.support_signals (
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

CREATE INDEX IF NOT EXISTS support_signals_kind_open_idx
    ON mecontrola.support_signals (kind, occurred_at)
    WHERE resolved_at IS NULL;

-- ============================================================
-- Platform: Channel deduplication
-- ============================================================

CREATE TABLE IF NOT EXISTS mecontrola.channel_processed_messages (
    channel       TEXT        NOT NULL,
    message_id    TEXT        NOT NULL,
    processed_at  TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT channel_processed_messages_pkey PRIMARY KEY (channel, message_id),
    CONSTRAINT channel_processed_messages_channel_check
        CHECK (channel IN ('whatsapp')),
    CONSTRAINT channel_processed_messages_message_id_nonempty_check
        CHECK (length(message_id) > 0)
);

CREATE INDEX IF NOT EXISTS channel_processed_messages_processed_at_idx
    ON mecontrola.channel_processed_messages (processed_at);


-- Consolidated from migrations/000003_categories_unaccent.up.sql
SET LOCAL lock_timeout    = '5s';
SET LOCAL statement_timeout = '120s';

CREATE EXTENSION IF NOT EXISTS unaccent WITH SCHEMA mecontrola;


-- Consolidated from migrations/000004_categories_baseline.up.sql
SET LOCAL lock_timeout    = '5s';
SET LOCAL statement_timeout = '120s';

CREATE OR REPLACE FUNCTION mecontrola.immutable_unaccent(text)
RETURNS text
LANGUAGE sql
IMMUTABLE
PARALLEL SAFE
AS $$
    SELECT mecontrola.unaccent($1);
$$;

CREATE TABLE IF NOT EXISTS mecontrola.category_editorial_version (
    version     BIGINT      NOT NULL PRIMARY KEY DEFAULT 1,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO mecontrola.category_editorial_version (version) VALUES (1)
ON CONFLICT DO NOTHING;

CREATE TABLE IF NOT EXISTS mecontrola.categories (
    id                UUID        NOT NULL PRIMARY KEY,
    slug              TEXT        NOT NULL,
    name              TEXT        NOT NULL,
    kind              TEXT        NOT NULL,
    parent_id         UUID        NULL REFERENCES mecontrola.categories(id),
    allocation_type   TEXT        NOT NULL DEFAULT 'consumption',
    deprecated_at     TIMESTAMPTZ NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT categories_kind_check CHECK (kind IN ('income', 'expense')),
    CONSTRAINT categories_allocation_type_check CHECK (allocation_type IN ('consumption', 'asset_allocation')),
    CONSTRAINT categories_no_cycles CHECK (parent_id IS NULL OR parent_id <> id)
);

CREATE OR REPLACE FUNCTION mecontrola.categories_parent_same_kind()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
    parent_kind TEXT;
BEGIN
    IF NEW.parent_id IS NULL THEN
        RETURN NEW;
    END IF;
    SELECT kind INTO parent_kind FROM mecontrola.categories WHERE id = NEW.parent_id;
    IF parent_kind IS NULL THEN
        RAISE EXCEPTION 'categories_parent_same_kind: parent_id % not found', NEW.parent_id;
    END IF;
    IF parent_kind <> NEW.kind THEN
        RAISE EXCEPTION 'categories_parent_same_kind: child kind % does not match parent kind %', NEW.kind, parent_kind;
    END IF;
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS categories_parent_same_kind_trg ON mecontrola.categories;
CREATE TRIGGER categories_parent_same_kind_trg
    BEFORE INSERT OR UPDATE OF parent_id, kind ON mecontrola.categories
    FOR EACH ROW
    EXECUTE FUNCTION mecontrola.categories_parent_same_kind();

CREATE UNIQUE INDEX IF NOT EXISTS categories_kind_slug_uniq_idx
    ON mecontrola.categories (kind, slug);

CREATE INDEX IF NOT EXISTS categories_kind_parent_idx
    ON mecontrola.categories (kind, parent_id)
    WHERE deprecated_at IS NULL;

CREATE INDEX IF NOT EXISTS categories_parent_sort_idx
    ON mecontrola.categories (parent_id, name COLLATE "pt-BR-x-icu")
    WHERE deprecated_at IS NULL;

CREATE TABLE IF NOT EXISTS mecontrola.category_dictionary (
    id                UUID        NOT NULL PRIMARY KEY,
    category_id       UUID        NOT NULL REFERENCES mecontrola.categories(id),
    kind              TEXT        NOT NULL,
    term              TEXT        NOT NULL,
    term_normalized   TEXT        GENERATED ALWAYS AS (lower(mecontrola.immutable_unaccent(term))) STORED,
    signal_type       TEXT        NOT NULL,
    confidence        TEXT        NOT NULL,
    is_ambiguous      BOOLEAN     NOT NULL DEFAULT false,
    deprecated_at     TIMESTAMPTZ NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT dictionary_kind_check CHECK (kind IN ('income', 'expense')),
    CONSTRAINT dictionary_signal_type_check CHECK (signal_type IN ('canonical_name', 'alias', 'phrase', 'merchant', 'segment')),
    CONSTRAINT dictionary_confidence_check CHECK (confidence IN ('high', 'medium', 'low'))
);

CREATE UNIQUE INDEX IF NOT EXISTS dictionary_active_term_uniq_idx
    ON mecontrola.category_dictionary (kind, category_id, term_normalized)
    WHERE deprecated_at IS NULL;

CREATE INDEX IF NOT EXISTS dictionary_term_normalized_idx
    ON mecontrola.category_dictionary (term_normalized)
    WHERE deprecated_at IS NULL;

CREATE INDEX IF NOT EXISTS dictionary_kind_term_normalized_idx
    ON mecontrola.category_dictionary (kind, term_normalized)
    WHERE deprecated_at IS NULL;



-- Consolidated from migrations/000007_create_platform_idempotency_keys.up.sql
SET LOCAL lock_timeout    = '5s';
SET LOCAL statement_timeout = '120s';

CREATE TABLE IF NOT EXISTS mecontrola.idempotency_keys (
    scope            TEXT        NOT NULL,
    key              TEXT        NOT NULL,
    user_id          UUID        NOT NULL,
    request_hash     TEXT        NOT NULL,
    response_status  INT         NOT NULL,
    response_body    BYTEA       NOT NULL,
    expires_at       TIMESTAMPTZ NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT idempotency_keys_pkey                  PRIMARY KEY (scope, key, user_id),
    CONSTRAINT idempotency_keys_key_len_chk           CHECK (char_length(key) BETWEEN 1 AND 128),
    CONSTRAINT idempotency_keys_request_hash_len_chk  CHECK (char_length(request_hash) = 64),
    CONSTRAINT idempotency_keys_status_chk            CHECK (response_status BETWEEN 200 AND 599),
    CONSTRAINT idempotency_keys_body_size_chk         CHECK (octet_length(response_body) <= 65536)
);

CREATE INDEX IF NOT EXISTS idempotency_keys_expires_idx
    ON mecontrola.idempotency_keys (expires_at);


-- Consolidated from migrations/000008_create_card_cards.up.sql
SET LOCAL lock_timeout    = '5s';
SET LOCAL statement_timeout = '120s';

CREATE TABLE IF NOT EXISTS mecontrola.banks (
    code            TEXT        NOT NULL,
    name            TEXT        NOT NULL,
    days_before_due SMALLINT    NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT banks_pkey         PRIMARY KEY (code),
    CONSTRAINT banks_code_len_chk CHECK (char_length(code) BETWEEN 1 AND 64),
    CONSTRAINT banks_name_len_chk CHECK (char_length(name) BETWEEN 1 AND 128),
    CONSTRAINT banks_days_chk     CHECK (days_before_due BETWEEN 1 AND 28)
);

INSERT INTO mecontrola.banks (code, name, days_before_due) VALUES
    ('nubank',          'Nubank',          7),
    ('itau',            'Itaú',            8),
    ('santander',       'Santander',       8),
    ('bradesco',        'Bradesco',        7),
    ('banco-do-brasil', 'Banco do Brasil', 7),
    ('caixa',           'Caixa',           7),
    ('inter',           'Inter',           7),
    ('c6-bank',         'C6 Bank',         7)
ON CONFLICT (code) DO NOTHING;

CREATE TABLE IF NOT EXISTS mecontrola.cards (
    id          UUID        NOT NULL,
    user_id     UUID        NOT NULL,
    bank        TEXT        NOT NULL,
    nickname    TEXT        NOT NULL,
    closing_day SMALLINT    NOT NULL,
    due_day     SMALLINT    NOT NULL,
    version     BIGINT      NOT NULL DEFAULT 1,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at  TIMESTAMPTZ NULL,
    CONSTRAINT cards_pkey             PRIMARY KEY (id),
    CONSTRAINT cards_user_fk          FOREIGN KEY (user_id)
        REFERENCES mecontrola.users(id) ON DELETE RESTRICT,
    CONSTRAINT cards_bank_len_chk     CHECK (char_length(bank)     BETWEEN 1 AND 64),
    CONSTRAINT cards_closing_day_chk  CHECK (closing_day BETWEEN 1 AND 31),
    CONSTRAINT cards_due_day_chk      CHECK (due_day     BETWEEN 1 AND 31),
    CONSTRAINT cards_nickname_len_chk CHECK (char_length(nickname) BETWEEN 1 AND 32)
);

CREATE UNIQUE INDEX IF NOT EXISTS cards_user_nickname_active_uniq_idx
    ON mecontrola.cards (user_id, nickname)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS cards_user_pagination_idx
    ON mecontrola.cards (user_id, created_at DESC, id DESC)
    WHERE deleted_at IS NULL;



-- Consolidated from migrations/000011_categories_hardening.up.sql
SET LOCAL lock_timeout    = '5s';
SET LOCAL statement_timeout = '120s';

DROP INDEX IF EXISTS mecontrola.dictionary_term_normalized_idx;
DROP INDEX IF EXISTS mecontrola.dictionary_kind_term_normalized_idx;

CREATE INDEX IF NOT EXISTS dictionary_term_normalized_idx
    ON mecontrola.category_dictionary (term_normalized COLLATE "pt-BR-x-icu")
    WHERE deprecated_at IS NULL;

CREATE INDEX IF NOT EXISTS dictionary_kind_term_normalized_idx
    ON mecontrola.category_dictionary (kind, term_normalized COLLATE "pt-BR-x-icu")
    WHERE deprecated_at IS NULL;

CREATE OR REPLACE FUNCTION mecontrola.categories_parent_kind_change_blocks_children()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
    child_count INT;
BEGIN
    IF NEW.kind = OLD.kind THEN
        RETURN NEW;
    END IF;
    SELECT count(*) INTO child_count FROM mecontrola.categories WHERE parent_id = NEW.id;
    IF child_count > 0 THEN
        RAISE EXCEPTION 'categories_parent_kind_change_blocks_children: cannot change kind of parent % with % active children', NEW.id, child_count;
    END IF;
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS categories_parent_kind_change_blocks_children_trg ON mecontrola.categories;
CREATE TRIGGER categories_parent_kind_change_blocks_children_trg
    BEFORE UPDATE OF kind ON mecontrola.categories
    FOR EACH ROW
    EXECUTE FUNCTION mecontrola.categories_parent_kind_change_blocks_children();


-- Consolidated from migrations/000012_create_budgets_baseline.up.sql
SET LOCAL lock_timeout    = '5s';
SET LOCAL statement_timeout = '120s';

CREATE TABLE IF NOT EXISTS mecontrola.budgets (
    id           UUID        NOT NULL,
    user_id      UUID        NOT NULL,
    competence   TEXT     NOT NULL CONSTRAINT budgets_competence_chk CHECK (competence ~ '^\d{4}-(0[1-9]|1[0-2])$'),
    total_cents  BIGINT      NOT NULL DEFAULT 0,
    state        SMALLINT    NOT NULL,
    activated_at TIMESTAMPTZ NULL,
    auto_draft   BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at   TIMESTAMPTZ NOT NULL,
    updated_at   TIMESTAMPTZ NOT NULL,
    CONSTRAINT budgets_pkey            PRIMARY KEY (id),
    CONSTRAINT budgets_user_comp_uk    UNIQUE (user_id, competence),
    CONSTRAINT budgets_state_chk        CHECK (state IN (1, 2))
);

CREATE INDEX IF NOT EXISTS budgets_competence_idx
    ON mecontrola.budgets (competence);

CREATE TABLE IF NOT EXISTS mecontrola.budgets_allocations (
    budget_id     UUID    NOT NULL,
    root_slug     TEXT    NOT NULL,
    basis_points  INTEGER NOT NULL,
    planned_cents BIGINT  NOT NULL,
    CONSTRAINT budgets_allocations_pkey             PRIMARY KEY (budget_id, root_slug),
    CONSTRAINT budgets_allocations_budget_fk        FOREIGN KEY (budget_id)
        REFERENCES mecontrola.budgets(id) ON DELETE CASCADE,
    CONSTRAINT budgets_allocations_basis_points_chk CHECK (basis_points >= 0 AND basis_points <= 10000),
    CONSTRAINT budgets_allocations_root_chk         CHECK (root_slug IN (
        'expense.custo_fixo',
        'expense.conhecimento',
        'expense.prazeres',
        'expense.metas',
        'expense.liberdade_financeira'
    ))
);

CREATE TABLE IF NOT EXISTS mecontrola.budgets_expenses (
    id                      UUID        NOT NULL,
    user_id                 UUID        NOT NULL,
    source                  TEXT        NOT NULL,
    external_transaction_id TEXT        NOT NULL,
    subcategory_id          UUID        NOT NULL,
    root_slug               TEXT        NOT NULL,
    competence              TEXT     NOT NULL CONSTRAINT budgets_expenses_competence_chk CHECK (competence ~ '^\d{4}-(0[1-9]|1[0-2])$'),
    amount_cents            BIGINT      NOT NULL,
    occurred_at             TIMESTAMPTZ NOT NULL,
    version                 BIGINT      NOT NULL,
    tombstone_version       BIGINT      NULL,
    deleted_at              TIMESTAMPTZ NULL,
    created_at              TIMESTAMPTZ NOT NULL,
    updated_at              TIMESTAMPTZ NOT NULL,
    CONSTRAINT budgets_expenses_pkey        PRIMARY KEY (id),
    CONSTRAINT budgets_expenses_amount_chk  CHECK (amount_cents > 0),
    CONSTRAINT budgets_expenses_identity_uk UNIQUE (user_id, source, external_transaction_id),
    CONSTRAINT budgets_expenses_root_chk    CHECK (root_slug IN (
        'expense.custo_fixo',
        'expense.conhecimento',
        'expense.prazeres',
        'expense.metas',
        'expense.liberdade_financeira'
    ))
);

CREATE INDEX IF NOT EXISTS budgets_expenses_summary_idx
    ON mecontrola.budgets_expenses (user_id, competence, subcategory_id)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS budgets_expenses_summary_root_idx
    ON mecontrola.budgets_expenses (user_id, competence, root_slug)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS budgets_expenses_deleted_at_idx
    ON mecontrola.budgets_expenses (deleted_at)
    WHERE deleted_at IS NOT NULL;

CREATE TABLE IF NOT EXISTS mecontrola.budgets_threshold_states (
    user_id                     UUID        NOT NULL,
    competence                  TEXT     NOT NULL CONSTRAINT budgets_threshold_competence_chk CHECK (competence ~ '^\d{4}-(0[1-9]|1[0-2])$'),
    root_slug                   TEXT        NOT NULL,
    threshold                   SMALLINT    NOT NULL,
    currently_crossed           BOOLEAN     NOT NULL DEFAULT FALSE,
    version                     BIGINT      NOT NULL DEFAULT 0,
    last_crossed_at             TIMESTAMPTZ NULL,
    last_uncrossed_at           TIMESTAMPTZ NULL,
    last_evaluated_committed_at TIMESTAMPTZ NULL,
    CONSTRAINT budgets_threshold_states_pkey          PRIMARY KEY (user_id, competence, root_slug, threshold),
    CONSTRAINT budgets_threshold_states_threshold_chk CHECK (threshold IN (80, 100))
);

CREATE TABLE IF NOT EXISTS mecontrola.budgets_alerts (
    id                        UUID        NOT NULL,
    user_id                   UUID        NOT NULL,
    competence                TEXT     NOT NULL CONSTRAINT budgets_alerts_competence_chk CHECK (competence ~ '^\d{4}-(0[1-9]|1[0-2])$'),
    root_slug                 TEXT        NOT NULL,
    threshold                 SMALLINT    NOT NULL,
    state                     SMALLINT    NOT NULL,
    triggered_by_committed_at TIMESTAMPTZ NOT NULL,
    spent_cents               BIGINT      NOT NULL,
    planned_cents             BIGINT      NOT NULL,
    created_at                TIMESTAMPTZ NOT NULL,
    CONSTRAINT budgets_alerts_pkey          PRIMARY KEY (id),
    CONSTRAINT budgets_alerts_threshold_chk CHECK (threshold IN (80, 100)),
    CONSTRAINT budgets_alerts_state_chk        CHECK (state BETWEEN 1 AND 5)
);

CREATE INDEX IF NOT EXISTS budgets_alerts_user_comp_idx
    ON mecontrola.budgets_alerts (user_id, competence, root_slug, threshold);

CREATE INDEX IF NOT EXISTS budgets_alerts_listing_idx
    ON mecontrola.budgets_alerts (user_id, created_at DESC)
    WHERE state IN (1, 2);

CREATE TABLE IF NOT EXISTS mecontrola.budgets_expense_events_pending (
    id                      UUID        NOT NULL,
    event_id                UUID        NOT NULL,
    source                  TEXT        NOT NULL,
    user_id                 UUID        NOT NULL,
    external_transaction_id TEXT        NOT NULL,
    expected_version        BIGINT      NOT NULL,
    mutation_kind           SMALLINT    NOT NULL,
    payload                 JSONB       NOT NULL,
    state                   SMALLINT    NOT NULL,
    received_at             TIMESTAMPTZ NOT NULL,
    transitioned_at         TIMESTAMPTZ NULL,
    reason                  TEXT        NULL,
    CONSTRAINT budgets_expense_events_pending_pkey         PRIMARY KEY (id),
    CONSTRAINT budgets_expense_events_pending_event_uk  UNIQUE (event_id),
    CONSTRAINT budgets_expense_events_pending_state_chk    CHECK (state BETWEEN 1 AND 4),
    CONSTRAINT budgets_expense_events_pending_mutation_chk CHECK (mutation_kind BETWEEN 1 AND 3)
);

CREATE INDEX IF NOT EXISTS budgets_pending_state_received_idx
    ON mecontrola.budgets_expense_events_pending (state, received_at)
    WHERE state = 1;

CREATE INDEX IF NOT EXISTS budgets_pending_identity_idx
    ON mecontrola.budgets_expense_events_pending (user_id, source, external_transaction_id)
    WHERE state = 1;


-- Consolidated from migrations/000013_create_budgets_abandoned_draft_signals.up.sql
CREATE TABLE IF NOT EXISTS mecontrola.budgets_abandoned_draft_signals (
    budget_id  UUID NOT NULL,
    signaled_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT budgets_abandoned_draft_signals_pk PRIMARY KEY (budget_id)
);


-- Consolidated from migrations/000014_create_transactions_baseline.up.sql
SET LOCAL lock_timeout    = '5s';
SET LOCAL statement_timeout = '120s';

CREATE TABLE IF NOT EXISTS mecontrola.transactions (
    id                          UUID        NOT NULL,
    user_id                     UUID        NOT NULL,
    direction                   SMALLINT    NOT NULL,
    payment_method              SMALLINT    NOT NULL,
    amount_cents                BIGINT      NOT NULL CONSTRAINT transactions_amount_cents_chk CHECK (amount_cents > 0),
    description                 TEXT        NOT NULL,
    category_id                 UUID        NOT NULL,
    subcategory_id              UUID        NULL,
    category_name_snapshot      TEXT        NOT NULL,
    subcategory_name_snapshot   TEXT        NULL,
    ref_month                   TEXT     NOT NULL CONSTRAINT transactions_ref_month_chk CHECK (ref_month ~ '^\d{4}-(0[1-9]|1[0-2])$'),
    occurred_at                 TIMESTAMPTZ NOT NULL,
    version                     BIGINT      NOT NULL DEFAULT 1,
    deleted_at                  TIMESTAMPTZ NULL,
    created_at                  TIMESTAMPTZ NOT NULL,
    updated_at                  TIMESTAMPTZ NOT NULL,
    CONSTRAINT transactions_pkey PRIMARY KEY (id)
);

CREATE INDEX IF NOT EXISTS transactions_user_month_idx
    ON mecontrola.transactions (user_id, ref_month)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS transactions_user_created_idx
    ON mecontrola.transactions (user_id, created_at DESC, id DESC)
    WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS mecontrola.transactions_card_purchases (
    id                          UUID        NOT NULL,
    user_id                     UUID        NOT NULL,
    card_id                     UUID        NOT NULL,
    direction                   SMALLINT    NOT NULL CONSTRAINT transactions_cp_direction_chk CHECK (direction = 2),
    total_amount_cents          BIGINT      NOT NULL CONSTRAINT transactions_cp_amount_cents_chk CHECK (total_amount_cents > 0),
    installments_total          SMALLINT    NOT NULL CONSTRAINT transactions_cp_installments_chk CHECK (installments_total BETWEEN 1 AND 24),
    description                 TEXT        NOT NULL,
    category_id                 UUID        NOT NULL,
    subcategory_id              UUID        NULL,
    category_name_snapshot      TEXT        NOT NULL,
    subcategory_name_snapshot   TEXT        NULL,
    purchased_at                TIMESTAMPTZ NOT NULL,
    card_closing_day            SMALLINT    NOT NULL CONSTRAINT transactions_cp_closing_day_chk CHECK (card_closing_day BETWEEN 1 AND 31),
    card_due_day                SMALLINT    NOT NULL CONSTRAINT transactions_cp_due_day_chk CHECK (card_due_day BETWEEN 1 AND 31),
    version                     BIGINT      NOT NULL DEFAULT 1,
    deleted_at                  TIMESTAMPTZ NULL,
    created_at                  TIMESTAMPTZ NOT NULL,
    updated_at                  TIMESTAMPTZ NOT NULL,
    CONSTRAINT transactions_card_purchases_pkey    PRIMARY KEY (id),
    CONSTRAINT transactions_card_purchases_card_fk
        FOREIGN KEY (card_id) REFERENCES mecontrola.cards(id) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS transactions_card_purchases_user_card_idx
    ON mecontrola.transactions_card_purchases (user_id, card_id, created_at DESC, id DESC)
    WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS mecontrola.transactions_card_invoices (
    id                  UUID        NOT NULL,
    user_id             UUID        NOT NULL,
    card_id             UUID        NOT NULL,
    ref_month           TEXT     NOT NULL CONSTRAINT transactions_ci_ref_month_chk CHECK (ref_month ~ '^\d{4}-(0[1-9]|1[0-2])$'),
    closing_at          TIMESTAMPTZ NOT NULL,
    due_at              TIMESTAMPTZ NOT NULL,
    items_total_cents   BIGINT      NOT NULL DEFAULT 0,
    version             BIGINT      NOT NULL DEFAULT 1,
    created_at          TIMESTAMPTZ NOT NULL,
    updated_at          TIMESTAMPTZ NOT NULL,
    CONSTRAINT transactions_card_invoices_pkey PRIMARY KEY (id),
    CONSTRAINT transactions_card_invoices_uk   UNIQUE (user_id, card_id, ref_month)
);

CREATE TABLE IF NOT EXISTS mecontrola.transactions_card_invoice_items (
    id                  UUID        NOT NULL,
    invoice_id          UUID        NOT NULL REFERENCES mecontrola.transactions_card_invoices(id),
    purchase_id         UUID        NOT NULL REFERENCES mecontrola.transactions_card_purchases(id),
    user_id             UUID        NOT NULL,
    ref_month           TEXT     NOT NULL CONSTRAINT transactions_cii_ref_month_chk CHECK (ref_month ~ '^\d{4}-(0[1-9]|1[0-2])$'),
    installment_index   SMALLINT    NOT NULL,
    amount_cents        BIGINT      NOT NULL CONSTRAINT transactions_cii_amount_cents_chk CHECK (amount_cents > 0),
    deleted_at          TIMESTAMPTZ NULL,
    created_at          TIMESTAMPTZ NOT NULL,
    updated_at          TIMESTAMPTZ NOT NULL,
    CONSTRAINT transactions_card_invoice_items_pkey        PRIMARY KEY (id),
    CONSTRAINT transactions_card_invoice_items_purchase_uk  UNIQUE (purchase_id, installment_index)
);

CREATE INDEX IF NOT EXISTS transactions_card_invoice_items_user_month_idx
    ON mecontrola.transactions_card_invoice_items (user_id, ref_month)
    WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS mecontrola.transactions_recurring_templates (
    id                          UUID        NOT NULL,
    user_id                     UUID        NOT NULL,
    direction                   SMALLINT    NOT NULL,
    payment_method              SMALLINT    NOT NULL,
    card_id                     UUID        NULL,
    amount_cents                BIGINT      NOT NULL CONSTRAINT transactions_rt_amount_cents_chk CHECK (amount_cents > 0),
    description                 TEXT        NOT NULL,
    category_id                 UUID        NOT NULL,
    subcategory_id              UUID        NULL,
    category_name_snapshot      TEXT        NOT NULL,
    subcategory_name_snapshot   TEXT        NULL,
    frequency                   SMALLINT    NOT NULL,
    day_of_month                SMALLINT    NOT NULL CONSTRAINT transactions_rt_day_of_month_chk CHECK (day_of_month BETWEEN 1 AND 28),
    installments_total          SMALLINT    NOT NULL DEFAULT 1 CONSTRAINT transactions_rt_installments_chk CHECK (installments_total BETWEEN 1 AND 24),
    started_at                  TIMESTAMPTZ NOT NULL,
    ended_at                    TIMESTAMPTZ NULL,
    version                     BIGINT      NOT NULL DEFAULT 1,
    deleted_at                  TIMESTAMPTZ NULL,
    created_at                  TIMESTAMPTZ NOT NULL,
    updated_at                  TIMESTAMPTZ NOT NULL,
    CONSTRAINT transactions_recurring_templates_pkey        PRIMARY KEY (id),
    CONSTRAINT transactions_recurring_templates_credit_chk
        CHECK ((payment_method <> 7) OR (card_id IS NOT NULL)),
    CONSTRAINT transactions_recurring_templates_card_fk
        FOREIGN KEY (card_id) REFERENCES mecontrola.cards(id) ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS transactions_recurring_templates_user_day_idx
    ON mecontrola.transactions_recurring_templates (user_id, day_of_month)
    WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS mecontrola.transactions_recurring_materializations (
    template_id                 UUID        NOT NULL REFERENCES mecontrola.transactions_recurring_templates(id),
    ref_month                   TEXT     NOT NULL CONSTRAINT transactions_rm_ref_month_chk CHECK (ref_month ~ '^\d{4}-(0[1-9]|1[0-2])$'),
    materialized_transaction_id UUID        NULL,
    materialized_purchase_id    UUID        NULL,
    materialized_at             TIMESTAMPTZ NOT NULL,
    CONSTRAINT transactions_recurring_materializations_pkey PRIMARY KEY (template_id, ref_month)
);

CREATE TABLE IF NOT EXISTS mecontrola.transactions_monthly_summary (
    user_id        UUID        NOT NULL,
    ref_month      TEXT     NOT NULL CONSTRAINT transactions_ms_ref_month_chk CHECK (ref_month ~ '^\d{4}-(0[1-9]|1[0-2])$'),
    income_cents   BIGINT      NOT NULL DEFAULT 0,
    outcome_cents  BIGINT      NOT NULL DEFAULT 0,
    total_cents    BIGINT      NOT NULL DEFAULT 0,
    version        BIGINT      NOT NULL DEFAULT 1,
    updated_at     TIMESTAMPTZ NOT NULL,
    CONSTRAINT transactions_monthly_summary_pkey PRIMARY KEY (user_id, ref_month)
);


-- Consolidated from migrations/000018_create_user_identities.up.sql
SET LOCAL lock_timeout      = '5s';
SET LOCAL statement_timeout = '120s';

CREATE TABLE IF NOT EXISTS mecontrola.user_identities (
    id          UUID        NOT NULL,
    user_id     UUID        NOT NULL,
    channel     TEXT        NOT NULL,
    external_id TEXT        NOT NULL,
    verified_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    unlinked_at TIMESTAMPTZ NULL,

    CONSTRAINT user_identities_pkey PRIMARY KEY (id),
    CONSTRAINT user_identities_user_id_fkey
        FOREIGN KEY (user_id) REFERENCES mecontrola.users (id) ON DELETE CASCADE,
    CONSTRAINT user_identities_channel_check
        CHECK (channel IN ('whatsapp')),
    CONSTRAINT user_identities_external_id_nonempty_check
        CHECK (length(external_id) > 0),
    CONSTRAINT user_identities_status_unlinked_at_check
        CHECK (unlinked_at IS NULL OR unlinked_at >= created_at)
);

CREATE UNIQUE INDEX IF NOT EXISTS user_identities_channel_external_active_uniq_idx
    ON mecontrola.user_identities (channel, external_id)
    WHERE unlinked_at IS NULL;

CREATE INDEX IF NOT EXISTS user_identities_user_channel_idx
    ON mecontrola.user_identities (user_id, channel)
    WHERE unlinked_at IS NULL;

CREATE INDEX IF NOT EXISTS user_identities_channel_external_unlinked_idx
    ON mecontrola.user_identities (channel, external_id)
    WHERE unlinked_at IS NOT NULL;

-- ============================================================
-- Budget Alerts Sent — final state (consolidated from 000003 + 000006)
-- ============================================================

CREATE TABLE IF NOT EXISTS mecontrola.budget_alerts_sent (
    user_id        UUID        NOT NULL,
    budget_id      UUID        NOT NULL,
    kind           TEXT        NOT NULL,
    ref_day        DATE        NOT NULL,
    sent_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    notified_at    TIMESTAMPTZ NULL,
    notify_channel TEXT        NULL,

    CONSTRAINT budget_alerts_sent_pkey     PRIMARY KEY (user_id, budget_id, kind, ref_day),
    CONSTRAINT budget_alerts_sent_kind_chk CHECK (kind IN (
        'category_threshold',
        'goal_achieved',
        'card_limit_near'
    ))
);

CREATE INDEX IF NOT EXISTS budget_alerts_sent_user_ref_day_idx
    ON mecontrola.budget_alerts_sent (user_id, ref_day DESC);

CREATE INDEX IF NOT EXISTS budget_alerts_sent_pending_notify_idx
    ON mecontrola.budget_alerts_sent (sent_at)
    WHERE notified_at IS NULL;

SET LOCAL lock_timeout    = '5s';
SET LOCAL statement_timeout = '120s';

CREATE TABLE IF NOT EXISTS mecontrola.card_invoice_alerts_sent (
    user_id        UUID        NOT NULL,
    card_id        UUID        NOT NULL,
    ref_due_date   DATE        NOT NULL,
    sent_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    notified_at    TIMESTAMPTZ NULL,
    notify_channel TEXT        NULL,

    CONSTRAINT card_invoice_alerts_sent_pkey PRIMARY KEY (user_id, card_id, ref_due_date),
    CONSTRAINT card_invoice_alerts_sent_card_fk FOREIGN KEY (card_id)
        REFERENCES mecontrola.cards(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS card_invoice_alerts_sent_user_due_idx
    ON mecontrola.card_invoice_alerts_sent (user_id, ref_due_date DESC);

CREATE INDEX IF NOT EXISTS card_invoice_alerts_sent_pending_notify_idx
    ON mecontrola.card_invoice_alerts_sent (sent_at)
    WHERE notified_at IS NULL;

CREATE INDEX IF NOT EXISTS cards_due_day_scan_idx
    ON mecontrola.cards (due_day)
    WHERE deleted_at IS NULL;

SET LOCAL lock_timeout    = '5s';
SET LOCAL statement_timeout = '120s';

CREATE TABLE IF NOT EXISTS mecontrola.whatsapp_message_status (
    id           UUID        NOT NULL,
    message_id   TEXT        NOT NULL,
    status       TEXT        NOT NULL,
    recipient_id TEXT        NOT NULL DEFAULT '',
    error_code   TEXT,
    error_title  TEXT,
    status_at    TIMESTAMPTZ NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT whatsapp_message_status_pkey PRIMARY KEY (id),
    CONSTRAINT whatsapp_message_status_message_id_len_chk
        CHECK (char_length(message_id) BETWEEN 1 AND 256),
    CONSTRAINT whatsapp_message_status_status_chk
        CHECK (status IN ('sent', 'delivered', 'read', 'failed')),
    CONSTRAINT whatsapp_message_status_message_status_uniq
        UNIQUE (message_id, status)
);

CREATE INDEX IF NOT EXISTS whatsapp_message_status_message_id_idx
    ON mecontrola.whatsapp_message_status (message_id);

CREATE INDEX IF NOT EXISTS whatsapp_message_status_status_idx
    ON mecontrola.whatsapp_message_status (status);

CREATE EXTENSION IF NOT EXISTS pgcrypto WITH SCHEMA mecontrola;

CREATE EXTENSION IF NOT EXISTS pg_trgm WITH SCHEMA mecontrola;

CREATE INDEX IF NOT EXISTS dictionary_term_trgm_idx
    ON mecontrola.category_dictionary
    USING gin (term_normalized gin_trgm_ops)
    WHERE deprecated_at IS NULL;

SET LOCAL lock_timeout    = '5s';
SET LOCAL statement_timeout = '120s';

CREATE TABLE IF NOT EXISTS mecontrola.workflow_runs (
    id              UUID        NOT NULL,
    workflow        TEXT        NOT NULL,
    correlation_key TEXT        NOT NULL,
    status          TEXT        NOT NULL,
    suspend_reason  TEXT        NOT NULL DEFAULT '',
    cursor          INT         NOT NULL DEFAULT 0,
    state           JSONB       NOT NULL DEFAULT '{}',
    attempts        INT         NOT NULL DEFAULT 0,
    max_attempts    INT         NOT NULL,
    version         BIGINT      NOT NULL DEFAULT 1,
    last_error      TEXT        NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    ended_at        TIMESTAMPTZ,
    CONSTRAINT workflow_runs_pkey PRIMARY KEY (id),
    CONSTRAINT workflow_runs_status_check
        CHECK (status IN ('running','suspended','succeeded','failed')),
    CONSTRAINT workflow_runs_attempts_check     CHECK (attempts >= 0),
    CONSTRAINT workflow_runs_max_attempts_check CHECK (max_attempts > 0)
) WITH (fillfactor = 70);

CREATE UNIQUE INDEX IF NOT EXISTS workflow_runs_active_key_uidx
    ON mecontrola.workflow_runs (workflow, correlation_key)
    WHERE status IN ('running','suspended');

CREATE INDEX IF NOT EXISTS workflow_runs_status_updated_idx
    ON mecontrola.workflow_runs (status, updated_at);

CREATE TABLE IF NOT EXISTS mecontrola.workflow_steps (
    id          UUID        NOT NULL,
    run_id      UUID        NOT NULL,
    step_id     TEXT        NOT NULL,
    seq         INT         NOT NULL,
    status      TEXT        NOT NULL,
    attempt     INT         NOT NULL DEFAULT 1,
    duration_ms BIGINT      NOT NULL DEFAULT 0,
    error       TEXT        NOT NULL DEFAULT '',
    started_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    ended_at    TIMESTAMPTZ,
    CONSTRAINT workflow_steps_pkey PRIMARY KEY (id),
    CONSTRAINT workflow_steps_run_seq_attempt_uidx
        UNIQUE (run_id, seq, attempt),
    CONSTRAINT workflow_steps_run_fkey
        FOREIGN KEY (run_id) REFERENCES mecontrola.workflow_runs (id) ON DELETE CASCADE,
    CONSTRAINT workflow_steps_status_check
        CHECK (status IN ('completed','suspended','failed','skipped'))
);

CREATE INDEX IF NOT EXISTS workflow_steps_run_seq_idx
    ON mecontrola.workflow_steps (run_id, seq);
INSERT INTO mecontrola.billing_plans (kiwify_product_id, code, duration_days) VALUES
    ('__PLACEHOLDER_MONTHLY__',   'MONTHLY',   30),
    ('__PLACEHOLDER_QUARTERLY__', 'QUARTERLY', 90),
    ('__PLACEHOLDER_ANNUAL__',    'ANNUAL',    365)
ON CONFLICT (kiwify_product_id) DO NOTHING;

-- Consolidated from migrations/000005_seed_categories.up.sql
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '120s';

INSERT INTO mecontrola.categories (id, slug, name, kind, parent_id, allocation_type) VALUES
('66cb85a0-3266-5900-b8e3-13cdcd00ab62', 'custo-fixo', 'Custo Fixo', 'expense', NULL, 'consumption'),
('8314f021-ee9c-53b4-872f-449ac618da50', 'conhecimento', 'Conhecimento', 'expense', NULL, 'consumption'),
('ac535261-4060-56ef-b2e8-57c8cc7032d1', 'prazeres', 'Prazeres', 'expense', NULL, 'consumption'),
('f133508e-7dc3-58a3-96db-199d8fbd2987', 'metas', 'Metas', 'expense', NULL, 'asset_allocation'),
('35ced21e-b436-5cea-afb9-ffd43f98a124', 'liberdade-financeira', 'Liberdade Financeira', 'expense', NULL, 'asset_allocation')
ON CONFLICT (kind, slug) DO NOTHING;

INSERT INTO mecontrola.categories (id, slug, name, kind, parent_id, allocation_type) VALUES
('c2fda6a3-c329-52c8-81ea-771b6ea4f365', 'aluguel', 'Aluguel', 'expense', '66cb85a0-3266-5900-b8e3-13cdcd00ab62', 'consumption'),
('f9d9e5b6-1437-5204-bd64-2bd7d43583a8', 'financiamento-imobiliario', 'Financiamento Imobiliário', 'expense', '66cb85a0-3266-5900-b8e3-13cdcd00ab62', 'consumption'),
('d0b1fa13-d19f-51b9-afc7-82bf83accf79', 'condominio', 'Condomínio', 'expense', '66cb85a0-3266-5900-b8e3-13cdcd00ab62', 'consumption'),
('80a870e9-831f-5e85-b95f-0afe2f8d372a', 'iptu', 'IPTU', 'expense', '66cb85a0-3266-5900-b8e3-13cdcd00ab62', 'consumption'),
('8eaa0160-80cd-5c14-a361-d98068aab2cd', 'taxas-residenciais', 'Taxas Residenciais', 'expense', '66cb85a0-3266-5900-b8e3-13cdcd00ab62', 'consumption'),
('0abec125-fa91-5ac6-a82e-3686533c4b8d', 'seguro-residencial', 'Seguro Residencial', 'expense', '66cb85a0-3266-5900-b8e3-13cdcd00ab62', 'consumption'),
('36916fab-eacc-50a3-8a53-93671c335952', 'energia', 'Energia', 'expense', '66cb85a0-3266-5900-b8e3-13cdcd00ab62', 'consumption'),
('fa93273d-e2d9-54ed-a6aa-53b5b1830867', 'agua', 'Água', 'expense', '66cb85a0-3266-5900-b8e3-13cdcd00ab62', 'consumption'),
('4e6f8b6b-8ffb-5d38-8ac9-68464679a544', 'gas', 'Gás', 'expense', '66cb85a0-3266-5900-b8e3-13cdcd00ab62', 'consumption'),
('9391ac38-ec2c-55d0-afc8-8c0940678814', 'internet', 'Internet', 'expense', '66cb85a0-3266-5900-b8e3-13cdcd00ab62', 'consumption'),
('7319ba14-0dc7-56ff-ac5c-96024e15ec02', 'telefonia', 'Telefonia', 'expense', '66cb85a0-3266-5900-b8e3-13cdcd00ab62', 'consumption'),
('2e90fdd3-1008-5423-8215-5db1880fa60b', 'tv-por-assinatura', 'TV por Assinatura', 'expense', '66cb85a0-3266-5900-b8e3-13cdcd00ab62', 'consumption'),
('97fa4b86-d43c-5ad5-a99b-c88c8427fb30', 'supermercado', 'Supermercado', 'expense', '66cb85a0-3266-5900-b8e3-13cdcd00ab62', 'consumption'),
('0c004f2d-ad42-5855-a408-f695906cd48c', 'feira-e-hortifruti', 'Feira e Hortifruti', 'expense', '66cb85a0-3266-5900-b8e3-13cdcd00ab62', 'consumption'),
('75af9f6b-78e4-5ef3-b6ca-b84a37f8901c', 'acougue', 'Açougue', 'expense', '66cb85a0-3266-5900-b8e3-13cdcd00ab62', 'consumption'),
('0b549268-cbaf-5531-af54-ab47e14a072a', 'padaria', 'Padaria', 'expense', '66cb85a0-3266-5900-b8e3-13cdcd00ab62', 'consumption'),
('007c090e-7a6d-5645-b751-b93cabb280ed', 'transporte-publico', 'Transporte Público', 'expense', '66cb85a0-3266-5900-b8e3-13cdcd00ab62', 'consumption'),
('c13dcc6e-c37b-521d-a889-8bb02765490f', 'transporte-por-aplicativo-recorrente', 'Transporte por Aplicativo Recorrente', 'expense', '66cb85a0-3266-5900-b8e3-13cdcd00ab62', 'consumption'),
('cb13d50d-43cb-553c-99cd-8851889d7f6e', 'combustivel', 'Combustível', 'expense', '66cb85a0-3266-5900-b8e3-13cdcd00ab62', 'consumption'),
('7e647851-411c-52d7-a0f2-13535469d918', 'estacionamento-mensal', 'Estacionamento Mensal', 'expense', '66cb85a0-3266-5900-b8e3-13cdcd00ab62', 'consumption'),
('9dc2ed94-0ea2-5b72-a948-850670f2bee7', 'pedagio', 'Pedágio', 'expense', '66cb85a0-3266-5900-b8e3-13cdcd00ab62', 'consumption'),
('bf2fcca0-09c3-5dcb-a61a-87eed2860c04', 'manutencao-veicular', 'Manutenção Veicular', 'expense', '66cb85a0-3266-5900-b8e3-13cdcd00ab62', 'consumption'),
('311c7b7f-56a3-5b53-ada7-5b85734ba45f', 'ipva-e-licenciamento', 'IPVA e Licenciamento', 'expense', '66cb85a0-3266-5900-b8e3-13cdcd00ab62', 'consumption'),
('75e7909d-d816-5609-ac03-89d1c6eb31f5', 'seguro-veicular', 'Seguro Veicular', 'expense', '66cb85a0-3266-5900-b8e3-13cdcd00ab62', 'consumption'),
('c8f579ea-952b-5e24-beed-ef22fb845a4b', 'plano-de-saude', 'Plano de Saúde', 'expense', '66cb85a0-3266-5900-b8e3-13cdcd00ab62', 'consumption'),
('1af66343-7305-534f-b8de-47ebcd3d17f1', 'plano-odontologico', 'Plano Odontológico', 'expense', '66cb85a0-3266-5900-b8e3-13cdcd00ab62', 'consumption'),
('af5619e0-3683-5b8c-b9fc-0b3ddfbd2075', 'consultas-e-exames', 'Consultas e Exames', 'expense', '66cb85a0-3266-5900-b8e3-13cdcd00ab62', 'consumption'),
('157b18fe-513e-55fa-969c-c9bd785530d1', 'medicamentos-continuos', 'Medicamentos Contínuos', 'expense', '66cb85a0-3266-5900-b8e3-13cdcd00ab62', 'consumption'),
('3ca95dd5-c630-5c03-bd47-071777bde81c', 'medicamentos-e-farmacia', 'Medicamentos e Farmácia', 'expense', '66cb85a0-3266-5900-b8e3-13cdcd00ab62', 'consumption'),
('4ded7fd4-5335-5cf2-aed1-bdcead596000', 'odontologia', 'Odontologia', 'expense', '66cb85a0-3266-5900-b8e3-13cdcd00ab62', 'consumption'),
('a15cba16-23da-504e-a22b-144392ed82bc', 'terapia-e-saude-mental', 'Terapia e Saúde Mental', 'expense', '66cb85a0-3266-5900-b8e3-13cdcd00ab62', 'consumption'),
('cab69263-ac14-5ed1-ab5d-8372487c9ee8', 'escola-e-creche', 'Escola e Creche', 'expense', '66cb85a0-3266-5900-b8e3-13cdcd00ab62', 'consumption'),
('46e492a0-3909-5e0a-bd3e-16bbdf29db8d', 'faculdade-e-pos-graduacao', 'Faculdade e Pós-graduação', 'expense', '66cb85a0-3266-5900-b8e3-13cdcd00ab62', 'consumption'),
('5828e634-94c1-5800-8160-4ecb1eff1a81', 'pensao-alimenticia', 'Pensão Alimentícia', 'expense', '66cb85a0-3266-5900-b8e3-13cdcd00ab62', 'consumption'),
('6a0d56cc-f9d8-5c95-be2a-60f8f69c912c', 'seguros-pessoais', 'Seguros Pessoais', 'expense', '66cb85a0-3266-5900-b8e3-13cdcd00ab62', 'consumption'),
('178d590e-bc16-5df3-a7c8-ec7c193896d5', 'assinaturas-essenciais', 'Assinaturas Essenciais', 'expense', '66cb85a0-3266-5900-b8e3-13cdcd00ab62', 'consumption'),
('347e0488-a4a7-55e8-8882-ae868c9d749d', 'tarifas-bancarias', 'Tarifas Bancárias', 'expense', '66cb85a0-3266-5900-b8e3-13cdcd00ab62', 'consumption'),
('7d56377d-bdd0-5152-9b94-10639bc7f39b', 'impostos-e-tributos', 'Impostos e Tributos', 'expense', '66cb85a0-3266-5900-b8e3-13cdcd00ab62', 'consumption'),
('b29895dd-f1c5-5375-a5d1-082d9e2c3620', 'emprestimos-e-financiamentos', 'Empréstimos e Financiamentos', 'expense', '66cb85a0-3266-5900-b8e3-13cdcd00ab62', 'consumption'),
('5b9a1cba-b400-508c-a615-a419d9b06dcf', 'dividas-e-juros', 'Dívidas e Juros', 'expense', '66cb85a0-3266-5900-b8e3-13cdcd00ab62', 'consumption'),
('3f7c80e0-820c-5766-ba50-826a6d82b8e6', 'manutencao-da-casa', 'Manutenção da Casa', 'expense', '66cb85a0-3266-5900-b8e3-13cdcd00ab62', 'consumption'),
('ca8e4a6c-58ae-5049-8c24-826bd471e896', 'servicos-domesticos', 'Serviços Domésticos', 'expense', '66cb85a0-3266-5900-b8e3-13cdcd00ab62', 'consumption'),
('d4b74050-db29-53e6-bcee-be5c333f8817', 'pets-recorrentes', 'Pets Recorrentes', 'expense', '66cb85a0-3266-5900-b8e3-13cdcd00ab62', 'consumption'),
('d1d7dbba-1e83-596c-a4e5-d520cd06c88a', 'outros-custos-fixos', 'Outros Custos Fixos', 'expense', '66cb85a0-3266-5900-b8e3-13cdcd00ab62', 'consumption')
ON CONFLICT (kind, slug) DO NOTHING;

INSERT INTO mecontrola.categories (id, slug, name, kind, parent_id, allocation_type) VALUES
('b3a4824f-e481-59fe-8f9e-0c33a59b5b5f', 'cursos-e-treinamentos', 'Cursos e Treinamentos', 'expense', '8314f021-ee9c-53b4-872f-449ac618da50', 'consumption'),
('01b51d39-347e-560c-ac07-d0a700f0c24f', 'plataformas-de-ensino', 'Plataformas de Ensino', 'expense', '8314f021-ee9c-53b4-872f-449ac618da50', 'consumption'),
('bac52783-54ca-5401-92da-5afa29fc05d4', 'livros-e-ebooks', 'Livros e E-books', 'expense', '8314f021-ee9c-53b4-872f-449ac618da50', 'consumption'),
('6f70f7d5-d319-5a97-a319-c864e7567285', 'material-de-estudo', 'Material de Estudo', 'expense', '8314f021-ee9c-53b4-872f-449ac618da50', 'consumption'),
('654552ab-829d-5b4d-b0ec-4cb1463454d7', 'certificacoes', 'Certificações', 'expense', '8314f021-ee9c-53b4-872f-449ac618da50', 'consumption'),
('3c5e9972-7f59-5f6b-aea4-ace59985cce0', 'congressos-e-workshops', 'Congressos e Workshops', 'expense', '8314f021-ee9c-53b4-872f-449ac618da50', 'consumption'),
('fec9aed9-2699-538e-bbae-eb4bcdfb1ce3', 'idiomas', 'Idiomas', 'expense', '8314f021-ee9c-53b4-872f-449ac618da50', 'consumption'),
('8d114d26-b1a4-5a5f-8995-194c088c7b3f', 'mentoria-e-coaching', 'Mentoria e Coaching', 'expense', '8314f021-ee9c-53b4-872f-449ac618da50', 'consumption'),
('ce2850ad-8d51-5224-b9a4-d884361e4639', 'aulas-particulares', 'Aulas Particulares', 'expense', '8314f021-ee9c-53b4-872f-449ac618da50', 'consumption'),
('4850d076-7dea-5b73-8d32-fff55765dd2f', 'software-e-ferramentas-de-estudo', 'Software e Ferramentas de Estudo', 'expense', '8314f021-ee9c-53b4-872f-449ac618da50', 'consumption'),
('ce233b45-2c19-536e-92bd-6b43958c9363', 'outros-conhecimentos', 'Outros Conhecimentos', 'expense', '8314f021-ee9c-53b4-872f-449ac618da50', 'consumption')
ON CONFLICT (kind, slug) DO NOTHING;

INSERT INTO mecontrola.categories (id, slug, name, kind, parent_id, allocation_type) VALUES
('ddbb0dc7-8b85-5177-8cfc-3bb2aed6c75c', 'delivery', 'Delivery', 'expense', 'ac535261-4060-56ef-b2e8-57c8cc7032d1', 'consumption'),
('d539672d-961f-5553-b807-0e0156a63163', 'restaurantes', 'Restaurantes', 'expense', 'ac535261-4060-56ef-b2e8-57c8cc7032d1', 'consumption'),
('a371851d-56cb-551d-addb-022575b8d6e9', 'bares-e-lanches', 'Bares e Lanches', 'expense', 'ac535261-4060-56ef-b2e8-57c8cc7032d1', 'consumption'),
('a20b4072-23b7-53e8-8d03-8146e0473218', 'cafeterias', 'Cafeterias', 'expense', 'ac535261-4060-56ef-b2e8-57c8cc7032d1', 'consumption'),
('85e56497-2e31-55d3-9516-376e61860708', 'streaming-de-video', 'Streaming de Vídeo', 'expense', 'ac535261-4060-56ef-b2e8-57c8cc7032d1', 'consumption'),
('8580a31d-041d-5fa4-b86e-af90108af0cb', 'musica-e-audio', 'Música e Áudio', 'expense', 'ac535261-4060-56ef-b2e8-57c8cc7032d1', 'consumption'),
('514c00a0-ca41-5798-85d0-39992fbc223c', 'games-e-assinaturas-de-jogos', 'Games e Assinaturas de Jogos', 'expense', 'ac535261-4060-56ef-b2e8-57c8cc7032d1', 'consumption'),
('5190df3d-8e6d-59bc-9e5b-d7e85e45154c', 'cinema-e-teatro', 'Cinema e Teatro', 'expense', 'ac535261-4060-56ef-b2e8-57c8cc7032d1', 'consumption'),
('09073cdd-4d58-5073-ae16-53ba2c3a4209', 'shows-e-eventos', 'Shows e Eventos', 'expense', 'ac535261-4060-56ef-b2e8-57c8cc7032d1', 'consumption'),
('aed45dcf-8fbe-5828-8fb6-87babd271d6c', 'passeios-e-parques', 'Passeios e Parques', 'expense', 'ac535261-4060-56ef-b2e8-57c8cc7032d1', 'consumption'),
('6feeb8fd-8faa-56d1-a0d0-d9d746e45f21', 'transporte-de-lazer', 'Transporte de Lazer', 'expense', 'ac535261-4060-56ef-b2e8-57c8cc7032d1', 'consumption'),
('0134668f-785b-5ac1-bcf5-e6c4f566de64', 'viagens-de-lazer', 'Viagens de Lazer', 'expense', 'ac535261-4060-56ef-b2e8-57c8cc7032d1', 'consumption'),
('7a69762f-6016-593a-9e62-f56f508ec9e1', 'hospedagem-de-lazer', 'Hospedagem de Lazer', 'expense', 'ac535261-4060-56ef-b2e8-57c8cc7032d1', 'consumption'),
('a2af4429-8e17-559f-bba4-f790c7732776', 'compras-pessoais', 'Compras Pessoais', 'expense', 'ac535261-4060-56ef-b2e8-57c8cc7032d1', 'consumption'),
('14416063-f271-53e2-8a58-6682461ec532', 'roupas-e-calcados', 'Roupas e Calçados', 'expense', 'ac535261-4060-56ef-b2e8-57c8cc7032d1', 'consumption'),
('f9656739-8d1c-5675-8eaf-63a057137307', 'beleza-e-estetica', 'Beleza e Estética', 'expense', 'ac535261-4060-56ef-b2e8-57c8cc7032d1', 'consumption'),
('671873dc-f403-5315-877c-d6d46d0f5a8f', 'hobbies', 'Hobbies', 'expense', 'ac535261-4060-56ef-b2e8-57c8cc7032d1', 'consumption'),
('c0e10d9f-b0fe-59e7-8fb9-22a3bebd4784', 'esportes-e-academia', 'Esportes e Academia', 'expense', 'ac535261-4060-56ef-b2e8-57c8cc7032d1', 'consumption'),
('09e7cd05-40bf-5100-92e9-439a7baf0c0c', 'presentes', 'Presentes', 'expense', 'ac535261-4060-56ef-b2e8-57c8cc7032d1', 'consumption'),
('481d2d82-a013-5991-8210-0bfcb44af4fa', 'pets-nao-recorrentes', 'Pets Não Recorrentes', 'expense', 'ac535261-4060-56ef-b2e8-57c8cc7032d1', 'consumption'),
('c2470946-ebf3-5baf-86cd-696b11baf497', 'doacoes', 'Doações', 'expense', 'ac535261-4060-56ef-b2e8-57c8cc7032d1', 'consumption'),
('0016763e-655c-571a-90cb-bec5a18d4969', 'outros-prazeres', 'Outros Prazeres', 'expense', 'ac535261-4060-56ef-b2e8-57c8cc7032d1', 'consumption')
ON CONFLICT (kind, slug) DO NOTHING;

INSERT INTO mecontrola.categories (id, slug, name, kind, parent_id, allocation_type) VALUES
('3ff5e6b5-b958-5848-9092-73eb541598fc', 'tecnologia', 'Tecnologia', 'expense', 'f133508e-7dc3-58a3-96db-199d8fbd2987', 'asset_allocation'),
('ef1a26ec-e12d-5b3c-b7ba-3634bb89647c', 'veiculo', 'Veículo', 'expense', 'f133508e-7dc3-58a3-96db-199d8fbd2987', 'asset_allocation'),
('61698c19-7281-5016-8cd3-b3799ddb575c', 'casa-e-reforma', 'Casa e Reforma', 'expense', 'f133508e-7dc3-58a3-96db-199d8fbd2987', 'asset_allocation'),
('8a4228f0-bc77-5d24-949d-5a7afa8063dc', 'viagem-planejada', 'Viagem Planejada', 'expense', 'f133508e-7dc3-58a3-96db-199d8fbd2987', 'asset_allocation'),
('6752f218-cbf9-5108-94e5-6732fdb6a0c6', 'casamento-e-festa', 'Casamento e Festa', 'expense', 'f133508e-7dc3-58a3-96db-199d8fbd2987', 'asset_allocation'),
('e91062ea-8bc9-5d30-a317-260faaf14e56', 'familia-e-enxoval', 'Família e Enxoval', 'expense', 'f133508e-7dc3-58a3-96db-199d8fbd2987', 'asset_allocation'),
('480b8f7d-6dc2-5d62-b154-669818123f65', 'empreendedorismo', 'Empreendedorismo', 'expense', 'f133508e-7dc3-58a3-96db-199d8fbd2987', 'asset_allocation'),
('1c178224-bd1b-51a0-bc6a-a8f12efa54c1', 'educacao-planejada', 'Educação Planejada', 'expense', 'f133508e-7dc3-58a3-96db-199d8fbd2987', 'asset_allocation'),
('568b9200-dae4-512c-a93c-192192d2ee4f', 'saude-planejada', 'Saúde Planejada', 'expense', 'f133508e-7dc3-58a3-96db-199d8fbd2987', 'asset_allocation'),
('946643a8-9e00-5bad-a860-f74ed74cf246', 'quitacao-de-dividas', 'Quitação de Dívidas', 'expense', 'f133508e-7dc3-58a3-96db-199d8fbd2987', 'asset_allocation'),
('ab070648-d71b-5920-a1dd-060f1f542371', 'compra-planejada', 'Compra Planejada', 'expense', 'f133508e-7dc3-58a3-96db-199d8fbd2987', 'asset_allocation'),
('8c1c3dd1-6b38-5b85-a37c-e7c9a769ff94', 'outras-metas', 'Outras Metas', 'expense', 'f133508e-7dc3-58a3-96db-199d8fbd2987', 'asset_allocation')
ON CONFLICT (kind, slug) DO NOTHING;

INSERT INTO mecontrola.categories (id, slug, name, kind, parent_id, allocation_type) VALUES
('45c7e533-fb00-50d9-aeb3-71bdb99098bd', 'reserva-de-emergencia', 'Reserva de Emergência', 'expense', '35ced21e-b436-5cea-afb9-ffd43f98a124', 'asset_allocation'),
('e79c7c54-c8c5-5b9f-9cbb-4bff3c98e429', 'reserva-de-oportunidade', 'Reserva de Oportunidade', 'expense', '35ced21e-b436-5cea-afb9-ffd43f98a124', 'asset_allocation'),
('9103a0e6-366b-5c77-a31d-e3ed58991d14', 'tesouro-direto', 'Tesouro Direto', 'expense', '35ced21e-b436-5cea-afb9-ffd43f98a124', 'asset_allocation'),
('ee26c4d9-ca74-5537-80b9-4d90815b9c06', 'cdb-e-rdb', 'CDB e RDB', 'expense', '35ced21e-b436-5cea-afb9-ffd43f98a124', 'asset_allocation'),
('d35da3b9-65c5-55b8-9915-13354e202644', 'lci-e-lca', 'LCI e LCA', 'expense', '35ced21e-b436-5cea-afb9-ffd43f98a124', 'asset_allocation'),
('b0fba509-bd7d-5f1c-9845-2288bee6c276', 'debentures-e-credito-privado', 'Debêntures e Crédito Privado', 'expense', '35ced21e-b436-5cea-afb9-ffd43f98a124', 'asset_allocation'),
('eb6c008a-2fe5-58bf-a879-a3a0d2ecf6cb', 'fundos-de-renda-fixa', 'Fundos de Renda Fixa', 'expense', '35ced21e-b436-5cea-afb9-ffd43f98a124', 'asset_allocation'),
('e1266272-eb97-5a9f-857d-e6b7b261cf9e', 'acoes', 'Ações', 'expense', '35ced21e-b436-5cea-afb9-ffd43f98a124', 'asset_allocation'),
('e917b351-60d6-53c5-ab5b-a92e663d700b', 'etfs', 'ETFs', 'expense', '35ced21e-b436-5cea-afb9-ffd43f98a124', 'asset_allocation'),
('1e5b4db2-b186-5524-b955-32553307d81c', 'fundos-imobiliarios', 'Fundos Imobiliários', 'expense', '35ced21e-b436-5cea-afb9-ffd43f98a124', 'asset_allocation'),
('abc00654-7b1d-5587-9de8-506710c42da4', 'bdrs', 'BDRs', 'expense', '35ced21e-b436-5cea-afb9-ffd43f98a124', 'asset_allocation'),
('e130a310-4a8d-5f0f-b050-405165e28966', 'fundos-de-investimento', 'Fundos de Investimento', 'expense', '35ced21e-b436-5cea-afb9-ffd43f98a124', 'asset_allocation'),
('b1ac9b12-0b4d-5791-87d5-6628c9bbfa9a', 'previdencia-privada', 'Previdência Privada', 'expense', '35ced21e-b436-5cea-afb9-ffd43f98a124', 'asset_allocation'),
('9747b1c4-f9dd-5565-ad6d-0f3476ebab9e', 'criptoativos', 'Criptoativos', 'expense', '35ced21e-b436-5cea-afb9-ffd43f98a124', 'asset_allocation'),
('da4f4c4c-864e-577b-9f4d-d7800f7a85ab', 'investimentos-internacionais', 'Investimentos Internacionais', 'expense', '35ced21e-b436-5cea-afb9-ffd43f98a124', 'asset_allocation'),
('866793cb-4059-54b0-9ee7-8f539ddebede', 'aportes-em-corretora', 'Aportes em Corretora', 'expense', '35ced21e-b436-5cea-afb9-ffd43f98a124', 'asset_allocation'),
('33191a7c-77d1-5fc8-bb8e-65268997cc65', 'outros-investimentos', 'Outros Investimentos', 'expense', '35ced21e-b436-5cea-afb9-ffd43f98a124', 'asset_allocation')
ON CONFLICT (kind, slug) DO NOTHING;

INSERT INTO mecontrola.categories (id, slug, name, kind, parent_id, allocation_type) VALUES
('86dd34b0-7342-525a-9a30-b1b5a76b109f', 'salario', 'Salário', 'income', NULL, 'consumption'),
('275ef473-b41d-5162-8488-0abf88a5e6f4', 'renda-variavel', 'Renda Variável', 'income', NULL, 'consumption'),
('1c801292-d1a0-56a9-8d05-a28f39f5e6dd', 'investimentos', 'Investimentos', 'income', NULL, 'asset_allocation'),
('6044ffc4-b869-598b-b7e9-8361ab7ee2f6', 'aluguel-recebido', 'Aluguel Recebido', 'income', NULL, 'consumption'),
('c0c8b110-d3de-5e7c-8080-0de827e67332', 'restituicoes-e-cashback', 'Restituições e Cashback', 'income', NULL, 'consumption'),
('be5c5726-10e7-5a39-b149-3ae784121cdd', 'presentes-recebidos', 'Presentes Recebidos', 'income', NULL, 'consumption'),
('8dba4d69-834f-5bdb-8c8c-9f86a9b56858', 'vendas', 'Vendas', 'income', NULL, 'consumption'),
('b01019ae-37b0-5dab-bc2e-3a000843c7bb', 'outras-receitas', 'Outras Receitas', 'income', NULL, 'consumption')
ON CONFLICT (kind, slug) DO NOTHING;

INSERT INTO mecontrola.categories (id, slug, name, kind, parent_id, allocation_type) VALUES
('98455e74-b1f3-5b9c-a8d8-05db0cdb465d', 'decimo-terceiro', 'Décimo Terceiro', 'income', '86dd34b0-7342-525a-9a30-b1b5a76b109f', 'consumption'),
('8f141d28-10c3-5a07-bfdf-4dfd79a049a1', 'ferias', 'Férias', 'income', '86dd34b0-7342-525a-9a30-b1b5a76b109f', 'consumption'),
('4b61504d-9cc2-579f-b927-d1963bd1e0ca', 'plr-e-bonus', 'PLR e Bônus', 'income', '86dd34b0-7342-525a-9a30-b1b5a76b109f', 'consumption'),
('bbc5809c-d567-59cf-80dd-e6f15b93b7e4', 'vale-alimentacao', 'Vale-Alimentação', 'income', '86dd34b0-7342-525a-9a30-b1b5a76b109f', 'consumption'),
('6e896533-af52-5938-bc38-2152ea443af8', 'vale-refeicao', 'Vale-Refeição', 'income', '86dd34b0-7342-525a-9a30-b1b5a76b109f', 'consumption')
ON CONFLICT (kind, slug) DO NOTHING;

INSERT INTO mecontrola.categories (id, slug, name, kind, parent_id, allocation_type) VALUES
('403192d5-5e85-54d3-a4b0-d4029e754c5c', 'freelance', 'Freelance', 'income', '275ef473-b41d-5162-8488-0abf88a5e6f4', 'consumption'),
('dc2303d9-246e-53d6-8448-2adc19993b22', 'trabalho-extra', 'Trabalho Extra', 'income', '275ef473-b41d-5162-8488-0abf88a5e6f4', 'consumption'),
('0d613676-5f32-5412-9408-fde944bed128', 'consultoria', 'Consultoria', 'income', '275ef473-b41d-5162-8488-0abf88a5e6f4', 'consumption')
ON CONFLICT (kind, slug) DO NOTHING;

INSERT INTO mecontrola.categories (id, slug, name, kind, parent_id, allocation_type) VALUES
('c8276187-8320-5be9-9519-8b6d2a4620b2', 'rendimentos', 'Rendimentos', 'income', '1c801292-d1a0-56a9-8d05-a28f39f5e6dd', 'asset_allocation'),
('5b983987-3b1d-5bd5-80d3-017416c3f0f8', 'dividendos', 'Dividendos', 'income', '1c801292-d1a0-56a9-8d05-a28f39f5e6dd', 'asset_allocation'),
('8d812f21-fe17-57e5-a71d-5eb890d29bb6', 'juros', 'Juros', 'income', '1c801292-d1a0-56a9-8d05-a28f39f5e6dd', 'asset_allocation'),
('cac84f1b-70c6-5eb1-81d9-241764043d66', 'resgates', 'Resgates', 'income', '1c801292-d1a0-56a9-8d05-a28f39f5e6dd', 'asset_allocation')
ON CONFLICT (kind, slug) DO NOTHING;

INSERT INTO mecontrola.categories (id, slug, name, kind, parent_id, allocation_type) VALUES
('00b886a7-d221-592e-8068-fa296924b333', 'aluguel-residencial-recebido', 'Aluguel Residencial Recebido', 'income', '6044ffc4-b869-598b-b7e9-8361ab7ee2f6', 'consumption')
ON CONFLICT (kind, slug) DO NOTHING;

INSERT INTO mecontrola.categories (id, slug, name, kind, parent_id, allocation_type) VALUES
('7a17fe1c-900c-57d8-a1dc-22bf9139cf83', 'restituicao-de-ir', 'Restituição de IR', 'income', 'c0c8b110-d3de-5e7c-8080-0de827e67332', 'consumption'),
('3791836d-bc96-57ae-87b4-fae12c1c111b', 'cashback', 'Cashback', 'income', 'c0c8b110-d3de-5e7c-8080-0de827e67332', 'consumption')
ON CONFLICT (kind, slug) DO NOTHING;

INSERT INTO mecontrola.categories (id, slug, name, kind, parent_id, allocation_type) VALUES
('06373332-7fb8-52b6-940e-f0c5699c6114', 'presentes-em-dinheiro', 'Presentes em Dinheiro', 'income', 'be5c5726-10e7-5a39-b149-3ae784121cdd', 'consumption'),
('1722bd29-031a-57d1-b4d8-2626d1971ce3', 'mesada-recebida', 'Mesada Recebida', 'income', 'be5c5726-10e7-5a39-b149-3ae784121cdd', 'consumption')
ON CONFLICT (kind, slug) DO NOTHING;

INSERT INTO mecontrola.categories (id, slug, name, kind, parent_id, allocation_type) VALUES
('6650a195-013b-5808-8845-22a0657da9ba', 'vendas-diversas', 'Vendas Diversas', 'income', '8dba4d69-834f-5bdb-8c8c-9f86a9b56858', 'consumption'),
('52ded4b8-b082-5ec2-90fe-633c934edae7', 'marketplace', 'Marketplace', 'income', '8dba4d69-834f-5bdb-8c8c-9f86a9b56858', 'consumption')
ON CONFLICT (kind, slug) DO NOTHING;

INSERT INTO mecontrola.categories (id, slug, name, kind, parent_id, allocation_type) VALUES
('9d996f66-81f2-5250-bb4d-bd3636e00544', 'outros', 'Outros', 'income', 'b01019ae-37b0-5dab-bc2e-3a000843c7bb', 'consumption')
ON CONFLICT (kind, slug) DO NOTHING;

UPDATE mecontrola.category_editorial_version SET version = 2, updated_at = now()
WHERE version = 1;

-- Consolidated from migrations/000006_seed_dictionary.up.sql
SET LOCAL lock_timeout    = '5s';
SET LOCAL statement_timeout = '120s';

DO $$
DECLARE
    v_version BIGINT;
BEGIN
    SELECT version INTO v_version FROM mecontrola.category_editorial_version;
    IF v_version IS NULL THEN
        RAISE EXCEPTION 'category_editorial_version não inicializada';
    END IF;
END $$;

INSERT INTO mecontrola.category_dictionary (id, category_id, kind, term, signal_type, confidence, is_ambiguous) VALUES
    -- Custo Fixo - canonical names (IDs são os mesmos das categorias)
    ('c2fda6a3-c329-52c8-81ea-771b6ea4f365', 'c2fda6a3-c329-52c8-81ea-771b6ea4f365', 'expense', 'aluguel', 'canonical_name', 'high', false),
    ('f9d9e5b6-1437-5204-bd64-2bd7d43583a8', 'f9d9e5b6-1437-5204-bd64-2bd7d43583a8', 'expense', 'financiamento-imobiliario', 'canonical_name', 'high', false),
    ('d0b1fa13-d19f-51b9-afc7-82bf83accf79', 'd0b1fa13-d19f-51b9-afc7-82bf83accf79', 'expense', 'condominio', 'canonical_name', 'high', false),
    ('80a870e9-831f-5e85-b95f-0afe2f8d372a', '80a870e9-831f-5e85-b95f-0afe2f8d372a', 'expense', 'iptu', 'canonical_name', 'high', false),
    ('36916fab-eacc-50a3-8a53-93671c335952', '36916fab-eacc-50a3-8a53-93671c335952', 'expense', 'energia', 'canonical_name', 'high', false),
    ('fa93273d-e2d9-54ed-a6aa-53b5b1830867', 'fa93273d-e2d9-54ed-a6aa-53b5b1830867', 'expense', 'agua', 'canonical_name', 'high', false),
    ('4e6f8b6b-8ffb-5d38-8ac9-68464679a544', '4e6f8b6b-8ffb-5d38-8ac9-68464679a544', 'expense', 'gas', 'canonical_name', 'high', false),
    ('9391ac38-ec2c-55d0-afc8-8c0940678814', '9391ac38-ec2c-55d0-afc8-8c0940678814', 'expense', 'internet', 'canonical_name', 'high', false),
    ('7319ba14-0dc7-56ff-ac5c-96024e15ec02', '7319ba14-0dc7-56ff-ac5c-96024e15ec02', 'expense', 'telefonia', 'canonical_name', 'high', false),
    ('97fa4b86-d43c-5ad5-a99b-c88c8427fb30', '97fa4b86-d43c-5ad5-a99b-c88c8427fb30', 'expense', 'supermercado', 'canonical_name', 'high', false),
    ('0c004f2d-ad42-5855-a408-f695906cd48c', '0c004f2d-ad42-5855-a408-f695906cd48c', 'expense', 'feira-e-hortifruti', 'canonical_name', 'high', false),
    ('75af9f6b-78e4-5ef3-b6ca-b84a37f8901c', '75af9f6b-78e4-5ef3-b6ca-b84a37f8901c', 'expense', 'acougue', 'canonical_name', 'high', false),
    ('007c090e-7a6d-5645-b751-b93cabb280ed', '007c090e-7a6d-5645-b751-b93cabb280ed', 'expense', 'transporte-publico', 'canonical_name', 'high', false),
    ('c13dcc6e-c37b-521d-a889-8bb02765490f', 'c13dcc6e-c37b-521d-a889-8bb02765490f', 'expense', 'transporte-por-aplicativo-recorrente', 'canonical_name', 'high', false),
    ('c13dcc6e-c37b-521d-a889-8bb02765491a', 'c13dcc6e-c37b-521d-a889-8bb02765490f', 'expense', 'uber', 'merchant', 'medium', true),
    ('c13dcc6e-c37b-521d-a889-8bb02765491b', 'c13dcc6e-c37b-521d-a889-8bb02765490f', 'expense', '99', 'merchant', 'medium', true),
    ('6feeb8fd-8faa-56d1-a0d0-d9d746e45f21', '6feeb8fd-8faa-56d1-a0d0-d9d746e45f21', 'expense', 'transporte-de-lazer', 'canonical_name', 'high', false),
    ('6feeb8fd-8faa-56d1-a0d0-d9d746e45f2a', '6feeb8fd-8faa-56d1-a0d0-d9d746e45f21', 'expense', 'uber', 'merchant', 'medium', true),
    ('6feeb8fd-8faa-56d1-a0d0-d9d746e45f2b', '6feeb8fd-8faa-56d1-a0d0-d9d746e45f21', 'expense', '99', 'merchant', 'medium', true),
    ('cb13d50d-43cb-553c-99cd-8851889d7f6e', 'cb13d50d-43cb-553c-99cd-8851889d7f6e', 'expense', 'combustivel', 'canonical_name', 'high', false),
    ('bf2fcca0-09c3-5dcb-a61a-87eed2860c04', 'bf2fcca0-09c3-5dcb-a61a-87eed2860c04', 'expense', 'manutencao-veicular', 'canonical_name', 'high', false),
    ('311c7b7f-56a3-5b53-ada7-5b85734ba45f', '311c7b7f-56a3-5b53-ada7-5b85734ba45f', 'expense', 'ipva-e-licenciamento', 'canonical_name', 'high', false),
    ('c8f579ea-952b-5e24-beed-ef22fb845a4b', 'c8f579ea-952b-5e24-beed-ef22fb845a4b', 'expense', 'plano-de-saude', 'canonical_name', 'high', false),
    ('1af66343-7305-534f-b8de-47ebcd3d17f1', '1af66343-7305-534f-b8de-47ebcd3d17f1', 'expense', 'plano-odontologico', 'canonical_name', 'high', false),
    ('af5619e0-3683-5b8c-b9fc-0b3ddfbd2075', 'af5619e0-3683-5b8c-b9fc-0b3ddfbd2075', 'expense', 'consultas-e-exames', 'canonical_name', 'high', false),
    ('157b18fe-513e-55fa-969c-c9bd785530d1', '157b18fe-513e-55fa-969c-c9bd785530d1', 'expense', 'medicamentos-continuos', 'canonical_name', 'high', false),
    ('3ca95dd5-c630-5c03-bd47-071777bde81c', '3ca95dd5-c630-5c03-bd47-071777bde81c', 'expense', 'medicamentos-e-farmacia', 'canonical_name', 'high', false),
    ('4ded7fd4-5335-5cf2-aed1-bdcead596000', '4ded7fd4-5335-5cf2-aed1-bdcead596000', 'expense', 'odontologia', 'canonical_name', 'high', false),
    ('a15cba16-23da-504e-a22b-144392ed82bc', 'a15cba16-23da-504e-a22b-144392ed82bc', 'expense', 'terapia-e-saude-mental', 'canonical_name', 'high', false),
    ('cab69263-ac14-5ed1-ab5d-8372487c9ee8', 'cab69263-ac14-5ed1-ab5d-8372487c9ee8', 'expense', 'escola-e-creche', 'canonical_name', 'high', false),
    ('46e492a0-3909-5e0a-bd3e-16bbdf29db8d', '46e492a0-3909-5e0a-bd3e-16bbdf29db8d', 'expense', 'faculdade-e-pos-graduacao', 'canonical_name', 'high', false),
    -- Conhecimento
    ('b3a4824f-e481-59fe-8f9e-0c33a59b5b5f', 'b3a4824f-e481-59fe-8f9e-0c33a59b5b5f', 'expense', 'cursos-e-treinamentos', 'canonical_name', 'high', false),
    ('01b51d39-347e-560c-ac07-d0a700f0c24f', '01b51d39-347e-560c-ac07-d0a700f0c24f', 'expense', 'plataformas-de-ensino', 'canonical_name', 'high', false),
    ('bac52783-54ca-5401-92da-5afa29fc05d4', 'bac52783-54ca-5401-92da-5afa29fc05d4', 'expense', 'livros-e-ebooks', 'canonical_name', 'high', false),
    ('654552ab-829d-5b4d-b0ec-4cb1463454d7', '654552ab-829d-5b4d-b0ec-4cb1463454d7', 'expense', 'certificacoes', 'canonical_name', 'high', false),
    ('3c5e9972-7f59-5f6b-aea4-ace59985cce0', '3c5e9972-7f59-5f6b-aea4-ace59985cce0', 'expense', 'congressos-e-workshops', 'canonical_name', 'high', false),
    ('fec9aed9-2699-538e-bbae-eb4bcdfb1ce3', 'fec9aed9-2699-538e-bbae-eb4bcdfb1ce3', 'expense', 'idiomas', 'canonical_name', 'high', false),
    -- Prazeres
    ('ddbb0dc7-8b85-5177-8cfc-3bb2aed6c75c', 'ddbb0dc7-8b85-5177-8cfc-3bb2aed6c75c', 'expense', 'delivery', 'canonical_name', 'high', false),
    ('d539672d-961f-5553-b807-0e0156a63163', 'd539672d-961f-5553-b807-0e0156a63163', 'expense', 'restaurantes', 'canonical_name', 'high', false),
    ('a371851d-56cb-551d-addb-022575b8d6e9', 'a371851d-56cb-551d-addb-022575b8d6e9', 'expense', 'bares-e-lanches', 'canonical_name', 'high', false),
    ('a20b4072-23b7-53e8-8d03-8146e0473218', 'a20b4072-23b7-53e8-8d03-8146e0473218', 'expense', 'cafeterias', 'canonical_name', 'high', false),
    ('85e56497-2e31-55d3-9516-376e61860708', '85e56497-2e31-55d3-9516-376e61860708', 'expense', 'streaming-de-video', 'canonical_name', 'high', false),
    ('8580a31d-041d-5fa4-b86e-af90108af0cb', '8580a31d-041d-5fa4-b86e-af90108af0cb', 'expense', 'musica-e-audio', 'canonical_name', 'high', false),
    ('514c00a0-ca41-5798-85d0-39992fbc223c', '514c00a0-ca41-5798-85d0-39992fbc223c', 'expense', 'games-e-assinaturas-de-jogos', 'canonical_name', 'high', false),
    ('5190df3d-8e6d-59bc-9e5b-d7e85e45154c', '5190df3d-8e6d-59bc-9e5b-d7e85e45154c', 'expense', 'cinema-e-teatro', 'canonical_name', 'high', false),
    ('09073cdd-4d58-5073-ae16-53ba2c3a4209', '09073cdd-4d58-5073-ae16-53ba2c3a4209', 'expense', 'shows-e-eventos', 'canonical_name', 'high', false),
    ('aed45dcf-8fbe-5828-8fb6-87babd271d6c', 'aed45dcf-8fbe-5828-8fb6-87babd271d6c', 'expense', 'passeios-e-parques', 'canonical_name', 'high', false),
    ('0134668f-785b-5ac1-bcf5-e6c4f566de64', '0134668f-785b-5ac1-bcf5-e6c4f566de64', 'expense', 'viagens-de-lazer', 'canonical_name', 'high', false),
    ('7a69762f-6016-593a-9e62-f56f508ec9e1', '7a69762f-6016-593a-9e62-f56f508ec9e1', 'expense', 'hospedagem-de-lazer', 'canonical_name', 'high', false),
    ('14416063-f271-53e2-8a58-6682461ec532', '14416063-f271-53e2-8a58-6682461ec532', 'expense', 'roupas-e-calcados', 'canonical_name', 'high', false),
    ('f9656739-8d1c-5675-8eaf-63a057137307', 'f9656739-8d1c-5675-8eaf-63a057137307', 'expense', 'beleza-e-estetica', 'canonical_name', 'high', false),
    ('671873dc-f403-5315-877c-d6d46d0f5a8f', '671873dc-f403-5315-877c-d6d46d0f5a8f', 'expense', 'hobbies', 'canonical_name', 'high', false),
    ('c0e10d9f-b0fe-59e7-8fb9-22a3bebd4784', 'c0e10d9f-b0fe-59e7-8fb9-22a3bebd4784', 'expense', 'esportes-e-academia', 'canonical_name', 'high', false),
    -- Metas
    ('3ff5e6b5-b958-5848-9092-73eb541598fc', '3ff5e6b5-b958-5848-9092-73eb541598fc', 'expense', 'tecnologia', 'canonical_name', 'high', false),
    ('ef1a26ec-e12d-5b3c-b7ba-3634bb89647c', 'ef1a26ec-e12d-5b3c-b7ba-3634bb89647c', 'expense', 'veiculo', 'canonical_name', 'high', false),
    ('61698c19-7281-5016-8cd3-b3799ddb575c', '61698c19-7281-5016-8cd3-b3799ddb575c', 'expense', 'casa-e-reforma', 'canonical_name', 'high', false),
    ('8a4228f0-bc77-5d24-949d-5a7afa8063dc', '8a4228f0-bc77-5d24-949d-5a7afa8063dc', 'expense', 'viagem-planejada', 'canonical_name', 'high', false),
    ('e91062ea-8bc9-5d30-a317-260faaf14e56', 'e91062ea-8bc9-5d30-a317-260faaf14e56', 'expense', 'familia-e-enxoval', 'canonical_name', 'high', false),
    ('480b8f7d-6dc2-5d62-b154-669818123f65', '480b8f7d-6dc2-5d62-b154-669818123f65', 'expense', 'empreendedorismo', 'canonical_name', 'high', false),
    ('946643a8-9e00-5bad-a860-f74ed74cf246', '946643a8-9e00-5bad-a860-f74ed74cf246', 'expense', 'quitacao-de-dividas', 'canonical_name', 'high', false),
    -- Liberdade Financeira
    ('45c7e533-fb00-50d9-aeb3-71bdb99098bd', '45c7e533-fb00-50d9-aeb3-71bdb99098bd', 'expense', 'reserva-de-emergencia', 'canonical_name', 'high', false),
    ('9103a0e6-366b-5c77-a31d-e3ed58991d14', '9103a0e6-366b-5c77-a31d-e3ed58991d14', 'expense', 'tesouro-direto', 'canonical_name', 'high', false),
    ('1e5b4db2-b186-5524-b955-32553307d81c', '1e5b4db2-b186-5524-b955-32553307d81c', 'expense', 'fundos-imobiliarios', 'canonical_name', 'high', false),
    ('b1ac9b12-0b4d-5791-87d5-6628c9bbfa9a', 'b1ac9b12-0b4d-5791-87d5-6628c9bbfa9a', 'expense', 'previdencia-privada', 'canonical_name', 'high', false),
    ('9747b1c4-f9dd-5565-ad6d-0f3476ebab9e', '9747b1c4-f9dd-5565-ad6d-0f3476ebab9e', 'expense', 'criptoativos', 'canonical_name', 'high', false),
    ('da4f4c4c-864e-577b-9f4d-d7800f7a85ab', 'da4f4c4c-864e-577b-9f4d-d7800f7a85ab', 'expense', 'investimentos-internacionais', 'canonical_name', 'high', false),
    -- Receitas
    ('98455e74-b1f3-5b9c-a8d8-05db0cdb465d', '98455e74-b1f3-5b9c-a8d8-05db0cdb465d', 'income', 'decimo-terceiro', 'canonical_name', 'high', false),
    ('98455e74-b1f3-5b9c-a8d8-05db0cdb465a', '98455e74-b1f3-5b9c-a8d8-05db0cdb465d', 'income', '13 salario', 'alias', 'high', false),
    ('98455e74-b1f3-5b9c-a8d8-05db0cdb465e', '98455e74-b1f3-5b9c-a8d8-05db0cdb465d', 'income', '13º salário', 'alias', 'high', false),
    ('98455e74-b1f3-5b9c-a8d8-05db0cdb465b', '98455e74-b1f3-5b9c-a8d8-05db0cdb465d', 'income', 'decimo terceiro salario', 'alias', 'high', false),
    ('4b61504d-9cc2-579f-b927-d1963bd1e0ca', '4b61504d-9cc2-579f-b927-d1963bd1e0ca', 'income', 'plr-e-bonus', 'canonical_name', 'high', false),
    ('403192d5-5e85-54d3-a4b0-d4029e754c5c', '403192d5-5e85-54d3-a4b0-d4029e754c5c', 'income', 'freelance', 'canonical_name', 'high', false),
    ('c8276187-8320-5be9-9519-8b6d2a4620b2', 'c8276187-8320-5be9-9519-8b6d2a4620b2', 'income', 'rendimentos', 'canonical_name', 'high', false),
    ('5b983987-3b1d-5bd5-80d3-017416c3f0f8', '5b983987-3b1d-5bd5-80d3-017416c3f0f8', 'income', 'dividendos', 'canonical_name', 'high', false),
    ('00b886a7-d221-592e-8068-fa296924b333', '00b886a7-d221-592e-8068-fa296924b333', 'income', 'aluguel-residencial-recebido', 'canonical_name', 'high', false),
    ('7a17fe1c-900c-57d8-a1dc-22bf9139cf83', '7a17fe1c-900c-57d8-a1dc-22bf9139cf83', 'income', 'restituicao-de-ir', 'canonical_name', 'high', false),
    ('3791836d-bc96-57ae-87b4-fae12c1c111b', '3791836d-bc96-57ae-87b4-fae12c1c111b', 'income', 'cashback', 'canonical_name', 'high', false),
    ('06373332-7fb8-52b6-940e-f0c5699c6114', '06373332-7fb8-52b6-940e-f0c5699c6114', 'income', 'presentes-em-dinheiro', 'canonical_name', 'high', false),
    ('6650a195-013b-5808-8845-22a0657da9ba', '6650a195-013b-5808-8845-22a0657da9ba', 'income', 'vendas-diversas', 'canonical_name', 'high', false)
ON CONFLICT (id) DO UPDATE SET
    category_id = EXCLUDED.category_id,
    kind = EXCLUDED.kind,
    term = EXCLUDED.term,
    signal_type = EXCLUDED.signal_type,
    confidence = EXCLUDED.confidence,
    is_ambiguous = EXCLUDED.is_ambiguous,
    deprecated_at = NULL
WHERE mecontrola.category_dictionary.deprecated_at IS NOT NULL;

UPDATE mecontrola.category_editorial_version
SET version = version + 1, updated_at = now();

-- Consolidated from migrations/000009_seed_dictionary_aliases.up.sql
SET LOCAL lock_timeout    = '5s';
SET LOCAL statement_timeout = '120s';

DO $$
DECLARE
    v_version BIGINT;
BEGIN
    SELECT version INTO v_version FROM mecontrola.category_editorial_version;
    IF v_version IS NULL THEN
        RAISE EXCEPTION 'category_editorial_version não inicializada';
    END IF;
END $$;

INSERT INTO mecontrola.category_dictionary (id, category_id, kind, term, signal_type, confidence, is_ambiguous) VALUES
    ('35250296-2645-5dee-9ed1-c6a701163d4e', 'c2fda6a3-c329-52c8-81ea-771b6ea4f365', 'expense', 'alug', 'alias', 'high', false),
    ('f8c5df06-0db9-52dc-a37c-ff290686066a', 'c2fda6a3-c329-52c8-81ea-771b6ea4f365', 'expense', 'locação residencial', 'phrase', 'high', false),
    ('c68e9718-6734-520b-a551-e6aead486ee6', 'c2fda6a3-c329-52c8-81ea-771b6ea4f365', 'expense', 'aluguel da casa', 'phrase', 'high', false),
    ('82096212-e53a-59b6-b245-725894731078', 'c2fda6a3-c329-52c8-81ea-771b6ea4f365', 'expense', 'aluguel do apartamento', 'phrase', 'high', false),
    ('f57f0b6d-4cb0-5291-8f0b-a28fc4d98fc1', 'f9d9e5b6-1437-5204-bd64-2bd7d43583a8', 'expense', 'financiamento do imóvel', 'phrase', 'high', false),
    ('98e4a871-a8da-5ab0-afe1-db7abf41e885', 'f9d9e5b6-1437-5204-bd64-2bd7d43583a8', 'expense', 'parcela da casa', 'phrase', 'high', false),
    ('ae38983f-c339-593a-b42e-5d265a3e16e6', 'f9d9e5b6-1437-5204-bd64-2bd7d43583a8', 'expense', 'prestação do imóvel', 'phrase', 'high', false),
    ('4efe8568-40ba-54cc-a94e-98be709fb500', 'd0b1fa13-d19f-51b9-afc7-82bf83accf79', 'expense', 'taxa condominial', 'phrase', 'high', false),
    ('75695367-6c4e-52ea-ad2d-15378d24e902', 'd0b1fa13-d19f-51b9-afc7-82bf83accf79', 'expense', 'boleto do condomínio', 'phrase', 'high', false),
    ('8cd5189a-7bef-55c7-8f2e-6e667ab6e9c9', '80a870e9-831f-5e85-b95f-0afe2f8d372a', 'expense', 'imposto predial', 'phrase', 'high', false),
    ('e2bc0b74-1114-55dc-86e5-f90b6e4d8a17', '36916fab-eacc-50a3-8a53-93671c335952', 'expense', 'conta de luz', 'phrase', 'high', false),
    ('6ee61fda-5328-56b7-a7e9-841a289dd6c5', '36916fab-eacc-50a3-8a53-93671c335952', 'expense', 'eletricidade', 'alias', 'high', false),
    ('f7f3bd2b-9c94-5f02-ba60-659022c11a47', 'fa93273d-e2d9-54ed-a6aa-53b5b1830867', 'expense', 'conta de água', 'phrase', 'high', false),
    ('538188e1-0726-5423-9841-2f5d091069d0', 'fa93273d-e2d9-54ed-a6aa-53b5b1830867', 'expense', 'saneamento', 'alias', 'high', false),
    ('b49bcb07-f23d-5ff0-8e78-4021f5325ab3', '4e6f8b6b-8ffb-5d38-8ac9-68464679a544', 'expense', 'botijão', 'alias', 'high', false),
    ('87c3ce9f-02cf-5b17-bd58-d3ce03f9e576', '4e6f8b6b-8ffb-5d38-8ac9-68464679a544', 'expense', 'gás encanado', 'phrase', 'high', false),
    ('14c6084b-91e2-5414-8c0a-8308fd764666', '4e6f8b6b-8ffb-5d38-8ac9-68464679a544', 'expense', 'conta de gás', 'phrase', 'high', false),
    ('c81e5a59-a3b7-5572-9e1e-e283cbe85516', '9391ac38-ec2c-55d0-afc8-8c0940678814', 'expense', 'banda larga', 'alias', 'high', false),
    ('b68b6564-36db-5ecd-89d0-e0a5504cc337', '9391ac38-ec2c-55d0-afc8-8c0940678814', 'expense', 'fibra', 'alias', 'high', false),
    ('dd100c97-18be-5ac5-bc83-762fd997c98a', '9391ac38-ec2c-55d0-afc8-8c0940678814', 'expense', 'wi-fi residencial', 'phrase', 'high', false),
    ('242e8a7f-e3a7-57eb-966e-93f94a6f7f63', '7319ba14-0dc7-56ff-ac5c-96024e15ec02', 'expense', 'plano móvel', 'phrase', 'high', false),
    ('85bf1124-cef4-5ab1-900d-780ace3db722', '7319ba14-0dc7-56ff-ac5c-96024e15ec02', 'expense', 'recarga de celular', 'phrase', 'high', false),
    ('6874b7f5-d1cf-5a42-b71c-06300529b83f', '97fa4b86-d43c-5ad5-a99b-c88c8427fb30', 'expense', 'compras do mês', 'phrase', 'high', false),
    ('d608e341-be85-5f0a-b7a1-329f9d234fa1', '97fa4b86-d43c-5ad5-a99b-c88c8427fb30', 'expense', 'mercearia', 'alias', 'high', false),
    ('df20316f-710c-56c2-bf17-831f7e0a4066', '0c004f2d-ad42-5855-a408-f695906cd48c', 'expense', 'feira', 'alias', 'high', false),
    ('66ffd868-073a-5393-942c-5878d7433dad', '0c004f2d-ad42-5855-a408-f695906cd48c', 'expense', 'hortifruti', 'alias', 'high', false),
    ('6708d4c0-ecf1-5b4e-a5d1-8b13bb0bff34', '0c004f2d-ad42-5855-a408-f695906cd48c', 'expense', 'sacolão', 'alias', 'high', false),
    ('bd8a39cb-31c6-58f7-abab-38e340e6d8c5', '0c004f2d-ad42-5855-a408-f695906cd48c', 'expense', 'frutas e verduras', 'phrase', 'high', false),
    ('abeddb3d-6c5c-556b-abbd-e5453a5c687b', '75af9f6b-78e4-5ef3-b6ca-b84a37f8901c', 'expense', 'carnes', 'alias', 'high', false),
    ('ab5bfaec-ddac-5fae-8cdd-5dcf7d00b417', '75af9f6b-78e4-5ef3-b6ca-b84a37f8901c', 'expense', 'frigorífico', 'alias', 'high', false),
    ('015f36fc-8a34-5d1e-b0cc-91c01c40e82b', '007c090e-7a6d-5645-b751-b93cabb280ed', 'expense', 'ônibus', 'alias', 'high', false),
    ('5f781b6b-28d3-56c8-a854-a3d63094aa98', '007c090e-7a6d-5645-b751-b93cabb280ed', 'expense', 'metrô', 'alias', 'high', false),
    ('f7c0557a-c970-591f-9f28-be830fb59726', '007c090e-7a6d-5645-b751-b93cabb280ed', 'expense', 'trem', 'alias', 'high', false),
    ('fef05a23-363b-5ab7-aad3-c0f126ed4062', '007c090e-7a6d-5645-b751-b93cabb280ed', 'expense', 'bilhete único', 'phrase', 'high', false),
    ('cacfba1c-8710-54b4-8d9d-3f4331ffb6df', '007c090e-7a6d-5645-b751-b93cabb280ed', 'expense', 'passagem urbana', 'phrase', 'high', false),
    ('a6f5b373-f558-515b-bce5-80491da4f089', 'c13dcc6e-c37b-521d-a889-8bb02765490f', 'expense', 'corrida para o trabalho', 'phrase', 'high', false),
    ('f32908c1-c635-51e6-b835-c966f28bbe73', 'c13dcc6e-c37b-521d-a889-8bb02765490f', 'expense', 'uber para o trabalho', 'phrase', 'high', false),
    ('5b283b00-4b65-5684-9b3d-4aba6718f1b0', 'c13dcc6e-c37b-521d-a889-8bb02765490f', 'expense', '99 para o trabalho', 'phrase', 'high', false),
    ('8f9709a2-2255-59d8-997e-9f7a76458884', 'cb13d50d-43cb-553c-99cd-8851889d7f6e', 'expense', 'gasolina', 'alias', 'high', false),
    ('59e31379-90ca-5a7c-9a06-fbb0bf53a3f6', 'cb13d50d-43cb-553c-99cd-8851889d7f6e', 'expense', 'etanol', 'alias', 'high', false),
    ('bb945bb4-a3d9-596f-bbb0-e6fe9b7e036b', 'cb13d50d-43cb-553c-99cd-8851889d7f6e', 'expense', 'diesel', 'alias', 'high', false),
    ('510e97c1-f1fb-57b9-a1ec-5958ef0c4598', 'bf2fcca0-09c3-5dcb-a61a-87eed2860c04', 'expense', 'oficina', 'alias', 'high', false),
    ('60d30654-bf53-5bc9-8820-fbc21af20894', 'bf2fcca0-09c3-5dcb-a61a-87eed2860c04', 'expense', 'revisão do carro', 'phrase', 'high', false),
    ('6a17d9ca-b495-5405-8554-6d5d10762a39', 'bf2fcca0-09c3-5dcb-a61a-87eed2860c04', 'expense', 'troca de óleo', 'phrase', 'high', false),
    ('ffd6b457-8104-55f5-a077-d870340bbaf1', 'bf2fcca0-09c3-5dcb-a61a-87eed2860c04', 'expense', 'borracharia', 'alias', 'high', false),
    ('ccd71b21-0454-5586-b2bb-44c190031fd0', '311c7b7f-56a3-5b53-ada7-5b85734ba45f', 'expense', 'ipva', 'alias', 'high', false),
    ('0e979eb1-71f1-548a-a4f2-b1b0e96c1d98', '311c7b7f-56a3-5b53-ada7-5b85734ba45f', 'expense', 'licenciamento', 'alias', 'high', false),
    ('15bcac4d-f4a5-58e0-82ae-a1c5cef50107', '311c7b7f-56a3-5b53-ada7-5b85734ba45f', 'expense', 'documento do carro', 'phrase', 'high', false),
    ('cf2562d8-7c7e-5726-b070-a96d54f9913e', 'c8f579ea-952b-5e24-beed-ef22fb845a4b', 'expense', 'convênio médico', 'phrase', 'high', false),
    ('11887fa8-029b-5d61-b39f-f58f5e810c23', '1af66343-7305-534f-b8de-47ebcd3d17f1', 'expense', 'convênio odontológico', 'phrase', 'high', false),
    ('ae6f3cdc-75d7-541b-be84-76915ffade32', 'af5619e0-3683-5b8c-b9fc-0b3ddfbd2075', 'expense', 'consulta médica', 'phrase', 'high', false),
    ('01dd7092-6a0f-57e9-a429-71d29ef078af', 'af5619e0-3683-5b8c-b9fc-0b3ddfbd2075', 'expense', 'exame médico', 'phrase', 'high', false),
    ('d8b843fa-5e99-53b0-a3ac-db4d2eb554c1', 'af5619e0-3683-5b8c-b9fc-0b3ddfbd2075', 'expense', 'laboratório', 'alias', 'high', false),
    ('8f81c90a-6699-537e-b29d-7e489a4c3176', 'af5619e0-3683-5b8c-b9fc-0b3ddfbd2075', 'expense', 'diagnóstico por imagem', 'phrase', 'high', false),
    ('0ddf54e0-e759-509d-825f-980f8949e4ab', '157b18fe-513e-55fa-969c-c9bd785530d1', 'expense', 'remédio contínuo', 'phrase', 'high', false),
    ('c93494c1-7fc3-59a6-bbf2-a4d1e0adc719', '157b18fe-513e-55fa-969c-c9bd785530d1', 'expense', 'medicamento contínuo', 'phrase', 'high', false),
    ('2e3cfb8f-fb3d-5250-a4e9-b0c989f068b2', '157b18fe-513e-55fa-969c-c9bd785530d1', 'expense', 'remédio de uso contínuo', 'phrase', 'high', false),
    ('beeeaea5-6578-5503-875c-0f7310a6ec50', '3ca95dd5-c630-5c03-bd47-071777bde81c', 'expense', 'drogaria', 'alias', 'high', false),
    ('45bcf50b-06a2-59b2-95ef-22763e03026b', '4ded7fd4-5335-5cf2-aed1-bdcead596000', 'expense', 'dentista', 'alias', 'high', false),
    ('ea35570c-5b36-579a-a0d1-bf037048660c', '4ded7fd4-5335-5cf2-aed1-bdcead596000', 'expense', 'consulta odontológica', 'phrase', 'high', false),
    ('293887e9-340c-5576-9dc4-acfd1ff6b4e3', '4ded7fd4-5335-5cf2-aed1-bdcead596000', 'expense', 'tratamento dentário', 'phrase', 'high', false),
    ('53f5b902-21e3-5c4b-9069-5b8044f712e1', 'a15cba16-23da-504e-a22b-144392ed82bc', 'expense', 'terapia', 'alias', 'high', false),
    ('05431c9e-5ebe-5c04-8444-2d66746420eb', 'a15cba16-23da-504e-a22b-144392ed82bc', 'expense', 'psicólogo', 'alias', 'high', false),
    ('d8e28ecd-264f-50ef-8edf-5046d01fa295', 'a15cba16-23da-504e-a22b-144392ed82bc', 'expense', 'psicóloga', 'alias', 'high', false),
    ('59d04f65-c11a-5c7c-a05b-6fdd05abac9c', 'a15cba16-23da-504e-a22b-144392ed82bc', 'expense', 'psiquiatra', 'alias', 'high', false),
    ('eef8844a-6d0d-596b-a233-bace2fd67f73', 'cab69263-ac14-5ed1-ab5d-8372487c9ee8', 'expense', 'mensalidade escolar', 'phrase', 'high', false),
    ('1e2cd4fb-0ef5-5e8d-92fe-fdfbf816bd3a', '46e492a0-3909-5e0a-bd3e-16bbdf29db8d', 'expense', 'universidade', 'alias', 'high', false),
    ('6fca496b-c9c8-5950-82c7-ffcbf3ecc840', '46e492a0-3909-5e0a-bd3e-16bbdf29db8d', 'expense', 'pós-graduação', 'phrase', 'high', false),
    ('c6d85fee-e170-5611-a43f-198d5f4dcba8', '46e492a0-3909-5e0a-bd3e-16bbdf29db8d', 'expense', 'mensalidade da faculdade', 'phrase', 'high', false),
    ('d39febb5-82ce-5f2a-b73d-c37feeda54fb', 'b3a4824f-e481-59fe-8f9e-0c33a59b5b5f', 'expense', 'treinamento', 'alias', 'high', false),
    ('9cc2a8b8-c865-5d19-8de6-a4f35d99d00e', 'b3a4824f-e481-59fe-8f9e-0c33a59b5b5f', 'expense', 'formação', 'alias', 'high', false),
    ('fbf7fc0c-f68a-5f86-a94e-a568c4dfdda1', 'b3a4824f-e481-59fe-8f9e-0c33a59b5b5f', 'expense', 'bootcamp', 'alias', 'high', false),
    ('8997ab39-0257-52cf-8a6b-c1406ce01cd5', '01b51d39-347e-560c-ac07-d0a700f0c24f', 'expense', 'plataforma de cursos', 'phrase', 'high', false),
    ('dedbe159-e6d5-5a5c-90e8-09fbbd75713a', '01b51d39-347e-560c-ac07-d0a700f0c24f', 'expense', 'assinatura de cursos', 'phrase', 'high', false),
    ('2c9d6dc6-f531-559a-bf7e-eb7433bd0425', 'bac52783-54ca-5401-92da-5afa29fc05d4', 'expense', 'livro', 'alias', 'high', false),
    ('0b5ff333-7c16-5795-b494-b59b2b40e3fc', 'bac52783-54ca-5401-92da-5afa29fc05d4', 'expense', 'livros', 'alias', 'high', false),
    ('c7f90283-2b5b-5573-a57f-9a1c6d8ebe13', 'bac52783-54ca-5401-92da-5afa29fc05d4', 'expense', 'e-book', 'alias', 'high', false),
    ('18bee7ea-16e6-5746-bfad-ccc40a84fdfa', 'bac52783-54ca-5401-92da-5afa29fc05d4', 'expense', 'ebook', 'alias', 'high', false),
    ('3c65421f-1091-592a-8d35-56fcd6198ccc', 'bac52783-54ca-5401-92da-5afa29fc05d4', 'expense', 'kindle', 'alias', 'high', false),
    ('fef0751b-8044-5f5a-bc6f-8c21e4b00b70', 'bac52783-54ca-5401-92da-5afa29fc05d4', 'expense', 'livraria', 'alias', 'high', false),
    ('abdae59c-8701-560e-9693-80228e556065', 'bac52783-54ca-5401-92da-5afa29fc05d4', 'expense', 'sebo', 'alias', 'high', false),
    ('c92b49a1-ea5f-5a6b-97a1-eb35c1e9a509', '654552ab-829d-5b4d-b0ec-4cb1463454d7', 'expense', 'certificação', 'alias', 'high', false),
    ('388f74c8-56fe-53c2-992e-44c5ce6b36bf', '654552ab-829d-5b4d-b0ec-4cb1463454d7', 'expense', 'prova de certificação', 'phrase', 'high', false),
    ('4abc24fc-86bf-5aa3-ac30-c0b7ba784bf7', '654552ab-829d-5b4d-b0ec-4cb1463454d7', 'expense', 'exame profissional', 'phrase', 'high', false),
    ('7254cfdb-4c9e-5afa-a264-eb606a175bad', '3c5e9972-7f59-5f6b-aea4-ace59985cce0', 'expense', 'congresso', 'alias', 'high', false),
    ('be375d26-fc0c-5638-a7f6-0ec86b725135', '3c5e9972-7f59-5f6b-aea4-ace59985cce0', 'expense', 'workshop', 'alias', 'high', false),
    ('5f42f630-9aff-5204-a9f3-fd51953acaa4', '3c5e9972-7f59-5f6b-aea4-ace59985cce0', 'expense', 'seminário', 'alias', 'high', false),
    ('f05dd011-55b2-5edc-a687-11d5c7dd02d3', '3c5e9972-7f59-5f6b-aea4-ace59985cce0', 'expense', 'palestra', 'alias', 'high', false),
    ('81b080d6-598c-5d28-9d3f-4591570fc264', '3c5e9972-7f59-5f6b-aea4-ace59985cce0', 'expense', 'feira de negócios', 'phrase', 'high', false),
    ('ee5b9568-cb5e-5b8d-a01a-0d5e1d645413', 'fec9aed9-2699-538e-bbae-eb4bcdfb1ce3', 'expense', 'inglês', 'alias', 'high', false),
    ('35306d2e-92be-5ae9-8e41-21c2efe0aa8a', 'fec9aed9-2699-538e-bbae-eb4bcdfb1ce3', 'expense', 'espanhol', 'alias', 'high', false),
    ('bdb2a8d3-ed40-5af1-847e-a17e17c43a20', 'fec9aed9-2699-538e-bbae-eb4bcdfb1ce3', 'expense', 'francês', 'alias', 'high', false),
    ('415c5c1e-8cb8-544c-8d76-c43479d7b8b5', 'fec9aed9-2699-538e-bbae-eb4bcdfb1ce3', 'expense', 'alemão', 'alias', 'high', false),
    ('27ea0b4d-f2bd-5e5f-b8c1-48ade8bdac10', 'fec9aed9-2699-538e-bbae-eb4bcdfb1ce3', 'expense', 'curso de idiomas', 'phrase', 'high', false),
    ('1f1cdad1-b7da-5aee-8a87-21ceddc3c3cf', 'ddbb0dc7-8b85-5177-8cfc-3bb2aed6c75c', 'expense', 'entrega de comida', 'phrase', 'high', false),
    ('933dd9db-c037-545f-b82f-6a16f7d73e2b', 'ddbb0dc7-8b85-5177-8cfc-3bb2aed6c75c', 'expense', 'pedir comida', 'phrase', 'high', false),
    ('76988004-a5cb-5698-80e2-0d1c3b2b1373', 'd539672d-961f-5553-b807-0e0156a63163', 'expense', 'restaurante', 'alias', 'high', false),
    ('58e49435-b94c-5be5-aae0-9525e60a2479', 'd539672d-961f-5553-b807-0e0156a63163', 'expense', 'jantar fora', 'phrase', 'high', false),
    ('d3cb137a-07b5-5d38-a1e6-a516da70251e', 'd539672d-961f-5553-b807-0e0156a63163', 'expense', 'almoço fora', 'phrase', 'high', false),
    ('0b499a3d-95c9-5902-8ce2-4459200a6559', 'a371851d-56cb-551d-addb-022575b8d6e9', 'expense', 'bar', 'alias', 'high', false),
    ('cf2cefa0-086c-57f5-adf5-be555707c250', 'a371851d-56cb-551d-addb-022575b8d6e9', 'expense', 'boteco', 'alias', 'high', false),
    ('ec9e32d4-7ec8-5a35-865e-dfca5c5594e4', 'a371851d-56cb-551d-addb-022575b8d6e9', 'expense', 'lanche', 'alias', 'high', false),
    ('f2636680-0521-5253-b9c7-9012c55e6980', 'a371851d-56cb-551d-addb-022575b8d6e9', 'expense', 'hamburgueria', 'alias', 'high', false),
    ('4d323f9a-657e-5a23-befc-7d7b6fb1b5a2', 'a371851d-56cb-551d-addb-022575b8d6e9', 'expense', 'pizzaria', 'alias', 'high', false),
    ('ce004b01-f458-5331-b494-c1ae00ff6b12', 'a20b4072-23b7-53e8-8d03-8146e0473218', 'expense', 'coffee shop', 'phrase', 'high', false),
    ('2321be04-299f-5494-9c9b-f4f1a42cd738', '85e56497-2e31-55d3-9516-376e61860708', 'expense', 'filme online', 'phrase', 'high', false),
    ('7d09866b-5a84-5dd8-b8e2-15f617b3eef6', '85e56497-2e31-55d3-9516-376e61860708', 'expense', 'série online', 'phrase', 'high', false),
    ('4a050457-4a04-585d-925d-69c6af4aae38', '8580a31d-041d-5fa4-b86e-af90108af0cb', 'expense', 'streaming de música', 'phrase', 'high', false),
    ('aebfb898-5fbb-5a5f-bbde-f2c4c6ff8c22', '8580a31d-041d-5fa4-b86e-af90108af0cb', 'expense', 'podcast premium', 'phrase', 'high', false),
    ('de658180-9e86-55a9-b465-4c92a773bb48', '514c00a0-ca41-5798-85d0-39992fbc223c', 'expense', 'game', 'alias', 'high', false),
    ('9590d709-2bae-58ab-b900-13035f0f0f7c', '514c00a0-ca41-5798-85d0-39992fbc223c', 'expense', 'videogame', 'alias', 'high', false),
    ('c3999dd4-be5d-5ad6-853f-4b31cee0b072', '514c00a0-ca41-5798-85d0-39992fbc223c', 'expense', 'assinatura de jogos', 'phrase', 'high', false),
    ('4ec2d34c-3bc5-57dc-93b9-2a33baf6669a', '5190df3d-8e6d-59bc-9e5b-d7e85e45154c', 'expense', 'ingresso de cinema', 'phrase', 'high', false),
    ('5732f962-48ae-523c-98e0-df52f1cab025', '5190df3d-8e6d-59bc-9e5b-d7e85e45154c', 'expense', 'ingresso de teatro', 'phrase', 'high', false),
    ('8a44474f-5f7e-5c01-a96a-19cb2627c73e', '09073cdd-4d58-5073-ae16-53ba2c3a4209', 'expense', 'show', 'alias', 'high', false),
    ('b1fe6509-3dfd-5ec4-89b0-7fe760b54c35', '09073cdd-4d58-5073-ae16-53ba2c3a4209', 'expense', 'festival', 'alias', 'high', false),
    ('ad8b3985-1d47-54bd-859f-e54cbe280f46', 'aed45dcf-8fbe-5828-8fb6-87babd271d6c', 'expense', 'passeio', 'alias', 'high', false),
    ('79737f9a-052e-52b6-ba95-e5f21b7b6c8b', 'aed45dcf-8fbe-5828-8fb6-87babd271d6c', 'expense', 'parque', 'alias', 'high', false),
    ('e3e44581-258c-5472-9a43-9de2a424a1d8', 'aed45dcf-8fbe-5828-8fb6-87babd271d6c', 'expense', 'zoológico', 'alias', 'high', false),
    ('32f54d64-bfa7-5fb8-ac24-b8566d7dac0f', 'aed45dcf-8fbe-5828-8fb6-87babd271d6c', 'expense', 'parque aquático', 'phrase', 'high', false),
    ('7e5ee527-c8be-5331-9321-598598801d93', '6feeb8fd-8faa-56d1-a0d0-d9d746e45f21', 'expense', 'corrida para passeio', 'phrase', 'high', false),
    ('33f3e094-26e6-5318-8097-c36d278d4460', '6feeb8fd-8faa-56d1-a0d0-d9d746e45f21', 'expense', 'transporte para passeio', 'phrase', 'high', false),
    ('816db162-50cd-5c67-a825-e3913347180d', '6feeb8fd-8faa-56d1-a0d0-d9d746e45f21', 'expense', 'volta do bar', 'phrase', 'high', false),
    ('30fbddf1-70cb-5bf9-9ead-6a92a4a2ff6a', '0134668f-785b-5ac1-bcf5-e6c4f566de64', 'expense', 'viagem de lazer', 'phrase', 'high', false),
    ('b1fecaa8-3123-5c67-943e-0383fad387bb', '0134668f-785b-5ac1-bcf5-e6c4f566de64', 'expense', 'férias', 'alias', 'high', false),
    ('cdfe36cf-16bc-5ef7-9013-1837471ba253', '0134668f-785b-5ac1-bcf5-e6c4f566de64', 'expense', 'passagem de férias', 'phrase', 'high', false),
    ('12219e79-5985-5107-9028-e4cff1abc232', '7a69762f-6016-593a-9e62-f56f508ec9e1', 'expense', 'pousada', 'alias', 'high', false),
    ('bf33fe5f-802f-5b90-a29c-c09932eada33', '7a69762f-6016-593a-9e62-f56f508ec9e1', 'expense', 'hospedagem', 'alias', 'high', false),
    ('3cab2f03-54ed-5470-a7c9-8aeceb4c6217', '7a69762f-6016-593a-9e62-f56f508ec9e1', 'expense', 'resort', 'alias', 'high', false),
    ('b33dcb1d-c739-5b87-9c3f-0495130f8330', '14416063-f271-53e2-8a58-6682461ec532', 'expense', 'roupa', 'alias', 'high', false),
    ('33fcfa59-273c-5483-96bf-514c2007671c', '14416063-f271-53e2-8a58-6682461ec532', 'expense', 'roupas', 'alias', 'high', false),
    ('6223130b-a912-5953-8681-64f6b7afa567', '14416063-f271-53e2-8a58-6682461ec532', 'expense', 'calçado', 'alias', 'high', false),
    ('7c2a3955-9408-53c2-ace3-cd3b4bb208b4', '14416063-f271-53e2-8a58-6682461ec532', 'expense', 'sapato', 'alias', 'high', false),
    ('020ccfd5-0f7e-5bc3-81c5-c3a0b9409296', '14416063-f271-53e2-8a58-6682461ec532', 'expense', 'tênis', 'alias', 'high', false),
    ('9adffd41-1eec-528f-98f3-3c595db832ab', 'f9656739-8d1c-5675-8eaf-63a057137307', 'expense', 'salão', 'alias', 'high', false),
    ('997da164-9aaa-5d8e-b831-b64fc2d5640d', 'f9656739-8d1c-5675-8eaf-63a057137307', 'expense', 'cabeleireiro', 'alias', 'high', false),
    ('9365a71f-299e-5448-bfef-441c6f06320e', 'f9656739-8d1c-5675-8eaf-63a057137307', 'expense', 'manicure', 'alias', 'high', false),
    ('7f1202c2-0bea-5388-b29f-990bfb8dd836', 'f9656739-8d1c-5675-8eaf-63a057137307', 'expense', 'pedicure', 'alias', 'high', false),
    ('ebdd7047-57bb-57e3-bcb8-e28f1096109d', 'f9656739-8d1c-5675-8eaf-63a057137307', 'expense', 'barbeiro', 'alias', 'high', false),
    ('8598b193-ff7c-50d1-9691-3d0db11a96f9', 'f9656739-8d1c-5675-8eaf-63a057137307', 'expense', 'estética', 'alias', 'high', false),
    ('91fa1b4b-c77b-578e-ae29-9389c6b36c25', 'f9656739-8d1c-5675-8eaf-63a057137307', 'expense', 'maquiagem', 'alias', 'high', false),
    ('a491740f-10e1-5841-b9ff-8c9ee8ade972', '671873dc-f403-5315-877c-d6d46d0f5a8f', 'expense', 'hobby', 'alias', 'high', false),
    ('e1a9fa4a-08e5-5baf-9fbc-dcb64781917c', '671873dc-f403-5315-877c-d6d46d0f5a8f', 'expense', 'artesanato', 'alias', 'high', false),
    ('ffd5649a-a703-5b47-ae76-076c85ef2693', '671873dc-f403-5315-877c-d6d46d0f5a8f', 'expense', 'fotografia', 'alias', 'high', false),
    ('4512fd6d-f0c0-5b96-a60f-a9ccad156ad2', '671873dc-f403-5315-877c-d6d46d0f5a8f', 'expense', 'coleção', 'alias', 'high', false),
    ('940d73e7-842d-5f30-8b8a-48b23854b3a4', 'c0e10d9f-b0fe-59e7-8fb9-22a3bebd4784', 'expense', 'academia', 'alias', 'high', false),
    ('6418d522-ed8d-54f0-bb85-e6c9a0780f99', 'c0e10d9f-b0fe-59e7-8fb9-22a3bebd4784', 'expense', 'esporte', 'alias', 'high', false),
    ('fdecb38e-1d16-5cc0-91a7-0f9fd3371cb0', 'c0e10d9f-b0fe-59e7-8fb9-22a3bebd4784', 'expense', 'futebol', 'alias', 'high', false),
    ('46925b84-1508-5f3b-b599-488b2973fb25', 'c0e10d9f-b0fe-59e7-8fb9-22a3bebd4784', 'expense', 'corrida', 'alias', 'high', false),
    ('d915ac2e-7402-5fa7-a5c5-d73299074baf', 'c0e10d9f-b0fe-59e7-8fb9-22a3bebd4784', 'expense', 'ciclismo', 'alias', 'high', false),
    ('19e7cb4b-fac2-576a-b869-b637fe0ee37c', '3ff5e6b5-b958-5848-9092-73eb541598fc', 'expense', 'celular novo', 'phrase', 'high', false),
    ('c3c85368-2692-5627-9908-d717b44b87c1', '3ff5e6b5-b958-5848-9092-73eb541598fc', 'expense', 'iphone', 'alias', 'high', false),
    ('a83cc854-6797-5923-b750-cc61a4e475dc', '3ff5e6b5-b958-5848-9092-73eb541598fc', 'expense', 'smartphone', 'alias', 'high', false),
    ('b69eee63-da39-5bd0-a9e5-1f7d697296d0', '3ff5e6b5-b958-5848-9092-73eb541598fc', 'expense', 'notebook', 'alias', 'high', false),
    ('00ebcedf-60b2-5812-925e-8ea7daabf708', '3ff5e6b5-b958-5848-9092-73eb541598fc', 'expense', 'computador', 'alias', 'high', false),
    ('c42800a1-0873-57fd-8afa-7069658c62a2', '3ff5e6b5-b958-5848-9092-73eb541598fc', 'expense', 'tablet', 'alias', 'high', false),
    ('f713c10c-c68b-5264-af51-f8718a722abe', '3ff5e6b5-b958-5848-9092-73eb541598fc', 'expense', 'smartwatch', 'alias', 'high', false),
    ('e64aa45a-5cd2-5f7c-8c51-7402a5fd7f30', 'ef1a26ec-e12d-5b3c-b7ba-3634bb89647c', 'expense', 'carro novo', 'phrase', 'high', false),
    ('e14fad82-3271-59d6-9e10-eb4075704506', 'ef1a26ec-e12d-5b3c-b7ba-3634bb89647c', 'expense', 'moto nova', 'phrase', 'high', false),
    ('c7e94f19-c4ee-5983-a273-41bd6437a857', 'ef1a26ec-e12d-5b3c-b7ba-3634bb89647c', 'expense', 'entrada do veículo', 'phrase', 'high', false),
    ('0acf595a-b3db-5b53-93a2-13fedbea2277', 'ef1a26ec-e12d-5b3c-b7ba-3634bb89647c', 'expense', 'troca de carro', 'phrase', 'high', false),
    ('c33f39b4-394d-526d-b60c-aa5ac2094e2a', '61698c19-7281-5016-8cd3-b3799ddb575c', 'expense', 'reforma', 'alias', 'high', false),
    ('4b9b69e3-a1f4-549b-a1e2-a58f6be04411', '61698c19-7281-5016-8cd3-b3799ddb575c', 'expense', 'móveis', 'alias', 'high', false),
    ('32426e6e-a3ea-5f02-b277-212a15fc2de7', '61698c19-7281-5016-8cd3-b3799ddb575c', 'expense', 'geladeira', 'alias', 'high', false),
    ('8e3c77dc-610b-5818-8ffb-dce4441a63ca', '61698c19-7281-5016-8cd3-b3799ddb575c', 'expense', 'sofá', 'alias', 'high', false),
    ('2fe697d6-8df8-52be-b44a-4dfbb8f87624', '61698c19-7281-5016-8cd3-b3799ddb575c', 'expense', 'televisão', 'alias', 'high', false),
    ('f4d19083-2ddb-59ab-8e60-2750a55e5107', '61698c19-7281-5016-8cd3-b3799ddb575c', 'expense', 'ar-condicionado', 'alias', 'high', false),
    ('5c8ba680-26d3-5231-b361-616989ab12d7', '8a4228f0-bc77-5d24-949d-5a7afa8063dc', 'expense', 'viagem planejada', 'phrase', 'high', false),
    ('a00d98f8-deea-55fa-b3cc-0763fdf0b0dd', '8a4228f0-bc77-5d24-949d-5a7afa8063dc', 'expense', 'fundo de viagem', 'phrase', 'high', false),
    ('c36955cf-5e28-56f4-b00f-88f52260385a', '8a4228f0-bc77-5d24-949d-5a7afa8063dc', 'expense', 'guardar para viajar', 'phrase', 'high', false),
    ('7d74ca0f-cd6c-58ed-b4ad-431732af7ffb', 'e91062ea-8bc9-5d30-a317-260faaf14e56', 'expense', 'enxoval', 'alias', 'high', false),
    ('5ef434ee-a1c1-5539-b443-513e160b7238', 'e91062ea-8bc9-5d30-a317-260faaf14e56', 'expense', 'chá de bebê', 'phrase', 'high', false),
    ('08478dcf-a4e1-5188-bb9d-751a625a4b69', 'e91062ea-8bc9-5d30-a317-260faaf14e56', 'expense', 'chegada do bebê', 'phrase', 'high', false),
    ('c64aebd5-eb5a-53af-8dff-0e529d2fcac5', '480b8f7d-6dc2-5d62-b154-669818123f65', 'expense', 'abrir empresa', 'phrase', 'high', false),
    ('cc702e2a-e8d6-5381-9a21-a38d3a0424a7', '480b8f7d-6dc2-5d62-b154-669818123f65', 'expense', 'capital de giro', 'phrase', 'high', false),
    ('6ab1bc42-0b32-5b14-a49c-c43a868a2bf1', '480b8f7d-6dc2-5d62-b154-669818123f65', 'expense', 'equipamento para empresa', 'phrase', 'high', false),
    ('4fd1ed46-eda4-5c0a-b061-d5de6069f702', '946643a8-9e00-5bad-a860-f74ed74cf246', 'expense', 'quitar dívida', 'phrase', 'high', false),
    ('89bbd83b-476e-551d-b17f-9d5f63963623', '946643a8-9e00-5bad-a860-f74ed74cf246', 'expense', 'amortizar dívida', 'phrase', 'high', false),
    ('a3a74c60-dd93-578e-9780-674ca1ef95bb', '946643a8-9e00-5bad-a860-f74ed74cf246', 'expense', 'antecipar financiamento', 'phrase', 'high', false),
    ('70c350e4-3e77-5897-af44-fb36f39e223c', '45c7e533-fb00-50d9-aeb3-71bdb99098bd', 'expense', 'reserva emergencial', 'phrase', 'high', false),
    ('543d3640-b99e-5c55-88d1-146c1aa6bc69', '45c7e533-fb00-50d9-aeb3-71bdb99098bd', 'expense', 'fundo de emergência', 'phrase', 'high', false),
    ('43df1122-db09-5d9c-a00c-d2ebfe97b57b', '9103a0e6-366b-5c77-a31d-e3ed58991d14', 'expense', 'tesouro selic', 'phrase', 'high', false),
    ('f6822ac3-c958-5518-b798-16bddbd8e03c', '9103a0e6-366b-5c77-a31d-e3ed58991d14', 'expense', 'tesouro ipca', 'phrase', 'high', false),
    ('fdeabfe1-5a64-5411-a7ad-873899e10954', '9103a0e6-366b-5c77-a31d-e3ed58991d14', 'expense', 'tesouro prefixado', 'phrase', 'high', false),
    ('def07487-fc0d-537a-a905-4cc17016b477', '1e5b4db2-b186-5524-b955-32553307d81c', 'expense', 'fii', 'alias', 'high', false),
    ('cd0661f4-bf31-51f6-a221-06bc92479c35', '1e5b4db2-b186-5524-b955-32553307d81c', 'expense', 'fiis', 'alias', 'high', false),
    ('6bc888f9-19c5-5440-9536-30736cd44752', '1e5b4db2-b186-5524-b955-32553307d81c', 'expense', 'fundo imobiliário', 'phrase', 'high', false),
    ('dc921791-ff36-5551-b37b-4012e3179726', 'b1ac9b12-0b4d-5791-87d5-6628c9bbfa9a', 'expense', 'pgbl', 'alias', 'high', false),
    ('a80d63b8-215b-5d40-8eb3-ddea8cda8645', 'b1ac9b12-0b4d-5791-87d5-6628c9bbfa9a', 'expense', 'vgbl', 'alias', 'high', false),
    ('965967aa-0dad-5f03-a67b-39a60beefe83', '9747b1c4-f9dd-5565-ad6d-0f3476ebab9e', 'expense', 'cripto', 'alias', 'high', false),
    ('f63add27-db3f-52d8-8e1b-f5ceb7141eda', '9747b1c4-f9dd-5565-ad6d-0f3476ebab9e', 'expense', 'criptomoeda', 'alias', 'high', false),
    ('911e9e88-851d-51ab-ae00-b451ffa4fad5', '9747b1c4-f9dd-5565-ad6d-0f3476ebab9e', 'expense', 'bitcoin', 'alias', 'high', false),
    ('9b3ed1af-253c-59f4-8394-c6d90f8b076f', '9747b1c4-f9dd-5565-ad6d-0f3476ebab9e', 'expense', 'ethereum', 'alias', 'high', false),
    ('395f24dd-7878-57c4-8662-0771f21a463a', '9747b1c4-f9dd-5565-ad6d-0f3476ebab9e', 'expense', 'solana', 'alias', 'high', false),
    ('a414d654-3ee8-5cb3-9f8e-42fb1d3cc784', '9747b1c4-f9dd-5565-ad6d-0f3476ebab9e', 'expense', 'usdt', 'alias', 'high', false),
    ('aa648874-4a90-5ee3-b7da-74e4b4c07df3', 'da4f4c4c-864e-577b-9f4d-d7800f7a85ab', 'expense', 'stocks', 'alias', 'high', false),
    ('242fe07c-589c-54f3-8423-d30cc038eb1c', 'da4f4c4c-864e-577b-9f4d-d7800f7a85ab', 'expense', 'etf internacional', 'phrase', 'high', false),
    ('1693ff4d-5514-5b25-969c-d21b2e5ce7cf', '4b61504d-9cc2-579f-b927-d1963bd1e0ca', 'income', 'plr', 'alias', 'high', false),
    ('70649930-4bc8-579d-933a-2f26e64b00af', '4b61504d-9cc2-579f-b927-d1963bd1e0ca', 'income', 'participação nos lucros', 'phrase', 'high', false),
    ('b79ce541-db10-5fd1-8006-c1442bed137a', '4b61504d-9cc2-579f-b927-d1963bd1e0ca', 'income', 'bônus salarial', 'phrase', 'high', false),
    ('f673328c-b95e-5e45-983f-4f8db5166312', '403192d5-5e85-54d3-a4b0-d4029e754c5c', 'income', 'freela', 'alias', 'high', false),
    ('9774bdb5-0c9b-565d-bb71-4a6b16aa7d8d', '403192d5-5e85-54d3-a4b0-d4029e754c5c', 'income', 'trabalho freelancer', 'phrase', 'high', false),
    ('9e7fd1ea-dedd-5cbd-9a77-d1997c2049f7', 'c8276187-8320-5be9-9519-8b6d2a4620b2', 'income', 'rendimento de investimento', 'phrase', 'high', false),
    ('8ab6f971-9544-5332-8ed1-abcc8a9a15ac', '5b983987-3b1d-5bd5-80d3-017416c3f0f8', 'income', 'dividendo', 'alias', 'high', false),
    ('bc1ccde0-c3d4-53f3-a7b3-feefc1952c26', '5b983987-3b1d-5bd5-80d3-017416c3f0f8', 'income', 'provento', 'alias', 'high', false),
    ('a192c929-8ae5-555b-98f3-3c4726e73d42', '00b886a7-d221-592e-8068-fa296924b333', 'income', 'aluguel recebido', 'phrase', 'high', false),
    ('0781ff8c-0a7c-5249-bb3f-fd4aaa2d079f', '00b886a7-d221-592e-8068-fa296924b333', 'income', 'renda de aluguel residencial', 'phrase', 'high', false),
    ('20907264-c9b3-588e-8d09-2d32e85c00c7', '7a17fe1c-900c-57d8-a1dc-22bf9139cf83', 'income', 'restituição do imposto de renda', 'phrase', 'high', false),
    ('c780cb14-0dc1-59a9-b415-45b1f5a7250b', '7a17fe1c-900c-57d8-a1dc-22bf9139cf83', 'income', 'restituição ir', 'phrase', 'high', false),
    ('e14aec5d-6a33-5d56-95b5-8740b1995f9e', '3791836d-bc96-57ae-87b4-fae12c1c111b', 'income', 'dinheiro de volta', 'phrase', 'high', false),
    ('93c79eae-5403-55de-b442-7fbf818bd673', '06373332-7fb8-52b6-940e-f0c5699c6114', 'income', 'presente em dinheiro', 'phrase', 'high', false),
    ('b82501dd-cefc-57d3-bf07-9072a433649a', '6650a195-013b-5808-8845-22a0657da9ba', 'income', 'venda', 'alias', 'high', false),
    ('624c16f1-2625-5b26-80f5-47fe290fb59b', '6650a195-013b-5808-8845-22a0657da9ba', 'income', 'vendas', 'alias', 'high', false);

UPDATE mecontrola.category_editorial_version SET version = version + 1;

-- Consolidated from migrations/000010_seed_dictionary_canonicals.up.sql
SET LOCAL lock_timeout    = '5s';
SET LOCAL statement_timeout = '120s';

-- Deprecate aliases/phrases that conflict with canonicals being inserted (RF-36a: deprecated_at update allowed)
UPDATE mecontrola.category_dictionary SET deprecated_at = now() WHERE id = '5c8ba680-26d3-5231-b361-616989ab12d7' AND deprecated_at IS NULL;

-- Deprecate broken canonicals where term = slug-with-hyphens (mismatched user input)
UPDATE mecontrola.category_dictionary SET deprecated_at = now()
WHERE signal_type = 'canonical_name' AND deprecated_at IS NULL AND term IN (
    'aluguel-residencial-recebido',
    'bares-e-lanches',
    'beleza-e-estetica',
    'casa-e-reforma',
    'cinema-e-teatro',
    'congressos-e-workshops',
    'consultas-e-exames',
    'cursos-e-treinamentos',
    'decimo-terceiro',
    'escola-e-creche',
    'esportes-e-academia',
    'faculdade-e-pos-graduacao',
    'familia-e-enxoval',
    'feira-e-hortifruti',
    'financiamento-imobiliario',
    'fundos-imobiliarios',
    'games-e-assinaturas-de-jogos',
    'hospedagem-de-lazer',
    'investimentos-internacionais',
    'ipva-e-licenciamento',
    'livros-e-ebooks',
    'manutencao-veicular',
    'medicamentos-continuos',
    'medicamentos-e-farmacia',
    'musica-e-audio',
    'passeios-e-parques',
    'plano-de-saude',
    'plano-odontologico',
    'plataformas-de-ensino',
    'plr-e-bonus',
    'presentes-em-dinheiro',
    'previdencia-privada',
    'quitacao-de-dividas',
    'reserva-de-emergencia',
    'restituicao-de-ir',
    'roupas-e-calcados',
    'shows-e-eventos',
    'streaming-de-video',
    'terapia-e-saude-mental',
    'tesouro-direto',
    'transporte-de-lazer',
    'transporte-por-aplicativo-recorrente',
    'transporte-publico',
    'vendas-diversas',
    'viagem-planejada',
    'viagens-de-lazer'
);

-- Insert canonical_name with human-readable accent-stripped lowercase term (RF-29, RF-32)
INSERT INTO mecontrola.category_dictionary (id, category_id, kind, term, signal_type, confidence, is_ambiguous) VALUES
    ('f85e22d3-8e36-52d1-bbd4-2cb7c3273efb', '8eaa0160-80cd-5c14-a361-d98068aab2cd', 'expense', 'taxas residenciais', 'canonical_name', 'high', false),
    ('9db7b239-47a3-5d1c-adb6-f248ae72ecdd', '0abec125-fa91-5ac6-a82e-3686533c4b8d', 'expense', 'seguro residencial', 'canonical_name', 'high', false),
    ('7fe9cedf-2371-50d1-b6af-2faa32a6f8df', '2e90fdd3-1008-5423-8215-5db1880fa60b', 'expense', 'tv por assinatura', 'canonical_name', 'high', false),
    ('6cb30b66-7c23-5285-9afd-78de3a5d76a2', '0b549268-cbaf-5531-af54-ab47e14a072a', 'expense', 'padaria', 'canonical_name', 'high', false),
    ('41837eb8-6c50-550a-9866-bd228c165d5e', '7e647851-411c-52d7-a0f2-13535469d918', 'expense', 'estacionamento mensal', 'canonical_name', 'high', false),
    ('f5e7cbc6-00ac-5b82-a448-9f3bf7acfceb', '9dc2ed94-0ea2-5b72-a948-850670f2bee7', 'expense', 'pedagio', 'canonical_name', 'high', false),
    ('9a2d6e91-6d33-5b51-a054-930bf6cfd792', '75e7909d-d816-5609-ac03-89d1c6eb31f5', 'expense', 'seguro veicular', 'canonical_name', 'high', false),
    ('e3e44c4c-c683-5228-ab29-f1a725c0542f', '5828e634-94c1-5800-8160-4ecb1eff1a81', 'expense', 'pensao alimenticia', 'canonical_name', 'high', false),
    ('80692cb8-7039-5104-915b-acdab54a266b', '6a0d56cc-f9d8-5c95-be2a-60f8f69c912c', 'expense', 'seguros pessoais', 'canonical_name', 'high', false),
    ('f27ae90d-b38d-5e84-8ff5-6fd645042a0f', '178d590e-bc16-5df3-a7c8-ec7c193896d5', 'expense', 'assinaturas essenciais', 'canonical_name', 'high', false),
    ('73701c37-0f0a-5c63-a756-df9699760f6b', '347e0488-a4a7-55e8-8882-ae868c9d749d', 'expense', 'tarifas bancarias', 'canonical_name', 'high', false),
    ('96221aa2-8f14-5510-b56c-807ee5706898', '7d56377d-bdd0-5152-9b94-10639bc7f39b', 'expense', 'impostos e tributos', 'canonical_name', 'high', false),
    ('8144a0ff-bda5-5be7-9c87-d185a90b81a3', 'b29895dd-f1c5-5375-a5d1-082d9e2c3620', 'expense', 'emprestimos e financiamentos', 'canonical_name', 'high', false),
    ('5c0451d8-7f97-5e5f-bf7d-b2005db4f6ca', '5b9a1cba-b400-508c-a615-a419d9b06dcf', 'expense', 'dividas e juros', 'canonical_name', 'high', false),
    ('fa96f333-b55b-5029-bb4e-4f28d6841750', '3f7c80e0-820c-5766-ba50-826a6d82b8e6', 'expense', 'manutencao da casa', 'canonical_name', 'high', false),
    ('1bc1d056-50fb-56d8-9750-afa87f397a57', 'ca8e4a6c-58ae-5049-8c24-826bd471e896', 'expense', 'servicos domesticos', 'canonical_name', 'high', false),
    ('15b5b408-0382-5a5b-a6ad-69484b657fae', 'd4b74050-db29-53e6-bcee-be5c333f8817', 'expense', 'pets recorrentes', 'canonical_name', 'high', false),
    ('fb568379-8cc6-5c71-aab6-b7b399d163e4', 'd1d7dbba-1e83-596c-a4e5-d520cd06c88a', 'expense', 'outros custos fixos', 'canonical_name', 'high', false),
    ('d6780834-98bb-50e5-a697-dafb3ee45e53', '6f70f7d5-d319-5a97-a319-c864e7567285', 'expense', 'material de estudo', 'canonical_name', 'high', false),
    ('a9165b0a-f117-515b-9375-7b8a54af233a', '8d114d26-b1a4-5a5f-8995-194c088c7b3f', 'expense', 'mentoria e coaching', 'canonical_name', 'high', false),
    ('fe8a84ba-97f2-52bc-9fd1-40a83499a0c3', 'ce2850ad-8d51-5224-b9a4-d884361e4639', 'expense', 'aulas particulares', 'canonical_name', 'high', false),
    ('cfd19f9b-e34a-5980-b508-81b7024617bf', '4850d076-7dea-5b73-8d32-fff55765dd2f', 'expense', 'software e ferramentas de estudo', 'canonical_name', 'high', false),
    ('b6705ba4-1796-5f13-b3f1-fd792231426f', 'ce233b45-2c19-536e-92bd-6b43958c9363', 'expense', 'outros conhecimentos', 'canonical_name', 'high', false),
    ('b5cded9c-7524-5733-a855-43e60a0a0031', 'a2af4429-8e17-559f-bba4-f790c7732776', 'expense', 'compras pessoais', 'canonical_name', 'high', false),
    ('6cd3121d-f7a1-57d7-8e40-dcfa1e83c823', '09e7cd05-40bf-5100-92e9-439a7baf0c0c', 'expense', 'presentes', 'canonical_name', 'high', false),
    ('dfc719cb-c559-5fba-9ef0-31994a9e6726', '481d2d82-a013-5991-8210-0bfcb44af4fa', 'expense', 'pets nao recorrentes', 'canonical_name', 'high', false),
    ('91d06ae3-d2b8-545c-9778-a358bd0c9ba9', 'c2470946-ebf3-5baf-86cd-696b11baf497', 'expense', 'doacoes', 'canonical_name', 'high', false),
    ('3880b219-13d3-5d21-acdb-1b1c64f78336', '0016763e-655c-571a-90cb-bec5a18d4969', 'expense', 'outros prazeres', 'canonical_name', 'high', false),
    ('91a3224c-f958-5606-b683-c9c08a67df69', '6752f218-cbf9-5108-94e5-6732fdb6a0c6', 'expense', 'casamento e festa', 'canonical_name', 'high', false),
    ('e8c20cf7-1af3-5c6c-8353-59d1bb2a73fe', '1c178224-bd1b-51a0-bc6a-a8f12efa54c1', 'expense', 'educacao planejada', 'canonical_name', 'high', false),
    ('378bc84e-d3ae-50a3-89b3-e4c08363d57c', '568b9200-dae4-512c-a93c-192192d2ee4f', 'expense', 'saude planejada', 'canonical_name', 'high', false),
    ('81e94a34-2958-5602-91e6-8e82dc1216cf', 'ab070648-d71b-5920-a1dd-060f1f542371', 'expense', 'compra planejada', 'canonical_name', 'high', false),
    ('dfbe4e1d-1770-5766-ac0e-9a81b9fcffb1', '8c1c3dd1-6b38-5b85-a37c-e7c9a769ff94', 'expense', 'outras metas', 'canonical_name', 'high', false),
    ('14fe0148-9565-59bb-9394-b891abf13788', 'e79c7c54-c8c5-5b9f-9cbb-4bff3c98e429', 'expense', 'reserva de oportunidade', 'canonical_name', 'high', false),
    ('3f45e640-c36d-5ef8-a6b3-81c3bec4909a', 'ee26c4d9-ca74-5537-80b9-4d90815b9c06', 'expense', 'cdb e rdb', 'canonical_name', 'high', false),
    ('9afc81fe-d159-5ff2-9d9d-4d62095c385a', 'd35da3b9-65c5-55b8-9915-13354e202644', 'expense', 'lci e lca', 'canonical_name', 'high', false),
    ('889fbac5-3a8d-5846-953e-28be7990c440', 'b0fba509-bd7d-5f1c-9845-2288bee6c276', 'expense', 'debentures e credito privado', 'canonical_name', 'high', false),
    ('6cf0b42c-f27f-5f0c-893f-cfc6a0f3dcfa', 'eb6c008a-2fe5-58bf-a879-a3a0d2ecf6cb', 'expense', 'fundos de renda fixa', 'canonical_name', 'high', false),
    ('a3459424-1164-515e-9254-cb577268dd3c', 'e1266272-eb97-5a9f-857d-e6b7b261cf9e', 'expense', 'acoes', 'canonical_name', 'high', false),
    ('780bfef1-8089-51b5-a219-e0b0380d5665', 'e917b351-60d6-53c5-ab5b-a92e663d700b', 'expense', 'etfs', 'canonical_name', 'high', false),
    ('0199bccc-b6c1-5c91-846f-4ccef0fc9937', 'abc00654-7b1d-5587-9de8-506710c42da4', 'expense', 'bdrs', 'canonical_name', 'high', false),
    ('2292bd53-5651-5cd0-a6d2-5a0db9198cfa', 'e130a310-4a8d-5f0f-b050-405165e28966', 'expense', 'fundos de investimento', 'canonical_name', 'high', false),
    ('23dcae3d-7a5b-5d29-8e43-015e76a604a5', '866793cb-4059-54b0-9ee7-8f539ddebede', 'expense', 'aportes em corretora', 'canonical_name', 'high', false),
    ('6db34aab-8ff6-5239-bddd-57f3939fa04c', '33191a7c-77d1-5fc8-bb8e-65268997cc65', 'expense', 'outros investimentos', 'canonical_name', 'high', false),
    ('e3382783-0c0b-5108-9b27-e46b8b53ff06', '8f141d28-10c3-5a07-bfdf-4dfd79a049a1', 'income', 'ferias', 'canonical_name', 'high', false),
    ('90584260-492f-547c-b270-1fff1eba5c6c', 'bbc5809c-d567-59cf-80dd-e6f15b93b7e4', 'income', 'vale-alimentacao', 'canonical_name', 'high', false),
    ('f477ba22-b099-56ed-af45-f1c6dd5c7eeb', '6e896533-af52-5938-bc38-2152ea443af8', 'income', 'vale-refeicao', 'canonical_name', 'high', false),
    ('fcec290b-f6ae-5725-8db6-7bcc6824a827', 'dc2303d9-246e-53d6-8448-2adc19993b22', 'income', 'trabalho extra', 'canonical_name', 'high', false),
    ('dc73fa72-e410-5aa5-b611-bc896df29dc5', '0d613676-5f32-5412-9408-fde944bed128', 'income', 'consultoria', 'canonical_name', 'high', false),
    ('3495b7f7-f6a3-544c-89c5-f629a4beb44b', '8d812f21-fe17-57e5-a71d-5eb890d29bb6', 'income', 'juros', 'canonical_name', 'high', false),
    ('6b4be0e3-9487-5d68-9643-dbfa98689697', 'cac84f1b-70c6-5eb1-81d9-241764043d66', 'income', 'resgates', 'canonical_name', 'high', false),
    ('cdad0f44-00d5-5737-9ed2-80b5945a97c9', '1722bd29-031a-57d1-b4d8-2626d1971ce3', 'income', 'mesada recebida', 'canonical_name', 'high', false),
    ('b19bd233-eb4a-59e8-bee2-d14f6ad9e8ed', '52ded4b8-b082-5ec2-90fe-633c934edae7', 'income', 'marketplace', 'canonical_name', 'high', false),
    ('c080fc23-733d-5f56-ac72-5b7b995ae872', '9d996f66-81f2-5250-bb4d-bd3636e00544', 'income', 'outros', 'canonical_name', 'high', false),
    ('dd393937-4841-528d-8a40-b97356f412d3', 'f9d9e5b6-1437-5204-bd64-2bd7d43583a8', 'expense', 'financiamento imobiliario', 'canonical_name', 'high', false),
    ('89fcd304-e6a1-51ec-b057-b6b88ee4c0b8', '0c004f2d-ad42-5855-a408-f695906cd48c', 'expense', 'feira e hortifruti', 'canonical_name', 'high', false),
    ('499bb09b-4590-54d2-967d-20e95c8d3c8c', '007c090e-7a6d-5645-b751-b93cabb280ed', 'expense', 'transporte publico', 'canonical_name', 'high', false),
    ('f2459ebe-082a-5f13-9258-42eb0390fb88', 'c13dcc6e-c37b-521d-a889-8bb02765490f', 'expense', 'transporte por aplicativo recorrente', 'canonical_name', 'high', false),
    ('975f6cb1-4cf1-57ec-a1cb-3c32236493cf', 'bf2fcca0-09c3-5dcb-a61a-87eed2860c04', 'expense', 'manutencao veicular', 'canonical_name', 'high', false),
    ('fcdb3845-3769-5a72-ac5d-57544b88b8fb', '311c7b7f-56a3-5b53-ada7-5b85734ba45f', 'expense', 'ipva e licenciamento', 'canonical_name', 'high', false),
    ('bd3bbd48-b853-52d8-9eda-d66c7da56965', 'c8f579ea-952b-5e24-beed-ef22fb845a4b', 'expense', 'plano de saude', 'canonical_name', 'high', false),
    ('0dff0000-8dc1-5731-ba39-4b949e57bcde', '1af66343-7305-534f-b8de-47ebcd3d17f1', 'expense', 'plano odontologico', 'canonical_name', 'high', false),
    ('464ac949-819c-5889-a47e-185b3a475a85', 'af5619e0-3683-5b8c-b9fc-0b3ddfbd2075', 'expense', 'consultas e exames', 'canonical_name', 'high', false),
    ('db27fcc3-6e01-54a0-8ca1-bdcc713f2873', '157b18fe-513e-55fa-969c-c9bd785530d1', 'expense', 'medicamentos continuos', 'canonical_name', 'high', false),
    ('db6864a0-0fe7-59c2-863b-66df7bcd65e6', '3ca95dd5-c630-5c03-bd47-071777bde81c', 'expense', 'medicamentos e farmacia', 'canonical_name', 'high', false),
    ('c6e409cd-fc41-5314-b39d-e826000cdaab', 'a15cba16-23da-504e-a22b-144392ed82bc', 'expense', 'terapia e saude mental', 'canonical_name', 'high', false),
    ('4503ce0d-44f2-5809-bc6b-908413ab5ab7', 'cab69263-ac14-5ed1-ab5d-8372487c9ee8', 'expense', 'escola e creche', 'canonical_name', 'high', false),
    ('e6f173bd-eae9-5354-87e6-70b82f2a4aca', '46e492a0-3909-5e0a-bd3e-16bbdf29db8d', 'expense', 'faculdade e pos-graduacao', 'canonical_name', 'high', false),
    ('24db2177-c0a3-5e6a-a17c-82556074d99c', 'b3a4824f-e481-59fe-8f9e-0c33a59b5b5f', 'expense', 'cursos e treinamentos', 'canonical_name', 'high', false),
    ('3f61e3b3-24f7-5dd3-8e23-b3e1a86806a5', '01b51d39-347e-560c-ac07-d0a700f0c24f', 'expense', 'plataformas de ensino', 'canonical_name', 'high', false),
    ('18ab9777-75d0-53cb-b4c7-0f5db3c57f8b', 'bac52783-54ca-5401-92da-5afa29fc05d4', 'expense', 'livros e e-books', 'canonical_name', 'high', false),
    ('53244186-38cb-5452-b85d-1a75a59ed273', '3c5e9972-7f59-5f6b-aea4-ace59985cce0', 'expense', 'congressos e workshops', 'canonical_name', 'high', false),
    ('c2291e10-fa2c-5850-9bff-29e95c96295f', 'a371851d-56cb-551d-addb-022575b8d6e9', 'expense', 'bares e lanches', 'canonical_name', 'high', false),
    ('46218a91-1d44-56db-9a4e-7c45a5274c92', '85e56497-2e31-55d3-9516-376e61860708', 'expense', 'streaming de video', 'canonical_name', 'high', false),
    ('65931b20-dbcb-5ac4-b727-6f939a3b238e', '8580a31d-041d-5fa4-b86e-af90108af0cb', 'expense', 'musica e audio', 'canonical_name', 'high', false),
    ('0ffafc5a-253f-5a8f-8dea-ce39d1ddaef6', '514c00a0-ca41-5798-85d0-39992fbc223c', 'expense', 'games e assinaturas de jogos', 'canonical_name', 'high', false),
    ('03c7d56f-4fb9-5811-ae00-1a80fe059ac3', '5190df3d-8e6d-59bc-9e5b-d7e85e45154c', 'expense', 'cinema e teatro', 'canonical_name', 'high', false),
    ('732eb4d6-94ec-5392-aa89-13dd6b39cd6a', '09073cdd-4d58-5073-ae16-53ba2c3a4209', 'expense', 'shows e eventos', 'canonical_name', 'high', false),
    ('b3e379f1-d8cf-5433-8f33-0b14a3e20d89', 'aed45dcf-8fbe-5828-8fb6-87babd271d6c', 'expense', 'passeios e parques', 'canonical_name', 'high', false),
    ('bdbceef6-90cf-5aad-9664-55203977f238', '6feeb8fd-8faa-56d1-a0d0-d9d746e45f21', 'expense', 'transporte de lazer', 'canonical_name', 'high', false),
    ('2b3f8117-81a2-5826-84d0-95790e59dcd9', '0134668f-785b-5ac1-bcf5-e6c4f566de64', 'expense', 'viagens de lazer', 'canonical_name', 'high', false),
    ('8447e4e7-4b70-5c5b-b4b0-bb1fddcdbbd0', '7a69762f-6016-593a-9e62-f56f508ec9e1', 'expense', 'hospedagem de lazer', 'canonical_name', 'high', false),
    ('5a46f77a-48f0-5d12-9fa1-bfe1fca86a66', '14416063-f271-53e2-8a58-6682461ec532', 'expense', 'roupas e calcados', 'canonical_name', 'high', false),
    ('e90eac76-f8ea-5d2d-9132-4e46aec7c953', 'f9656739-8d1c-5675-8eaf-63a057137307', 'expense', 'beleza e estetica', 'canonical_name', 'high', false),
    ('91cb809e-32bd-550a-bb9b-734802095dfd', 'c0e10d9f-b0fe-59e7-8fb9-22a3bebd4784', 'expense', 'esportes e academia', 'canonical_name', 'high', false),
    ('13cb1311-da7d-5419-a2cd-1521bf622310', '61698c19-7281-5016-8cd3-b3799ddb575c', 'expense', 'casa e reforma', 'canonical_name', 'high', false),
    ('bcb6343c-31c7-52d2-bc8f-801b77ca1a5e', '8a4228f0-bc77-5d24-949d-5a7afa8063dc', 'expense', 'viagem planejada', 'canonical_name', 'high', false),
    ('7c35de8a-4adb-52f0-bf34-0ee4603a5a50', 'e91062ea-8bc9-5d30-a317-260faaf14e56', 'expense', 'familia e enxoval', 'canonical_name', 'high', false),
    ('70a49cfc-4e1e-5e9f-8419-0295b0339179', '946643a8-9e00-5bad-a860-f74ed74cf246', 'expense', 'quitacao de dividas', 'canonical_name', 'high', false),
    ('f6f263f9-06c6-5761-82d0-3ac6864d28ec', '45c7e533-fb00-50d9-aeb3-71bdb99098bd', 'expense', 'reserva de emergencia', 'canonical_name', 'high', false),
    ('f180a37a-8a1e-5399-b0e7-f75ba666443b', '9103a0e6-366b-5c77-a31d-e3ed58991d14', 'expense', 'tesouro direto', 'canonical_name', 'high', false),
    ('8ad5783c-da09-524d-ad82-d4805acb4df4', '1e5b4db2-b186-5524-b955-32553307d81c', 'expense', 'fundos imobiliarios', 'canonical_name', 'high', false),
    ('1aea28ca-2172-556c-9ddd-54bccb7ee69c', 'b1ac9b12-0b4d-5791-87d5-6628c9bbfa9a', 'expense', 'previdencia privada', 'canonical_name', 'high', false),
    ('72f73617-41ee-5b00-ae8d-eda8cd927a4d', 'da4f4c4c-864e-577b-9f4d-d7800f7a85ab', 'expense', 'investimentos internacionais', 'canonical_name', 'high', false),
    ('f13b70b6-5f8c-5f0e-9b23-ee5385290cc5', '98455e74-b1f3-5b9c-a8d8-05db0cdb465d', 'income', 'decimo terceiro', 'canonical_name', 'high', false),
    ('b1d3ad56-59ad-5137-a154-78971b08bc3f', '4b61504d-9cc2-579f-b927-d1963bd1e0ca', 'income', 'plr e bonus', 'canonical_name', 'high', false),
    ('cabe2dbd-7c47-50e2-87b6-292c3168cdff', '00b886a7-d221-592e-8068-fa296924b333', 'income', 'aluguel residencial recebido', 'canonical_name', 'high', false),
    ('146847ad-e672-5a5a-8379-e33e4fcce00e', '7a17fe1c-900c-57d8-a1dc-22bf9139cf83', 'income', 'restituicao de ir', 'canonical_name', 'high', false),
    ('487cd787-a938-5f64-8cdc-3c8f7fa9c87a', '06373332-7fb8-52b6-940e-f0c5699c6114', 'income', 'presentes em dinheiro', 'canonical_name', 'high', false),
    ('4ea8fd7d-4735-5486-89f9-3b33332f72f3', '6650a195-013b-5808-8845-22a0657da9ba', 'income', 'vendas diversas', 'canonical_name', 'high', false)
ON CONFLICT (id) DO NOTHING;

UPDATE mecontrola.category_editorial_version SET version = version + 1, updated_at = now();

-- ============================================================
-- Category Dictionary: Bulk seed v2 (consolidated from 000008)
-- ============================================================

SET LOCAL lock_timeout    = '5s';
SET LOCAL statement_timeout = '120s';

-- Bulk seed v2 — aliases PT-BR de alto uso para Telegram/WhatsApp parsing.
-- Mapeia somente para subcategorias EXISTENTES; falha cedo se subcategoria sumir.
-- Marca inserts com nota 'seed_v2' via signal_type/created_at; down remove por id determinístico.

DO $$
DECLARE
    v_missing TEXT[];
BEGIN
    v_missing := ARRAY[]::TEXT[];

    IF NOT EXISTS (SELECT 1 FROM mecontrola.categories WHERE id = 'ddbb0dc7-8b85-5177-8cfc-3bb2aed6c75c') THEN
        v_missing := array_append(v_missing, 'prazeres/delivery');
    END IF;
    IF NOT EXISTS (SELECT 1 FROM mecontrola.categories WHERE id = 'd539672d-961f-5553-b807-0e0156a63163') THEN
        v_missing := array_append(v_missing, 'prazeres/restaurantes');
    END IF;
    IF NOT EXISTS (SELECT 1 FROM mecontrola.categories WHERE id = 'a371851d-56cb-551d-addb-022575b8d6e9') THEN
        v_missing := array_append(v_missing, 'prazeres/bares-e-lanches');
    END IF;
    IF NOT EXISTS (SELECT 1 FROM mecontrola.categories WHERE id = '97fa4b86-d43c-5ad5-a99b-c88c8427fb30') THEN
        v_missing := array_append(v_missing, 'custo-fixo/supermercado');
    END IF;
    IF NOT EXISTS (SELECT 1 FROM mecontrola.categories WHERE id = '0c004f2d-ad42-5855-a408-f695906cd48c') THEN
        v_missing := array_append(v_missing, 'custo-fixo/feira-e-hortifruti');
    END IF;
    IF NOT EXISTS (SELECT 1 FROM mecontrola.categories WHERE id = 'cb13d50d-43cb-553c-99cd-8851889d7f6e') THEN
        v_missing := array_append(v_missing, 'custo-fixo/combustivel');
    END IF;
    IF NOT EXISTS (SELECT 1 FROM mecontrola.categories WHERE id = 'c13dcc6e-c37b-521d-a889-8bb02765490f') THEN
        v_missing := array_append(v_missing, 'custo-fixo/transporte-por-aplicativo-recorrente');
    END IF;
    IF NOT EXISTS (SELECT 1 FROM mecontrola.categories WHERE id = '36916fab-eacc-50a3-8a53-93671c335952') THEN
        v_missing := array_append(v_missing, 'custo-fixo/energia');
    END IF;
    IF NOT EXISTS (SELECT 1 FROM mecontrola.categories WHERE id = 'fa93273d-e2d9-54ed-a6aa-53b5b1830867') THEN
        v_missing := array_append(v_missing, 'custo-fixo/agua');
    END IF;
    IF NOT EXISTS (SELECT 1 FROM mecontrola.categories WHERE id = '4e6f8b6b-8ffb-5d38-8ac9-68464679a544') THEN
        v_missing := array_append(v_missing, 'custo-fixo/gas');
    END IF;
    IF NOT EXISTS (SELECT 1 FROM mecontrola.categories WHERE id = 'd0b1fa13-d19f-51b9-afc7-82bf83accf79') THEN
        v_missing := array_append(v_missing, 'custo-fixo/condominio');
    END IF;
    IF NOT EXISTS (SELECT 1 FROM mecontrola.categories WHERE id = '9391ac38-ec2c-55d0-afc8-8c0940678814') THEN
        v_missing := array_append(v_missing, 'custo-fixo/internet');
    END IF;
    IF NOT EXISTS (SELECT 1 FROM mecontrola.categories WHERE id = '7319ba14-0dc7-56ff-ac5c-96024e15ec02') THEN
        v_missing := array_append(v_missing, 'custo-fixo/telefonia');
    END IF;
    IF NOT EXISTS (SELECT 1 FROM mecontrola.categories WHERE id = '2e90fdd3-1008-5423-8215-5db1880fa60b') THEN
        v_missing := array_append(v_missing, 'custo-fixo/tv-por-assinatura');
    END IF;
    IF NOT EXISTS (SELECT 1 FROM mecontrola.categories WHERE id = 'b3a4824f-e481-59fe-8f9e-0c33a59b5b5f') THEN
        v_missing := array_append(v_missing, 'conhecimento/cursos-e-treinamentos');
    END IF;
    IF NOT EXISTS (SELECT 1 FROM mecontrola.categories WHERE id = '01b51d39-347e-560c-ac07-d0a700f0c24f') THEN
        v_missing := array_append(v_missing, 'conhecimento/plataformas-de-ensino');
    END IF;
    IF NOT EXISTS (SELECT 1 FROM mecontrola.categories WHERE id = 'bac52783-54ca-5401-92da-5afa29fc05d4') THEN
        v_missing := array_append(v_missing, 'conhecimento/livros-e-ebooks');
    END IF;
    IF NOT EXISTS (SELECT 1 FROM mecontrola.categories WHERE id = '9103a0e6-366b-5c77-a31d-e3ed58991d14') THEN
        v_missing := array_append(v_missing, 'liberdade-financeira/tesouro-direto');
    END IF;
    IF NOT EXISTS (SELECT 1 FROM mecontrola.categories WHERE id = 'ee26c4d9-ca74-5537-80b9-4d90815b9c06') THEN
        v_missing := array_append(v_missing, 'liberdade-financeira/cdb-e-rdb');
    END IF;
    IF NOT EXISTS (SELECT 1 FROM mecontrola.categories WHERE id = 'd35da3b9-65c5-55b8-9915-13354e202644') THEN
        v_missing := array_append(v_missing, 'liberdade-financeira/lci-e-lca');
    END IF;
    IF NOT EXISTS (SELECT 1 FROM mecontrola.categories WHERE id = '1e5b4db2-b186-5524-b955-32553307d81c') THEN
        v_missing := array_append(v_missing, 'liberdade-financeira/fundos-imobiliarios');
    END IF;
    IF NOT EXISTS (SELECT 1 FROM mecontrola.categories WHERE id = '866793cb-4059-54b0-9ee7-8f539ddebede') THEN
        v_missing := array_append(v_missing, 'liberdade-financeira/aportes-em-corretora');
    END IF;
    IF NOT EXISTS (SELECT 1 FROM mecontrola.categories WHERE id = '45c7e533-fb00-50d9-aeb3-71bdb99098bd') THEN
        v_missing := array_append(v_missing, 'liberdade-financeira/reserva-de-emergencia');
    END IF;
    IF NOT EXISTS (SELECT 1 FROM mecontrola.categories WHERE id = '0134668f-785b-5ac1-bcf5-e6c4f566de64') THEN
        v_missing := array_append(v_missing, 'prazeres/viagens-de-lazer');
    END IF;
    IF NOT EXISTS (SELECT 1 FROM mecontrola.categories WHERE id = '7a69762f-6016-593a-9e62-f56f508ec9e1') THEN
        v_missing := array_append(v_missing, 'prazeres/hospedagem-de-lazer');
    END IF;
    IF NOT EXISTS (SELECT 1 FROM mecontrola.categories WHERE id = '8a4228f0-bc77-5d24-949d-5a7afa8063dc') THEN
        v_missing := array_append(v_missing, 'metas/viagem-planejada');
    END IF;

    IF array_length(v_missing, 1) IS NOT NULL THEN
        RAISE EXCEPTION 'seed_v2: subcategorias ausentes: %', v_missing;
    END IF;
END $$;

INSERT INTO mecontrola.category_dictionary (id, category_id, kind, term, signal_type, confidence, is_ambiguous)
SELECT
    src.id::uuid,
    src.category_id::uuid,
    'expense',
    src.term,
    src.signal_type,
    src.confidence,
    src.is_ambiguous
FROM (VALUES
    ('a1b00001-0000-5007-0000-000000000001', 'ddbb0dc7-8b85-5177-8cfc-3bb2aed6c75c', 'ifood',           'merchant', 'high',   false),
    ('a1b00001-0000-5007-0000-000000000002', 'ddbb0dc7-8b85-5177-8cfc-3bb2aed6c75c', 'ifoods',          'merchant', 'high',   false),
    ('a1b00001-0000-5007-0000-000000000003', 'ddbb0dc7-8b85-5177-8cfc-3bb2aed6c75c', 'rappi',           'merchant', 'high',   false),
    ('a1b00001-0000-5007-0000-000000000004', 'ddbb0dc7-8b85-5177-8cfc-3bb2aed6c75c', 'ubereats',        'merchant', 'high',   false),
    ('a1b00001-0000-5007-0000-000000000005', 'ddbb0dc7-8b85-5177-8cfc-3bb2aed6c75c', 'uber eats',       'merchant', 'high',   false),
    ('a1b00001-0000-5007-0000-000000000006', 'ddbb0dc7-8b85-5177-8cfc-3bb2aed6c75c', 'james delivery',  'merchant', 'high',   false),
    ('a1b00001-0000-5007-0000-000000000007', 'ddbb0dc7-8b85-5177-8cfc-3bb2aed6c75c', '99food',          'merchant', 'high',   false),
    ('a1b00001-0000-5007-0000-000000000008', 'ddbb0dc7-8b85-5177-8cfc-3bb2aed6c75c', 'dieta delivery',  'merchant', 'high',   false),
    ('a1b00001-0000-5007-0000-000000000009', 'ddbb0dc7-8b85-5177-8cfc-3bb2aed6c75c', 'delivery',        'alias',    'high',   false),
    ('a1b00001-0000-5007-0000-00000000000a', 'ddbb0dc7-8b85-5177-8cfc-3bb2aed6c75c', 'aiqfome',         'merchant', 'high',   false),

    ('a1b00001-0000-5007-0000-000000000020', 'd539672d-961f-5553-b807-0e0156a63163', 'lanchonete',      'alias',    'high',   false),
    ('a1b00001-0000-5007-0000-000000000021', 'd539672d-961f-5553-b807-0e0156a63163', 'sushi',           'alias',    'high',   false),
    ('a1b00001-0000-5007-0000-000000000022', 'd539672d-961f-5553-b807-0e0156a63163', 'churrascaria',    'alias',    'high',   false),
    ('a1b00001-0000-5007-0000-000000000023', 'd539672d-961f-5553-b807-0e0156a63163', 'rodizio',         'alias',    'high',   false),
    ('a1b00001-0000-5007-0000-000000000024', 'a371851d-56cb-551d-addb-022575b8d6e9', 'mc donalds',      'merchant', 'high',   false),
    ('a1b00001-0000-5007-0000-000000000025', 'a371851d-56cb-551d-addb-022575b8d6e9', 'mcdonalds',       'merchant', 'high',   false),
    ('a1b00001-0000-5007-0000-000000000026', 'a371851d-56cb-551d-addb-022575b8d6e9', 'burger king',     'merchant', 'high',   false),
    ('a1b00001-0000-5007-0000-000000000027', 'a371851d-56cb-551d-addb-022575b8d6e9', 'bk',              'merchant', 'medium', true),
    ('a1b00001-0000-5007-0000-000000000028', 'a371851d-56cb-551d-addb-022575b8d6e9', 'kfc',             'merchant', 'high',   false),
    ('a1b00001-0000-5007-0000-000000000029', 'a371851d-56cb-551d-addb-022575b8d6e9', 'subway',          'merchant', 'high',   false),
    ('a1b00001-0000-5007-0000-00000000002a', 'a371851d-56cb-551d-addb-022575b8d6e9', 'habibs',          'merchant', 'high',   false),
    ('a1b00001-0000-5007-0000-00000000002b', 'a371851d-56cb-551d-addb-022575b8d6e9', 'bobs',            'merchant', 'high',   false),

    ('a1b00001-0000-5007-0000-000000000040', '97fa4b86-d43c-5ad5-a99b-c88c8427fb30', 'mercado',           'alias',    'high',   true),
    ('a1b00001-0000-5007-0000-000000000041', '97fa4b86-d43c-5ad5-a99b-c88c8427fb30', 'compra do mercado', 'phrase',   'high',   false),
    ('a1b00001-0000-5007-0000-000000000042', '97fa4b86-d43c-5ad5-a99b-c88c8427fb30', 'atacadao',          'merchant', 'high',   false),
    ('a1b00001-0000-5007-0000-000000000043', '97fa4b86-d43c-5ad5-a99b-c88c8427fb30', 'carrefour',         'merchant', 'high',   false),
    ('a1b00001-0000-5007-0000-000000000044', '97fa4b86-d43c-5ad5-a99b-c88c8427fb30', 'extra',             'merchant', 'medium', true),
    ('a1b00001-0000-5007-0000-000000000045', '97fa4b86-d43c-5ad5-a99b-c88c8427fb30', 'pao de acucar',     'merchant', 'high',   false),
    ('a1b00001-0000-5007-0000-000000000046', '97fa4b86-d43c-5ad5-a99b-c88c8427fb30', 'assai',             'merchant', 'high',   false),
    ('a1b00001-0000-5007-0000-000000000047', '97fa4b86-d43c-5ad5-a99b-c88c8427fb30', 'sams club',         'merchant', 'high',   false),
    ('a1b00001-0000-5007-0000-000000000048', '97fa4b86-d43c-5ad5-a99b-c88c8427fb30', 'big',               'merchant', 'medium', true),
    ('a1b00001-0000-5007-0000-000000000049', '97fa4b86-d43c-5ad5-a99b-c88c8427fb30', 'dia',               'merchant', 'medium', true),

    ('a1b00001-0000-5007-0000-000000000060', 'cb13d50d-43cb-553c-99cd-8851889d7f6e', 'posto',           'alias',    'high',   true),
    ('a1b00001-0000-5007-0000-000000000061', 'cb13d50d-43cb-553c-99cd-8851889d7f6e', 'alcool',          'alias',    'high',   false),
    ('a1b00001-0000-5007-0000-000000000062', 'cb13d50d-43cb-553c-99cd-8851889d7f6e', 'shell',           'merchant', 'high',   false),
    ('a1b00001-0000-5007-0000-000000000063', 'cb13d50d-43cb-553c-99cd-8851889d7f6e', 'ipiranga',        'merchant', 'high',   false),
    ('a1b00001-0000-5007-0000-000000000064', 'cb13d50d-43cb-553c-99cd-8851889d7f6e', 'petrobras',       'merchant', 'high',   false),
    ('a1b00001-0000-5007-0000-000000000065', 'cb13d50d-43cb-553c-99cd-8851889d7f6e', 'br mania',        'merchant', 'high',   false),
    ('a1b00001-0000-5007-0000-000000000066', 'cb13d50d-43cb-553c-99cd-8851889d7f6e', 'abastecer',       'alias',    'high',   false),

    ('a1b00001-0000-5007-0000-000000000080', 'c13dcc6e-c37b-521d-a889-8bb02765490f', 'indrive',         'merchant', 'high',   false),
    ('a1b00001-0000-5007-0000-000000000081', 'c13dcc6e-c37b-521d-a889-8bb02765490f', 'taxi',            'alias',    'high',   false),
    ('a1b00001-0000-5007-0000-000000000082', 'c13dcc6e-c37b-521d-a889-8bb02765490f', '99 pop',          'merchant', 'high',   false),
    ('a1b00001-0000-5007-0000-000000000083', 'c13dcc6e-c37b-521d-a889-8bb02765490f', 'cabify',          'merchant', 'high',   false),
    ('a1b00001-0000-5007-0000-000000000084', 'c13dcc6e-c37b-521d-a889-8bb02765490f', 'uber x',          'merchant', 'high',   false),

    ('a1b00001-0000-5007-0000-0000000000a0', '36916fab-eacc-50a3-8a53-93671c335952', 'luz',             'alias',    'high',   false),
    ('a1b00001-0000-5007-0000-0000000000a1', '36916fab-eacc-50a3-8a53-93671c335952', 'enel',            'merchant', 'high',   false),
    ('a1b00001-0000-5007-0000-0000000000a2', '36916fab-eacc-50a3-8a53-93671c335952', 'cpfl',            'merchant', 'high',   false),
    ('a1b00001-0000-5007-0000-0000000000a3', '36916fab-eacc-50a3-8a53-93671c335952', 'light',           'merchant', 'high',   false),
    ('a1b00001-0000-5007-0000-0000000000a4', 'fa93273d-e2d9-54ed-a6aa-53b5b1830867', 'agua',            'alias',    'high',   false),
    ('a1b00001-0000-5007-0000-0000000000a5', 'fa93273d-e2d9-54ed-a6aa-53b5b1830867', 'sabesp',          'merchant', 'high',   false),
    ('a1b00001-0000-5007-0000-0000000000a6', '9391ac38-ec2c-55d0-afc8-8c0940678814', 'vivo fibra',      'merchant', 'high',   false),
    ('a1b00001-0000-5007-0000-0000000000a7', '9391ac38-ec2c-55d0-afc8-8c0940678814', 'claro net',       'merchant', 'high',   false),
    ('a1b00001-0000-5007-0000-0000000000a8', '7319ba14-0dc7-56ff-ac5c-96024e15ec02', 'vivo',            'merchant', 'medium', true),
    ('a1b00001-0000-5007-0000-0000000000a9', '7319ba14-0dc7-56ff-ac5c-96024e15ec02', 'claro',           'merchant', 'medium', true),
    ('a1b00001-0000-5007-0000-0000000000aa', '7319ba14-0dc7-56ff-ac5c-96024e15ec02', 'tim',             'merchant', 'medium', true),
    ('a1b00001-0000-5007-0000-0000000000ab', '7319ba14-0dc7-56ff-ac5c-96024e15ec02', 'oi',              'merchant', 'medium', true),
    ('a1b00001-0000-5007-0000-0000000000ac', '7319ba14-0dc7-56ff-ac5c-96024e15ec02', 'celular',         'alias',    'medium', true),
    ('a1b00001-0000-5007-0000-0000000000ad', '2e90fdd3-1008-5423-8215-5db1880fa60b', 'sky',             'merchant', 'high',   false),
    ('a1b00001-0000-5007-0000-0000000000ae', '2e90fdd3-1008-5423-8215-5db1880fa60b', 'directv',         'merchant', 'high',   false),

    ('a1b00001-0000-5007-0000-0000000000c0', 'b3a4824f-e481-59fe-8f9e-0c33a59b5b5f', 'curso',           'alias',    'high',   true),
    ('a1b00001-0000-5007-0000-0000000000c1', 'b3a4824f-e481-59fe-8f9e-0c33a59b5b5f', 'cursos',          'alias',    'high',   false),
    ('a1b00001-0000-5007-0000-0000000000c2', 'b3a4824f-e481-59fe-8f9e-0c33a59b5b5f', 'dominio',         'alias',    'low',    true),
    ('a1b00001-0000-5007-0000-0000000000c3', '01b51d39-347e-560c-ac07-d0a700f0c24f', 'alura',           'merchant', 'high',   false),
    ('a1b00001-0000-5007-0000-0000000000c4', '01b51d39-347e-560c-ac07-d0a700f0c24f', 'udemy',           'merchant', 'high',   false),
    ('a1b00001-0000-5007-0000-0000000000c5', '01b51d39-347e-560c-ac07-d0a700f0c24f', 'hotmart',         'merchant', 'high',   false),
    ('a1b00001-0000-5007-0000-0000000000c6', '01b51d39-347e-560c-ac07-d0a700f0c24f', 'coursera',        'merchant', 'high',   false),
    ('a1b00001-0000-5007-0000-0000000000c7', '01b51d39-347e-560c-ac07-d0a700f0c24f', 'edx',             'merchant', 'high',   false),
    ('a1b00001-0000-5007-0000-0000000000c8', '01b51d39-347e-560c-ac07-d0a700f0c24f', 'rocketseat',      'merchant', 'high',   false),
    ('a1b00001-0000-5007-0000-0000000000c9', '01b51d39-347e-560c-ac07-d0a700f0c24f', 'kultivi',         'merchant', 'high',   false),
    ('a1b00001-0000-5007-0000-0000000000ca', 'bac52783-54ca-5401-92da-5afa29fc05d4', 'audible',         'merchant', 'high',   false),
    ('a1b00001-0000-5007-0000-0000000000cb', 'bac52783-54ca-5401-92da-5afa29fc05d4', 'amazon kindle',   'merchant', 'high',   false),
    ('a1b00001-0000-5007-0000-0000000000cc', 'bac52783-54ca-5401-92da-5afa29fc05d4', 'kindle unlimited','merchant', 'high',   false),

    ('a1b00001-0000-5007-0000-0000000000e0', '866793cb-4059-54b0-9ee7-8f539ddebede', 'investimento',    'alias',    'high',   true),
    ('a1b00001-0000-5007-0000-0000000000e1', '866793cb-4059-54b0-9ee7-8f539ddebede', 'aporte',          'alias',    'high',   false),
    ('a1b00001-0000-5007-0000-0000000000e2', '866793cb-4059-54b0-9ee7-8f539ddebede', 'b3',              'alias',    'high',   false),
    ('a1b00001-0000-5007-0000-0000000000e3', '866793cb-4059-54b0-9ee7-8f539ddebede', 'xp',              'merchant', 'high',   false),
    ('a1b00001-0000-5007-0000-0000000000e4', '866793cb-4059-54b0-9ee7-8f539ddebede', 'rico',            'merchant', 'high',   false),
    ('a1b00001-0000-5007-0000-0000000000e5', '866793cb-4059-54b0-9ee7-8f539ddebede', 'modal',           'merchant', 'high',   false),
    ('a1b00001-0000-5007-0000-0000000000e6', '866793cb-4059-54b0-9ee7-8f539ddebede', 'nuinvest',        'merchant', 'high',   false),
    ('a1b00001-0000-5007-0000-0000000000e7', '866793cb-4059-54b0-9ee7-8f539ddebede', 'inter invest',    'merchant', 'high',   false),
    ('a1b00001-0000-5007-0000-0000000000e8', '866793cb-4059-54b0-9ee7-8f539ddebede', 'clear',           'merchant', 'medium', true),
    ('a1b00001-0000-5007-0000-0000000000e9', '9103a0e6-366b-5c77-a31d-e3ed58991d14', 'tesouro',         'alias',    'high',   false),
    ('a1b00001-0000-5007-0000-0000000000ea', 'ee26c4d9-ca74-5537-80b9-4d90815b9c06', 'cdb',             'alias',    'high',   false),
    ('a1b00001-0000-5007-0000-0000000000eb', 'd35da3b9-65c5-55b8-9915-13354e202644', 'lci',             'alias',    'high',   false),
    ('a1b00001-0000-5007-0000-0000000000ec', 'd35da3b9-65c5-55b8-9915-13354e202644', 'lca',             'alias',    'high',   false),
    ('a1b00001-0000-5007-0000-0000000000ed', '1e5b4db2-b186-5524-b955-32553307d81c', 'fundos imobiliarios', 'phrase', 'high', false),
    ('a1b00001-0000-5007-0000-0000000000ee', '45c7e533-fb00-50d9-aeb3-71bdb99098bd', 'reserva',         'alias',    'medium', true),
    ('a1b00001-0000-5007-0000-0000000000ef', '45c7e533-fb00-50d9-aeb3-71bdb99098bd', 'emergencia',      'alias',    'high',   false),
    ('a1b00001-0000-5007-0000-0000000000f0', '45c7e533-fb00-50d9-aeb3-71bdb99098bd', 'poupanca',        'alias',    'high',   false),
    ('a1b00001-0000-5007-0000-0000000000f1', '45c7e533-fb00-50d9-aeb3-71bdb99098bd', 'poupanca digital','phrase',   'high',   false),

    ('a1b00001-0000-5007-0000-000000000100', '8a4228f0-bc77-5d24-949d-5a7afa8063dc', 'viagem',          'alias',    'high',   true),
    ('a1b00001-0000-5007-0000-000000000101', '7a69762f-6016-593a-9e62-f56f508ec9e1', 'hotel',           'alias',    'high',   true),
    ('a1b00001-0000-5007-0000-000000000102', '7a69762f-6016-593a-9e62-f56f508ec9e1', 'airbnb',          'merchant', 'high',   false),
    ('a1b00001-0000-5007-0000-000000000103', '7a69762f-6016-593a-9e62-f56f508ec9e1', 'booking',         'merchant', 'high',   false),
    ('a1b00001-0000-5007-0000-000000000104', '7a69762f-6016-593a-9e62-f56f508ec9e1', 'decolar',         'merchant', 'high',   false),
    ('a1b00001-0000-5007-0000-000000000105', '0134668f-785b-5ac1-bcf5-e6c4f566de64', 'latam',           'merchant', 'high',   false),
    ('a1b00001-0000-5007-0000-000000000106', '0134668f-785b-5ac1-bcf5-e6c4f566de64', 'gol',             'merchant', 'medium', true),
    ('a1b00001-0000-5007-0000-000000000107', '0134668f-785b-5ac1-bcf5-e6c4f566de64', 'azul',            'merchant', 'medium', true),
    ('a1b00001-0000-5007-0000-000000000108', '0134668f-785b-5ac1-bcf5-e6c4f566de64', 'smiles',          'merchant', 'high',   false),
    ('a1b00001-0000-5007-0000-000000000109', '0134668f-785b-5ac1-bcf5-e6c4f566de64', 'miles',           'alias',    'medium', true),
    ('a1b00001-0000-5007-0000-00000000010a', '0134668f-785b-5ac1-bcf5-e6c4f566de64', 'tudo azul',       'merchant', 'high',   false),
    ('a1b00001-0000-5007-0000-00000000010b', '0134668f-785b-5ac1-bcf5-e6c4f566de64', 'latam pass',      'merchant', 'high',   false),
    ('a1b00001-0000-5007-0000-00000000010c', '0134668f-785b-5ac1-bcf5-e6c4f566de64', 'passagem aerea',  'phrase',   'high',   false)
) AS src(id, category_id, term, signal_type, confidence, is_ambiguous)
WHERE NOT EXISTS (
    SELECT 1
    FROM mecontrola.category_dictionary d
    WHERE d.kind = 'expense'
      AND d.category_id = src.category_id::uuid
      AND d.term_normalized = lower(mecontrola.immutable_unaccent(src.term))
      AND d.deprecated_at IS NULL
);

UPDATE mecontrola.category_editorial_version SET version = version + 1, updated_at = now();

ANALYZE;

INSERT INTO mecontrola.category_dictionary (id, category_id, kind, term, signal_type, confidence, is_ambiguous)
SELECT v.id, v.category_id, v.kind, v.term, v.signal_type, v.confidence, v.is_ambiguous
FROM (VALUES
    ('d1c10000-0000-4000-8000-000000000001'::uuid, '97fa4b86-d43c-5ad5-a99b-c88c8427fb30'::uuid, 'expense', 'mercado', 'alias', 'high', false),
    ('d1c10000-0000-4000-8000-000000000002'::uuid, '97fa4b86-d43c-5ad5-a99b-c88c8427fb30'::uuid, 'expense', 'mercadinho', 'alias', 'high', false),
    ('d1c10000-0000-4000-8000-000000000003'::uuid, '0c004f2d-ad42-5855-a408-f695906cd48c'::uuid, 'expense', 'feira', 'alias', 'high', false),
    ('d1c10000-0000-4000-8000-000000000004'::uuid, '0c004f2d-ad42-5855-a408-f695906cd48c'::uuid, 'expense', 'hortifruti', 'alias', 'high', false),
    ('d1c10000-0000-4000-8000-000000000005'::uuid, '36916fab-eacc-50a3-8a53-93671c335952'::uuid, 'expense', 'luz', 'alias', 'high', false),
    ('d1c10000-0000-4000-8000-000000000006'::uuid, '36916fab-eacc-50a3-8a53-93671c335952'::uuid, 'expense', 'conta de luz', 'alias', 'high', false),
    ('d1c10000-0000-4000-8000-000000000007'::uuid, '36916fab-eacc-50a3-8a53-93671c335952'::uuid, 'expense', 'energia eletrica', 'alias', 'high', false),
    ('d1c10000-0000-4000-8000-000000000008'::uuid, 'fa93273d-e2d9-54ed-a6aa-53b5b1830867'::uuid, 'expense', 'conta de agua', 'alias', 'high', false),
    ('d1c10000-0000-4000-8000-000000000009'::uuid, '9391ac38-ec2c-55d0-afc8-8c0940678814'::uuid, 'expense', 'wifi', 'alias', 'high', false),
    ('d1c10000-0000-4000-8000-00000000000a'::uuid, '9391ac38-ec2c-55d0-afc8-8c0940678814'::uuid, 'expense', 'banda larga', 'alias', 'high', false),
    ('d1c10000-0000-4000-8000-00000000000b'::uuid, 'cb13d50d-43cb-553c-99cd-8851889d7f6e'::uuid, 'expense', 'gasolina', 'alias', 'high', false),
    ('d1c10000-0000-4000-8000-00000000000c'::uuid, '4e6f8b6b-8ffb-5d38-8ac9-68464679a544'::uuid, 'expense', 'gas de cozinha', 'alias', 'high', false),
    ('d1c10000-0000-4000-8000-00000000000d'::uuid, '4e6f8b6b-8ffb-5d38-8ac9-68464679a544'::uuid, 'expense', 'botijao', 'alias', 'high', false),
    ('d1c10000-0000-4000-8000-00000000000e'::uuid, '311c7b7f-56a3-5b53-ada7-5b85734ba45f'::uuid, 'expense', 'ipva', 'alias', 'high', false),
    ('d1c10000-0000-4000-8000-00000000000f'::uuid, '311c7b7f-56a3-5b53-ada7-5b85734ba45f'::uuid, 'expense', 'licenciamento', 'alias', 'high', false),
    ('d1c10000-0000-4000-8000-000000000010'::uuid, 'c8f579ea-952b-5e24-beed-ef22fb845a4b'::uuid, 'expense', 'convenio medico', 'alias', 'high', false),
    ('d1c10000-0000-4000-8000-000000000011'::uuid, '3ca95dd5-c630-5c03-bd47-071777bde81c'::uuid, 'expense', 'farmacia', 'alias', 'high', false),
    ('d1c10000-0000-4000-8000-000000000012'::uuid, '3ca95dd5-c630-5c03-bd47-071777bde81c'::uuid, 'expense', 'remedio', 'alias', 'high', false),
    ('d1c10000-0000-4000-8000-000000000013'::uuid, '4ded7fd4-5335-5cf2-aed1-bdcead596000'::uuid, 'expense', 'dentista', 'alias', 'high', false),
    ('d1c10000-0000-4000-8000-000000000014'::uuid, 'a15cba16-23da-504e-a22b-144392ed82bc'::uuid, 'expense', 'psicologo', 'alias', 'high', false),
    ('d1c10000-0000-4000-8000-000000000015'::uuid, 'cab69263-ac14-5ed1-ab5d-8372487c9ee8'::uuid, 'expense', 'escola', 'alias', 'high', false),
    ('d1c10000-0000-4000-8000-000000000016'::uuid, 'cab69263-ac14-5ed1-ab5d-8372487c9ee8'::uuid, 'expense', 'creche', 'alias', 'high', false),
    ('d1c10000-0000-4000-8000-000000000017'::uuid, '7d56377d-bdd0-5152-9b94-10639bc7f39b'::uuid, 'expense', 'imposto', 'alias', 'high', false),
    ('d1c10000-0000-4000-8000-000000000018'::uuid, 'b29895dd-f1c5-5375-a5d1-082d9e2c3620'::uuid, 'expense', 'emprestimo', 'alias', 'high', false),
    ('d1c10000-0000-4000-8000-000000000019'::uuid, '347e0488-a4a7-55e8-8882-ae868c9d749d'::uuid, 'expense', 'tarifa bancaria', 'alias', 'high', false),
    ('d1c10000-0000-4000-8000-00000000001a'::uuid, 'd0b1fa13-d19f-51b9-afc7-82bf83accf79'::uuid, 'expense', 'taxa de condominio', 'alias', 'high', false),
    ('d1c10000-0000-4000-8000-00000000001b'::uuid, 'b3a4824f-e481-59fe-8f9e-0c33a59b5b5f'::uuid, 'expense', 'curso', 'alias', 'high', false),
    ('d1c10000-0000-4000-8000-00000000001c'::uuid, 'b3a4824f-e481-59fe-8f9e-0c33a59b5b5f'::uuid, 'expense', 'treinamento', 'alias', 'high', false),
    ('d1c10000-0000-4000-8000-00000000001d'::uuid, '46e492a0-3909-5e0a-bd3e-16bbdf29db8d'::uuid, 'expense', 'faculdade', 'alias', 'high', false),
    ('d1c10000-0000-4000-8000-00000000001e'::uuid, '46e492a0-3909-5e0a-bd3e-16bbdf29db8d'::uuid, 'expense', 'pos graduacao', 'alias', 'high', false),
    ('d1c10000-0000-4000-8000-00000000001f'::uuid, 'bac52783-54ca-5401-92da-5afa29fc05d4'::uuid, 'expense', 'livro', 'alias', 'high', false),
    ('d1c10000-0000-4000-8000-000000000020'::uuid, 'bac52783-54ca-5401-92da-5afa29fc05d4'::uuid, 'expense', 'ebook', 'alias', 'high', false),
    ('d1c10000-0000-4000-8000-000000000021'::uuid, 'fec9aed9-2699-538e-bbae-eb4bcdfb1ce3'::uuid, 'expense', 'ingles', 'alias', 'high', false),
    ('d1c10000-0000-4000-8000-000000000022'::uuid, '85e56497-2e31-55d3-9516-376e61860708'::uuid, 'expense', 'netflix', 'alias', 'high', false),
    ('d1c10000-0000-4000-8000-000000000023'::uuid, '85e56497-2e31-55d3-9516-376e61860708'::uuid, 'expense', 'disney plus', 'alias', 'high', false),
    ('d1c10000-0000-4000-8000-000000000024'::uuid, '85e56497-2e31-55d3-9516-376e61860708'::uuid, 'expense', 'hbo max', 'alias', 'high', false),
    ('d1c10000-0000-4000-8000-000000000025'::uuid, '85e56497-2e31-55d3-9516-376e61860708'::uuid, 'expense', 'prime video', 'alias', 'high', false),
    ('d1c10000-0000-4000-8000-000000000026'::uuid, '8580a31d-041d-5fa4-b86e-af90108af0cb'::uuid, 'expense', 'spotify', 'alias', 'high', false),
    ('d1c10000-0000-4000-8000-000000000027'::uuid, '8580a31d-041d-5fa4-b86e-af90108af0cb'::uuid, 'expense', 'deezer', 'alias', 'high', false),
    ('d1c10000-0000-4000-8000-000000000028'::uuid, 'c0e10d9f-b0fe-59e7-8fb9-22a3bebd4784'::uuid, 'expense', 'academia', 'alias', 'high', false),
    ('d1c10000-0000-4000-8000-000000000029'::uuid, 'c0e10d9f-b0fe-59e7-8fb9-22a3bebd4784'::uuid, 'expense', 'personal trainer', 'alias', 'high', false),
    ('d1c10000-0000-4000-8000-00000000002a'::uuid, 'ddbb0dc7-8b85-5177-8cfc-3bb2aed6c75c'::uuid, 'expense', 'ifood', 'merchant', 'medium', false),
    ('d1c10000-0000-4000-8000-00000000002b'::uuid, 'ddbb0dc7-8b85-5177-8cfc-3bb2aed6c75c'::uuid, 'expense', 'rappi', 'merchant', 'medium', false),
    ('d1c10000-0000-4000-8000-00000000002c'::uuid, '5190df3d-8e6d-59bc-9e5b-d7e85e45154c'::uuid, 'expense', 'cinema', 'alias', 'high', false),
    ('d1c10000-0000-4000-8000-00000000002d'::uuid, '09073cdd-4d58-5073-ae16-53ba2c3a4209'::uuid, 'expense', 'show', 'alias', 'high', false),
    ('d1c10000-0000-4000-8000-00000000002e'::uuid, 'f9656739-8d1c-5675-8eaf-63a057137307'::uuid, 'expense', 'cabeleireiro', 'alias', 'high', false),
    ('d1c10000-0000-4000-8000-00000000002f'::uuid, 'f9656739-8d1c-5675-8eaf-63a057137307'::uuid, 'expense', 'barbearia', 'alias', 'high', false),
    ('d1c10000-0000-4000-8000-000000000030'::uuid, '866793cb-4059-54b0-9ee7-8f539ddebede'::uuid, 'expense', 'aporte', 'alias', 'high', false),
    ('d1c10000-0000-4000-8000-000000000031'::uuid, '866793cb-4059-54b0-9ee7-8f539ddebede'::uuid, 'expense', 'aportes', 'alias', 'high', false),
    ('d1c10000-0000-4000-8000-000000000032'::uuid, '9103a0e6-366b-5c77-a31d-e3ed58991d14'::uuid, 'expense', 'tesouro direto', 'alias', 'high', false),
    ('d1c10000-0000-4000-8000-000000000033'::uuid, '9747b1c4-f9dd-5565-ad6d-0f3476ebab9e'::uuid, 'expense', 'cripto', 'alias', 'high', false),
    ('d1c10000-0000-4000-8000-000000000034'::uuid, '9747b1c4-f9dd-5565-ad6d-0f3476ebab9e'::uuid, 'expense', 'bitcoin', 'alias', 'high', false),
    ('d1c10000-0000-4000-8000-000000000035'::uuid, '9747b1c4-f9dd-5565-ad6d-0f3476ebab9e'::uuid, 'expense', 'criptomoeda', 'alias', 'high', false),
    ('d1c10000-0000-4000-8000-000000000036'::uuid, 'b1ac9b12-0b4d-5791-87d5-6628c9bbfa9a'::uuid, 'expense', 'previdencia', 'alias', 'high', false),
    ('d1c10000-0000-4000-8000-000000000037'::uuid, '1e5b4db2-b186-5524-b955-32553307d81c'::uuid, 'expense', 'fii', 'alias', 'high', false),
    ('d1c10000-0000-4000-8000-000000000038'::uuid, 'f9d9e5b6-1437-5204-bd64-2bd7d43583a8'::uuid, 'expense', 'prestacao da casa', 'alias', 'high', false),
    ('d1c10000-0000-4000-8000-000000000039'::uuid, 'f9d9e5b6-1437-5204-bd64-2bd7d43583a8'::uuid, 'expense', 'financiamento da casa', 'alias', 'high', false),
    ('d1c10000-0000-4000-8000-00000000003a'::uuid, '7319ba14-0dc7-56ff-ac5c-96024e15ec02'::uuid, 'expense', 'conta de celular', 'alias', 'high', false),
    ('d1c10000-0000-4000-8000-00000000003b'::uuid, '7319ba14-0dc7-56ff-ac5c-96024e15ec02'::uuid, 'expense', 'plano de celular', 'alias', 'high', false),
    ('d1c10000-0000-4000-8000-00000000003c'::uuid, '7319ba14-0dc7-56ff-ac5c-96024e15ec02'::uuid, 'expense', 'vivo', 'merchant', 'medium', false),
    ('d1c10000-0000-4000-8000-00000000003d'::uuid, '7319ba14-0dc7-56ff-ac5c-96024e15ec02'::uuid, 'expense', 'claro', 'merchant', 'medium', false),
    ('d1c10000-0000-4000-8000-00000000003e'::uuid, '7319ba14-0dc7-56ff-ac5c-96024e15ec02'::uuid, 'expense', 'tim', 'merchant', 'medium', false),
    ('d1c10000-0000-4000-8000-00000000003f'::uuid, '007c090e-7a6d-5645-b751-b93cabb280ed'::uuid, 'expense', 'onibus', 'alias', 'high', false),
    ('d1c10000-0000-4000-8000-000000000040'::uuid, '007c090e-7a6d-5645-b751-b93cabb280ed'::uuid, 'expense', 'metro', 'alias', 'high', false),
    ('d1c10000-0000-4000-8000-000000000041'::uuid, 'cb13d50d-43cb-553c-99cd-8851889d7f6e'::uuid, 'expense', 'etanol', 'alias', 'high', false),
    ('d1c10000-0000-4000-8000-000000000042'::uuid, 'cb13d50d-43cb-553c-99cd-8851889d7f6e'::uuid, 'expense', 'diesel', 'alias', 'high', false),
    ('d1c10000-0000-4000-8000-000000000043'::uuid, 'bf2fcca0-09c3-5dcb-a61a-87eed2860c04'::uuid, 'expense', 'mecanico', 'alias', 'high', false),
    ('d1c10000-0000-4000-8000-000000000044'::uuid, 'bf2fcca0-09c3-5dcb-a61a-87eed2860c04'::uuid, 'expense', 'oficina', 'alias', 'high', false),
    ('d1c10000-0000-4000-8000-000000000045'::uuid, 'af5619e0-3683-5b8c-b9fc-0b3ddfbd2075'::uuid, 'expense', 'consulta medica', 'alias', 'high', false),
    ('d1c10000-0000-4000-8000-000000000046'::uuid, 'af5619e0-3683-5b8c-b9fc-0b3ddfbd2075'::uuid, 'expense', 'exame', 'alias', 'high', false),
    ('d1c10000-0000-4000-8000-000000000047'::uuid, '01b51d39-347e-560c-ac07-d0a700f0c24f'::uuid, 'expense', 'udemy', 'merchant', 'medium', false),
    ('d1c10000-0000-4000-8000-000000000048'::uuid, '01b51d39-347e-560c-ac07-d0a700f0c24f'::uuid, 'expense', 'alura', 'merchant', 'medium', false),
    ('d1c10000-0000-4000-8000-000000000049'::uuid, '654552ab-829d-5b4d-b0ec-4cb1463454d7'::uuid, 'expense', 'certificacao', 'alias', 'high', false),
    ('d1c10000-0000-4000-8000-00000000004a'::uuid, 'a371851d-56cb-551d-addb-022575b8d6e9'::uuid, 'expense', 'lanche', 'alias', 'high', false),
    ('d1c10000-0000-4000-8000-00000000004b'::uuid, 'a20b4072-23b7-53e8-8d03-8146e0473218'::uuid, 'expense', 'cafeteria', 'alias', 'high', false),
    ('d1c10000-0000-4000-8000-00000000004c'::uuid, 'a20b4072-23b7-53e8-8d03-8146e0473218'::uuid, 'expense', 'starbucks', 'merchant', 'medium', false),
    ('d1c10000-0000-4000-8000-00000000004d'::uuid, '514c00a0-ca41-5798-85d0-39992fbc223c'::uuid, 'expense', 'jogos', 'alias', 'high', false),
    ('d1c10000-0000-4000-8000-00000000004e'::uuid, '514c00a0-ca41-5798-85d0-39992fbc223c'::uuid, 'expense', 'steam', 'merchant', 'medium', false),
    ('d1c10000-0000-4000-8000-00000000004f'::uuid, '514c00a0-ca41-5798-85d0-39992fbc223c'::uuid, 'expense', 'playstation plus', 'alias', 'high', false),
    ('d1c10000-0000-4000-8000-000000000050'::uuid, '14416063-f271-53e2-8a58-6682461ec532'::uuid, 'expense', 'calcado', 'alias', 'high', false),
    ('d1c10000-0000-4000-8000-000000000051'::uuid, '14416063-f271-53e2-8a58-6682461ec532'::uuid, 'expense', 'sapato', 'alias', 'high', false),
    ('d1c10000-0000-4000-8000-000000000052'::uuid, 'f9656739-8d1c-5675-8eaf-63a057137307'::uuid, 'expense', 'manicure', 'alias', 'high', false),
    ('d1c10000-0000-4000-8000-000000000053'::uuid, 'e917b351-60d6-53c5-ab5b-a92e663d700b'::uuid, 'expense', 'etf', 'alias', 'high', false),
    ('d1c10000-0000-4000-8000-000000000054'::uuid, 'ee26c4d9-ca74-5537-80b9-4d90815b9c06'::uuid, 'expense', 'cdb', 'alias', 'high', false),
    ('d1c10000-0000-4000-8000-000000000055'::uuid, '866793cb-4059-54b0-9ee7-8f539ddebede'::uuid, 'expense', 'corretora', 'alias', 'high', false)
) AS v(id, category_id, kind, term, signal_type, confidence, is_ambiguous)
WHERE NOT EXISTS (
    SELECT 1
    FROM mecontrola.category_dictionary d
    WHERE d.kind = v.kind
      AND d.term_normalized = lower(mecontrola.immutable_unaccent(v.term))
      AND d.deprecated_at IS NULL
);

UPDATE mecontrola.category_editorial_version SET version = version + 1, updated_at = now();

SET LOCAL lock_timeout    = '5s';
SET LOCAL statement_timeout = '120s';

CREATE INDEX IF NOT EXISTS onboarding_tokens_mobile_activable_idx
    ON mecontrola.onboarding_tokens (customer_mobile_e164, paid_at)
    WHERE status = 'PAID';

ALTER TABLE mecontrola.onboarding_tokens
    ADD COLUMN IF NOT EXISTS email_sent_at         TIMESTAMPTZ NULL,
    ADD COLUMN IF NOT EXISTS page_opened_at        TIMESTAMPTZ NULL,
    ADD COLUMN IF NOT EXISTS activation_started_at TIMESTAMPTZ NULL,
    ADD COLUMN IF NOT EXISTS whatsapp_opened_at    TIMESTAMPTZ NULL;

CREATE TABLE IF NOT EXISTS mecontrola.onboarding_activation_nomatch_throttle (
    mobile_e164  TEXT        NOT NULL,
    window_start TIMESTAMPTZ NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT onboarding_activation_nomatch_throttle_pkey PRIMARY KEY (mobile_e164, window_start)
);

CREATE TABLE IF NOT EXISTS mecontrola.onboarding_welcome_processed (
    event_id     TEXT        NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT onboarding_welcome_processed_pkey PRIMARY KEY (event_id),
    CONSTRAINT onboarding_welcome_processed_event_id_nonempty_check CHECK (length(event_id) > 0)
);

SET LOCAL lock_timeout    = '5s';
SET LOCAL statement_timeout = '120s';

CREATE TABLE IF NOT EXISTS mecontrola.agents_write_ledger (
    id            UUID        NOT NULL DEFAULT gen_random_uuid(),
    user_id       UUID        NOT NULL,
    wamid         TEXT        NOT NULL,
    item_seq      INT         NOT NULL,
    operation     TEXT        NOT NULL,
    resource_id   UUID        NOT NULL,
    resource_kind TEXT        NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT agents_write_ledger_pkey PRIMARY KEY (id),
    CONSTRAINT agents_write_ledger_wamid_nonempty_check CHECK (length(wamid) > 0),
    CONSTRAINT agents_write_ledger_operation_nonempty_check CHECK (length(operation) > 0),
    CONSTRAINT agents_write_ledger_item_seq_positive_check CHECK (item_seq >= 0),
    CONSTRAINT agents_write_ledger_uniq UNIQUE (wamid, item_seq, operation)
);

CREATE INDEX IF NOT EXISTS agents_write_ledger_user_created_idx
    ON mecontrola.agents_write_ledger (user_id, created_at);

SET LOCAL lock_timeout    = '5s';
SET LOCAL statement_timeout = '120s';

ALTER TABLE mecontrola.transactions
    ADD COLUMN IF NOT EXISTS origin_wamid     TEXT NULL,
    ADD COLUMN IF NOT EXISTS origin_item_seq  INT  NULL,
    ADD COLUMN IF NOT EXISTS origin_operation TEXT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS transactions_origin_uk
    ON mecontrola.transactions (origin_wamid, origin_item_seq, origin_operation)
    WHERE origin_wamid IS NOT NULL;

ALTER TABLE mecontrola.transactions_card_purchases
    ADD COLUMN IF NOT EXISTS origin_wamid     TEXT NULL,
    ADD COLUMN IF NOT EXISTS origin_item_seq  INT  NULL,
    ADD COLUMN IF NOT EXISTS origin_operation TEXT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS transactions_card_purchases_origin_uk
    ON mecontrola.transactions_card_purchases (origin_wamid, origin_item_seq, origin_operation)
    WHERE origin_wamid IS NOT NULL;

SET LOCAL lock_timeout    = '5s';
SET LOCAL statement_timeout = '120s';

CREATE EXTENSION IF NOT EXISTS vector WITH SCHEMA mecontrola;

CREATE TABLE IF NOT EXISTS mecontrola.platform_threads (
    id          UUID        NOT NULL,
    resource_id TEXT        NOT NULL,
    thread_id   TEXT        NOT NULL,
    title       TEXT        NOT NULL DEFAULT '',
    metadata    JSONB       NOT NULL DEFAULT '{}'::jsonb,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT platform_threads_pkey PRIMARY KEY (id),
    CONSTRAINT platform_threads_resource_thread_uniq UNIQUE (resource_id, thread_id),
    CONSTRAINT platform_threads_resource_len_chk CHECK (char_length(resource_id) BETWEEN 1 AND 256),
    CONSTRAINT platform_threads_thread_len_chk CHECK (char_length(thread_id) BETWEEN 1 AND 256)
);

CREATE INDEX IF NOT EXISTS platform_threads_resource_id_idx
    ON mecontrola.platform_threads (resource_id);

CREATE TABLE IF NOT EXISTS mecontrola.platform_resources (
    resource_id    TEXT        NOT NULL,
    working_memory TEXT        NOT NULL DEFAULT '',
    metadata       JSONB       NOT NULL DEFAULT '{}'::jsonb,
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT platform_resources_pkey PRIMARY KEY (resource_id),
    CONSTRAINT platform_resources_resource_len_chk CHECK (char_length(resource_id) BETWEEN 1 AND 256)
);

CREATE TABLE IF NOT EXISTS mecontrola.platform_messages (
    id                 UUID        NOT NULL,
    platform_thread_id UUID        NOT NULL,
    resource_id        TEXT        NOT NULL,
    role               TEXT        NOT NULL,
    content            TEXT        NOT NULL DEFAULT '',
    parts              JSONB       NOT NULL DEFAULT '[]'::jsonb,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT platform_messages_pkey PRIMARY KEY (id),
    CONSTRAINT platform_messages_platform_thread_id_fkey
        FOREIGN KEY (platform_thread_id) REFERENCES mecontrola.platform_threads(id) ON DELETE CASCADE,
    CONSTRAINT platform_messages_resource_len_chk CHECK (char_length(resource_id) BETWEEN 1 AND 256),
    CONSTRAINT platform_messages_role_chk CHECK (role = ANY (ARRAY['user'::text, 'assistant'::text, 'tool'::text, 'system'::text]))
);

CREATE INDEX IF NOT EXISTS platform_messages_platform_thread_id_created_idx
    ON mecontrola.platform_messages (platform_thread_id, created_at);

CREATE TABLE IF NOT EXISTS mecontrola.platform_runs (
    id                 UUID        NOT NULL,
    platform_thread_id UUID        NOT NULL,
    resource_id        TEXT        NOT NULL,
    thread_id          TEXT        NOT NULL,
    agent_id           TEXT        NOT NULL DEFAULT '',
    workflow           TEXT        NOT NULL DEFAULT '',
    correlation_key    TEXT        NOT NULL DEFAULT '',
    status             TEXT        NOT NULL,
    outcome            TEXT        NOT NULL DEFAULT '',
    error              TEXT        NOT NULL DEFAULT '',
    started_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    ended_at           TIMESTAMPTZ,
    duration_ms        BIGINT      NOT NULL DEFAULT 0,
    CONSTRAINT platform_runs_pkey PRIMARY KEY (id),
    CONSTRAINT platform_runs_platform_thread_id_fkey
        FOREIGN KEY (platform_thread_id) REFERENCES mecontrola.platform_threads(id) ON DELETE CASCADE,
    CONSTRAINT platform_runs_duration_chk CHECK (duration_ms >= 0),
    CONSTRAINT platform_runs_resource_len_chk CHECK (char_length(resource_id) BETWEEN 1 AND 256),
    CONSTRAINT platform_runs_status_chk CHECK (status = ANY (ARRAY['running'::text, 'succeeded'::text, 'failed'::text])),
    CONSTRAINT platform_runs_thread_len_chk CHECK (char_length(thread_id) BETWEEN 1 AND 256)
);

CREATE INDEX IF NOT EXISTS platform_runs_resource_started_idx
    ON mecontrola.platform_runs (resource_id, started_at DESC);

CREATE INDEX IF NOT EXISTS platform_runs_platform_thread_id_started_idx
    ON mecontrola.platform_runs (platform_thread_id, started_at DESC);

CREATE TABLE IF NOT EXISTS mecontrola.platform_embeddings (
    id                UUID                     NOT NULL,
    resource_id       TEXT                     NOT NULL,
    thread_id         TEXT                     NOT NULL,
    source_message_id UUID,
    content           TEXT                     NOT NULL,
    embedding         mecontrola.vector(1536)  NOT NULL,
    model             TEXT                     NOT NULL DEFAULT '',
    created_at        TIMESTAMPTZ              NOT NULL DEFAULT now(),
    CONSTRAINT platform_embeddings_pkey PRIMARY KEY (id),
    CONSTRAINT platform_embeddings_resource_len_chk CHECK (char_length(resource_id) BETWEEN 1 AND 256),
    CONSTRAINT platform_embeddings_thread_len_chk CHECK (char_length(thread_id) BETWEEN 1 AND 256)
);

CREATE INDEX IF NOT EXISTS platform_embeddings_hnsw_idx
    ON mecontrola.platform_embeddings USING hnsw (embedding mecontrola.vector_cosine_ops);

CREATE INDEX IF NOT EXISTS platform_embeddings_resource_idx
    ON mecontrola.platform_embeddings (resource_id);

CREATE UNIQUE INDEX IF NOT EXISTS platform_embeddings_source_model_uniq
    ON mecontrola.platform_embeddings (source_message_id, model)
    WHERE source_message_id IS NOT NULL;

CREATE TABLE IF NOT EXISTS mecontrola.platform_scorer_results (
    id         UUID             NOT NULL,
    run_id     UUID             NOT NULL,
    scorer_id  TEXT             NOT NULL,
    kind       TEXT             NOT NULL,
    score      DOUBLE PRECISION NOT NULL DEFAULT 0,
    reason     TEXT             NOT NULL DEFAULT '',
    metadata   JSONB            NOT NULL DEFAULT '{}'::jsonb,
    sampled    BOOLEAN          NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ      NOT NULL DEFAULT now(),
    CONSTRAINT platform_scorer_results_pkey PRIMARY KEY (id),
    CONSTRAINT platform_scorer_results_run_fkey
        FOREIGN KEY (run_id) REFERENCES mecontrola.platform_runs(id) ON DELETE CASCADE,
    CONSTRAINT platform_scorer_results_kind_chk CHECK (kind = ANY (ARRAY['code_based'::text, 'llm_judged'::text])),
    CONSTRAINT platform_scorer_results_score_chk CHECK (score >= 0 AND score <= 1)
);

CREATE INDEX IF NOT EXISTS platform_scorer_results_run_idx
    ON mecontrola.platform_scorer_results (run_id);
