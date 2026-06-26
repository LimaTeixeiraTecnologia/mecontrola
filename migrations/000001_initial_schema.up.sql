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

CREATE TABLE IF NOT EXISTS mecontrola.cards (
    id          UUID        NOT NULL,
    user_id     UUID        NOT NULL,
    name        TEXT        NOT NULL,
    nickname    TEXT        NOT NULL,
    closing_day SMALLINT    NOT NULL,
    due_day     SMALLINT    NOT NULL,
    limit_cents BIGINT      NOT NULL DEFAULT 0,
    version     BIGINT      NOT NULL DEFAULT 1,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at  TIMESTAMPTZ NULL,
    CONSTRAINT cards_pkey             PRIMARY KEY (id),
    CONSTRAINT cards_user_fk          FOREIGN KEY (user_id)
        REFERENCES mecontrola.users(id) ON DELETE RESTRICT,
    CONSTRAINT cards_closing_day_chk  CHECK (closing_day BETWEEN 1 AND 31),
    CONSTRAINT cards_due_day_chk      CHECK (due_day     BETWEEN 1 AND 31),
    CONSTRAINT cards_name_len_chk     CHECK (char_length(name)     BETWEEN 1 AND 64),
    CONSTRAINT cards_nickname_len_chk CHECK (char_length(nickname) BETWEEN 1 AND 32),
    CONSTRAINT cards_limit_cents_chk  CHECK (limit_cents >= 0 AND limit_cents <= 100000000)
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
-- Cards: limit index (consolidated from 000005)
-- ============================================================

CREATE INDEX IF NOT EXISTS cards_user_limit_positive_idx
    ON mecontrola.cards (user_id)
    WHERE limit_cents > 0 AND deleted_at IS NULL;


-- ============================================================
-- Onboarding: Sessions (consolidated from 000004)
-- ============================================================

CREATE TABLE IF NOT EXISTS mecontrola.onboarding_sessions (
    user_id    UUID        NOT NULL,
    channel    TEXT        NOT NULL,
    state      TEXT        NOT NULL,
    payload    JSONB       NOT NULL DEFAULT '{}'::jsonb,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT onboarding_sessions_pkey                  PRIMARY KEY (user_id),
    CONSTRAINT onboarding_sessions_user_id_fk
        FOREIGN KEY (user_id) REFERENCES mecontrola.users (id) ON DELETE CASCADE,
    CONSTRAINT onboarding_sessions_channel_chk
        CHECK (channel IN ('whatsapp')),
    CONSTRAINT onboarding_sessions_state_nonempty_chk
        CHECK (length(state) > 0)
);

CREATE INDEX IF NOT EXISTS onboarding_sessions_channel_state_idx
    ON mecontrola.onboarding_sessions (channel, state);

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

CREATE TABLE IF NOT EXISTS mecontrola.agent_sessions (
    id             UUID        NOT NULL,
    user_id        UUID        NOT NULL,
    channel        TEXT        NOT NULL,
    pending_action JSONB       NOT NULL DEFAULT '{}'::jsonb,
    recent_turns   JSONB       NOT NULL DEFAULT '[]'::jsonb,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at     TIMESTAMPTZ NOT NULL,
    CONSTRAINT agent_sessions_pkey PRIMARY KEY (id),
    CONSTRAINT agent_sessions_user_id_fkey
        FOREIGN KEY (user_id) REFERENCES mecontrola.users (id) ON DELETE CASCADE,
    CONSTRAINT agent_sessions_channel_len_chk
        CHECK (char_length(channel) BETWEEN 1 AND 32),
    CONSTRAINT agent_sessions_pending_action_size_chk
        CHECK (octet_length(pending_action::text) <= 16384),
    CONSTRAINT agent_sessions_recent_turns_size_chk
        CHECK (octet_length(recent_turns::text) <= 65536)
);

CREATE UNIQUE INDEX IF NOT EXISTS agent_sessions_user_channel_uniq_idx
    ON mecontrola.agent_sessions (user_id, channel);

CREATE INDEX IF NOT EXISTS agent_sessions_expires_idx
    ON mecontrola.agent_sessions (expires_at);

SET LOCAL lock_timeout    = '5s';
SET LOCAL statement_timeout = '120s';

CREATE TABLE IF NOT EXISTS mecontrola.agent_decisions (
    id                 UUID        NOT NULL,
    user_id            UUID        NOT NULL,
    channel            TEXT        NOT NULL,
    message_id         TEXT        NOT NULL,
    intent_kind        TEXT        NOT NULL,
    prompt_sha256      TEXT        NOT NULL,
    llm_model          TEXT        NOT NULL,
    redacted_response  JSONB       NOT NULL DEFAULT '{}'::jsonb,
    trace_id           TEXT        NOT NULL DEFAULT '',
    decided_action     TEXT        NOT NULL,
    resulting_event_id UUID,
    status             TEXT        NOT NULL,
    step_index         INT         NOT NULL DEFAULT 0,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    settled_at         TIMESTAMPTZ,
    CONSTRAINT agent_decisions_pkey PRIMARY KEY (id),
    CONSTRAINT agent_decisions_user_id_fkey
        FOREIGN KEY (user_id) REFERENCES mecontrola.users (id) ON DELETE CASCADE,
    CONSTRAINT agent_decisions_channel_len_chk
        CHECK (char_length(channel) BETWEEN 1 AND 32),
    CONSTRAINT agent_decisions_message_id_len_chk
        CHECK (char_length(message_id) BETWEEN 1 AND 256),
    CONSTRAINT agent_decisions_intent_kind_len_chk
        CHECK (char_length(intent_kind) BETWEEN 1 AND 64),
    CONSTRAINT agent_decisions_prompt_sha256_len_chk
        CHECK (char_length(prompt_sha256) = 64),
    CONSTRAINT agent_decisions_llm_model_len_chk
        CHECK (char_length(llm_model) BETWEEN 1 AND 128),
    CONSTRAINT agent_decisions_decided_action_len_chk
        CHECK (char_length(decided_action) BETWEEN 1 AND 64),
    CONSTRAINT agent_decisions_redacted_response_size_chk
        CHECK (octet_length(redacted_response::text) <= 65536),
    CONSTRAINT agent_decisions_status_chk
        CHECK (status IN ('pending', 'executed', 'rejected', 'awaiting_confirmation'))
);

CREATE UNIQUE INDEX IF NOT EXISTS agent_decisions_user_channel_message_step_idx
    ON mecontrola.agent_decisions (user_id, channel, message_id, step_index);

CREATE INDEX IF NOT EXISTS agent_decisions_user_created_idx
    ON mecontrola.agent_decisions (user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS agent_decisions_status_idx
    ON mecontrola.agent_decisions (status);

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

CREATE TABLE mecontrola.agent_working_memory (
    user_id    UUID        NOT NULL,
    content    TEXT        NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT agent_working_memory_pk PRIMARY KEY (user_id)
);

CREATE EXTENSION IF NOT EXISTS pgcrypto WITH SCHEMA mecontrola;

CREATE TABLE mecontrola.agent_observations (
    id         UUID        NOT NULL DEFAULT gen_random_uuid(),
    user_id    UUID        NOT NULL,
    channel    TEXT        NOT NULL,
    content    TEXT        NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL DEFAULT now() + interval '90 days',
    CONSTRAINT agent_observations_pk PRIMARY KEY (id)
);

CREATE INDEX agent_observations_user_channel_idx ON mecontrola.agent_observations (user_id, channel, created_at DESC);
CREATE INDEX agent_observations_expires_at_idx ON mecontrola.agent_observations (expires_at);

CREATE EXTENSION IF NOT EXISTS pg_trgm WITH SCHEMA mecontrola;

CREATE INDEX IF NOT EXISTS dictionary_term_trgm_idx
    ON mecontrola.category_dictionary
    USING gin (term_normalized gin_trgm_ops)
    WHERE deprecated_at IS NULL;

SET LOCAL lock_timeout    = '5s';
SET LOCAL statement_timeout = '120s';

CREATE TABLE IF NOT EXISTS mecontrola.agent_threads (
    id         UUID        NOT NULL,
    user_id    UUID        NOT NULL,
    channel    TEXT        NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT agent_threads_pkey PRIMARY KEY (id),
    CONSTRAINT agent_threads_user_id_fkey
        FOREIGN KEY (user_id) REFERENCES mecontrola.users (id) ON DELETE CASCADE,
    CONSTRAINT agent_threads_channel_len_chk
        CHECK (char_length(channel) BETWEEN 1 AND 32),
    CONSTRAINT agent_threads_user_channel_uniq
        UNIQUE (user_id, channel)
);

SET LOCAL lock_timeout    = '5s';
SET LOCAL statement_timeout = '120s';

CREATE TABLE IF NOT EXISTS mecontrola.agent_runs (
    id          UUID        NOT NULL,
    thread_id   UUID        NOT NULL,
    user_id     UUID        NOT NULL,
    channel     TEXT        NOT NULL,
    message_id  TEXT        NOT NULL DEFAULT '',
    agent_id    TEXT        NOT NULL DEFAULT '',
    workflow    TEXT        NOT NULL DEFAULT '',
    tool_name   TEXT        NOT NULL DEFAULT '',
    intent_kind TEXT        NOT NULL DEFAULT '',
    outcome     TEXT        NOT NULL DEFAULT '',
    status      TEXT        NOT NULL,
    error       TEXT        NOT NULL DEFAULT '',
    decision_id UUID,
    started_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    ended_at    TIMESTAMPTZ,
    duration_ms BIGINT      NOT NULL DEFAULT 0,
    CONSTRAINT agent_runs_pkey PRIMARY KEY (id),
    CONSTRAINT agent_runs_thread_id_fkey
        FOREIGN KEY (thread_id) REFERENCES mecontrola.agent_threads (id) ON DELETE CASCADE,
    CONSTRAINT agent_runs_decision_id_fkey
        FOREIGN KEY (decision_id) REFERENCES mecontrola.agent_decisions (id) ON DELETE SET NULL,
    CONSTRAINT agent_runs_channel_len_chk
        CHECK (char_length(channel) BETWEEN 1 AND 32),
    CONSTRAINT agent_runs_status_chk
        CHECK (status IN ('running', 'succeeded', 'failed'))
);

CREATE INDEX IF NOT EXISTS agent_runs_thread_started_idx
    ON mecontrola.agent_runs (thread_id, started_at DESC);

CREATE INDEX IF NOT EXISTS agent_runs_user_started_idx
    ON mecontrola.agent_runs (user_id, started_at DESC);

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

SET LOCAL lock_timeout    = '5s';
SET LOCAL statement_timeout = '120s';

CREATE TABLE IF NOT EXISTS mecontrola.agent_processed_events (
    event_id          UUID        NOT NULL,
    event_type        TEXT        NOT NULL,
    aggregate_user_id UUID        NULL,
    processed_at      TIMESTAMPTZ NOT NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT agent_processed_events_pkey PRIMARY KEY (event_id)
);

CREATE INDEX IF NOT EXISTS agent_processed_events_aggregate_user_id_idx
    ON mecontrola.agent_processed_events (aggregate_user_id);
