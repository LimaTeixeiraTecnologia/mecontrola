SET LOCAL lock_timeout    = '5s';
SET LOCAL statement_timeout = '120s';

CREATE TABLE IF NOT EXISTS mecontrola.transactions (
    id                          UUID        NOT NULL,
    user_id                     UUID        NOT NULL,
    direction                   SMALLINT    NOT NULL,
    payment_method              SMALLINT    NOT NULL,
    amount_cents                BIGINT      NOT NULL CHECK (amount_cents > 0),
    description                 TEXT        NOT NULL,
    category_id                 UUID        NOT NULL,
    subcategory_id              UUID        NULL,
    category_name_snapshot      TEXT        NOT NULL,
    subcategory_name_snapshot   TEXT        NULL,
    ref_month                   CHAR(7)     NOT NULL,
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
    direction                   SMALLINT    NOT NULL CHECK (direction = 2),
    total_amount_cents          BIGINT      NOT NULL CHECK (total_amount_cents > 0),
    installments_total          SMALLINT    NOT NULL CHECK (installments_total BETWEEN 1 AND 24),
    description                 TEXT        NOT NULL,
    category_id                 UUID        NOT NULL,
    subcategory_id              UUID        NULL,
    category_name_snapshot      TEXT        NOT NULL,
    subcategory_name_snapshot   TEXT        NULL,
    purchased_at                TIMESTAMPTZ NOT NULL,
    card_closing_day            SMALLINT    NOT NULL CHECK (card_closing_day BETWEEN 1 AND 31),
    card_due_day                SMALLINT    NOT NULL CHECK (card_due_day BETWEEN 1 AND 31),
    version                     BIGINT      NOT NULL DEFAULT 1,
    deleted_at                  TIMESTAMPTZ NULL,
    created_at                  TIMESTAMPTZ NOT NULL,
    updated_at                  TIMESTAMPTZ NOT NULL,
    CONSTRAINT transactions_card_purchases_pkey PRIMARY KEY (id)
);

CREATE INDEX IF NOT EXISTS transactions_card_purchases_user_card_idx
    ON mecontrola.transactions_card_purchases (user_id, card_id, created_at DESC, id DESC)
    WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS mecontrola.transactions_card_invoices (
    id                  UUID        NOT NULL,
    user_id             UUID        NOT NULL,
    card_id             UUID        NOT NULL,
    ref_month           CHAR(7)     NOT NULL,
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
    ref_month           CHAR(7)     NOT NULL,
    installment_index   SMALLINT    NOT NULL,
    amount_cents        BIGINT      NOT NULL CHECK (amount_cents > 0),
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
    amount_cents                BIGINT      NOT NULL CHECK (amount_cents > 0),
    description                 TEXT        NOT NULL,
    category_id                 UUID        NOT NULL,
    subcategory_id              UUID        NULL,
    category_name_snapshot      TEXT        NOT NULL,
    subcategory_name_snapshot   TEXT        NULL,
    frequency                   SMALLINT    NOT NULL,
    day_of_month                SMALLINT    NOT NULL CHECK (day_of_month BETWEEN 1 AND 28),
    installments_total          SMALLINT    NOT NULL DEFAULT 1 CHECK (installments_total BETWEEN 1 AND 24),
    started_at                  TIMESTAMPTZ NOT NULL,
    ended_at                    TIMESTAMPTZ NULL,
    version                     BIGINT      NOT NULL DEFAULT 1,
    deleted_at                  TIMESTAMPTZ NULL,
    created_at                  TIMESTAMPTZ NOT NULL,
    updated_at                  TIMESTAMPTZ NOT NULL,
    CONSTRAINT transactions_recurring_templates_pkey PRIMARY KEY (id),
    CONSTRAINT transactions_recurring_templates_credit_chk
        CHECK ((payment_method <> 7) OR (card_id IS NOT NULL))
);

CREATE INDEX IF NOT EXISTS transactions_recurring_templates_user_day_idx
    ON mecontrola.transactions_recurring_templates (user_id, day_of_month)
    WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS mecontrola.transactions_recurring_materializations (
    template_id                 UUID        NOT NULL REFERENCES mecontrola.transactions_recurring_templates(id),
    ref_month                   CHAR(7)     NOT NULL,
    materialized_transaction_id UUID        NULL,
    materialized_purchase_id    UUID        NULL,
    materialized_at             TIMESTAMPTZ NOT NULL,
    CONSTRAINT transactions_recurring_materializations_pkey PRIMARY KEY (template_id, ref_month)
);

CREATE TABLE IF NOT EXISTS mecontrola.transactions_monthly_summary (
    user_id        UUID        NOT NULL,
    ref_month      CHAR(7)     NOT NULL,
    income_cents   BIGINT      NOT NULL DEFAULT 0,
    outcome_cents  BIGINT      NOT NULL DEFAULT 0,
    total_cents    BIGINT      NOT NULL DEFAULT 0,
    version        BIGINT      NOT NULL DEFAULT 1,
    updated_at     TIMESTAMPTZ NOT NULL,
    CONSTRAINT transactions_monthly_summary_pkey PRIMARY KEY (user_id, ref_month)
);
