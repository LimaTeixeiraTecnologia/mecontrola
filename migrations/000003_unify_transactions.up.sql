SET LOCAL lock_timeout      = '5s';
SET LOCAL statement_timeout = '120s';

ALTER TABLE mecontrola.transactions
    ADD COLUMN IF NOT EXISTS card_id            UUID     NULL,
    ADD COLUMN IF NOT EXISTS installments_total SMALLINT NULL,
    ADD COLUMN IF NOT EXISTS card_closing_day   SMALLINT NULL,
    ADD COLUMN IF NOT EXISTS card_due_day       SMALLINT NULL;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'transactions_card_completeness_chk') THEN
        ALTER TABLE mecontrola.transactions
            ADD CONSTRAINT transactions_card_completeness_chk CHECK (
                (payment_method = 7
                    AND card_id IS NOT NULL
                    AND installments_total IS NOT NULL
                    AND card_closing_day IS NOT NULL
                    AND card_due_day IS NOT NULL)
                OR
                (payment_method <> 7
                    AND card_id IS NULL
                    AND installments_total IS NULL
                    AND card_closing_day IS NULL
                    AND card_due_day IS NULL)
            );
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'transactions_installments_range_chk') THEN
        ALTER TABLE mecontrola.transactions
            ADD CONSTRAINT transactions_installments_range_chk CHECK (
                installments_total IS NULL OR installments_total BETWEEN 1 AND 24
            );
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'transactions_card_closing_day_chk') THEN
        ALTER TABLE mecontrola.transactions
            ADD CONSTRAINT transactions_card_closing_day_chk CHECK (
                card_closing_day IS NULL OR card_closing_day BETWEEN 1 AND 31
            );
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'transactions_card_due_day_chk') THEN
        ALTER TABLE mecontrola.transactions
            ADD CONSTRAINT transactions_card_due_day_chk CHECK (
                card_due_day IS NULL OR card_due_day BETWEEN 1 AND 31
            );
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'transactions_card_fk') THEN
        ALTER TABLE mecontrola.transactions
            ADD CONSTRAINT transactions_card_fk
                FOREIGN KEY (card_id) REFERENCES mecontrola.cards(id) ON DELETE RESTRICT;
    END IF;
END $$;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.columns
                WHERE table_schema = 'mecontrola'
                  AND table_name = 'transactions_card_invoice_items'
                  AND column_name = 'purchase_id')
       AND NOT EXISTS (SELECT 1 FROM information_schema.columns
                WHERE table_schema = 'mecontrola'
                  AND table_name = 'transactions_card_invoice_items'
                  AND column_name = 'transaction_id') THEN
        ALTER TABLE mecontrola.transactions_card_invoice_items
            RENAME COLUMN purchase_id TO transaction_id;
    END IF;
END $$;

ALTER TABLE mecontrola.transactions_card_invoice_items
    DROP CONSTRAINT IF EXISTS transactions_card_invoice_items_purchase_id_fkey;
ALTER TABLE mecontrola.transactions_card_invoice_items
    DROP CONSTRAINT IF EXISTS transactions_card_invoice_items_purchase_uk;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'transactions_card_invoice_items_transaction_uk') THEN
        ALTER TABLE mecontrola.transactions_card_invoice_items
            ADD CONSTRAINT transactions_card_invoice_items_transaction_uk
                UNIQUE (transaction_id, installment_index);
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'transactions_card_invoice_items_transaction_fk') THEN
        ALTER TABLE mecontrola.transactions_card_invoice_items
            ADD CONSTRAINT transactions_card_invoice_items_transaction_fk
                FOREIGN KEY (transaction_id) REFERENCES mecontrola.transactions(id);
    END IF;
END $$;

ALTER TABLE mecontrola.transactions_recurring_materializations
    DROP COLUMN IF EXISTS materialized_purchase_id;

DROP TABLE IF EXISTS mecontrola.transactions_card_purchases;
