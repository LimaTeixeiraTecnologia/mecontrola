SET LOCAL lock_timeout    = '5s';
SET LOCAL statement_timeout = '120s';

ALTER TABLE mecontrola.cards DROP CONSTRAINT cards_bank_len_chk;
ALTER TABLE mecontrola.cards DROP COLUMN bank;
ALTER TABLE mecontrola.cards ADD COLUMN name TEXT NOT NULL DEFAULT '';
ALTER TABLE mecontrola.cards ALTER COLUMN name DROP DEFAULT;
ALTER TABLE mecontrola.cards ADD CONSTRAINT cards_name_len_chk CHECK (char_length(name) BETWEEN 1 AND 64);
ALTER TABLE mecontrola.cards ADD COLUMN limit_cents BIGINT NOT NULL DEFAULT 0;
ALTER TABLE mecontrola.cards ADD CONSTRAINT cards_limit_cents_chk CHECK (limit_cents >= 0 AND limit_cents <= 100000000);

CREATE INDEX IF NOT EXISTS cards_user_limit_positive_idx
    ON mecontrola.cards (user_id)
    WHERE limit_cents > 0 AND deleted_at IS NULL;

DROP TABLE IF EXISTS mecontrola.banks CASCADE;
