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
