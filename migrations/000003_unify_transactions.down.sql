SET LOCAL lock_timeout      = '5s';
SET LOCAL statement_timeout = '120s';

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

ALTER TABLE mecontrola.transactions_recurring_materializations
    ADD COLUMN IF NOT EXISTS materialized_purchase_id UUID NULL;

ALTER TABLE mecontrola.transactions_card_invoice_items
    DROP CONSTRAINT IF EXISTS transactions_card_invoice_items_transaction_fk;
ALTER TABLE mecontrola.transactions_card_invoice_items
    DROP CONSTRAINT IF EXISTS transactions_card_invoice_items_transaction_uk;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.columns
                WHERE table_schema = 'mecontrola'
                  AND table_name = 'transactions_card_invoice_items'
                  AND column_name = 'transaction_id')
       AND NOT EXISTS (SELECT 1 FROM information_schema.columns
                WHERE table_schema = 'mecontrola'
                  AND table_name = 'transactions_card_invoice_items'
                  AND column_name = 'purchase_id') THEN
        ALTER TABLE mecontrola.transactions_card_invoice_items
            RENAME COLUMN transaction_id TO purchase_id;
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'transactions_card_invoice_items_purchase_uk') THEN
        ALTER TABLE mecontrola.transactions_card_invoice_items
            ADD CONSTRAINT transactions_card_invoice_items_purchase_uk
                UNIQUE (purchase_id, installment_index);
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'transactions_card_invoice_items_purchase_id_fkey') THEN
        ALTER TABLE mecontrola.transactions_card_invoice_items
            ADD CONSTRAINT transactions_card_invoice_items_purchase_id_fkey
                FOREIGN KEY (purchase_id) REFERENCES mecontrola.transactions_card_purchases(id);
    END IF;
END $$;

ALTER TABLE mecontrola.transactions DROP CONSTRAINT IF EXISTS transactions_card_fk;
ALTER TABLE mecontrola.transactions DROP CONSTRAINT IF EXISTS transactions_card_due_day_chk;
ALTER TABLE mecontrola.transactions DROP CONSTRAINT IF EXISTS transactions_card_closing_day_chk;
ALTER TABLE mecontrola.transactions DROP CONSTRAINT IF EXISTS transactions_installments_range_chk;
ALTER TABLE mecontrola.transactions DROP CONSTRAINT IF EXISTS transactions_card_completeness_chk;

ALTER TABLE mecontrola.transactions
    DROP COLUMN IF EXISTS card_due_day,
    DROP COLUMN IF EXISTS card_closing_day,
    DROP COLUMN IF EXISTS installments_total,
    DROP COLUMN IF EXISTS card_id;
