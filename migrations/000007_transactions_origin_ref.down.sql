SET LOCAL lock_timeout    = '5s';
SET LOCAL statement_timeout = '120s';

DROP INDEX IF EXISTS mecontrola.transactions_card_purchases_origin_uk;

ALTER TABLE mecontrola.transactions_card_purchases
    DROP COLUMN IF EXISTS origin_operation,
    DROP COLUMN IF EXISTS origin_item_seq,
    DROP COLUMN IF EXISTS origin_wamid;

DROP INDEX IF EXISTS mecontrola.transactions_origin_uk;

ALTER TABLE mecontrola.transactions
    DROP COLUMN IF EXISTS origin_operation,
    DROP COLUMN IF EXISTS origin_item_seq,
    DROP COLUMN IF EXISTS origin_wamid;
