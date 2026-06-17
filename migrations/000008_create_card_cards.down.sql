SET LOCAL lock_timeout    = '5s';
SET LOCAL statement_timeout = '120s';

DROP INDEX IF EXISTS mecontrola.cards_user_pagination_idx;
DROP INDEX IF EXISTS mecontrola.cards_user_nickname_active_uniq_idx;

ALTER TABLE mecontrola.cards
    RENAME CONSTRAINT cards_pkey TO cards_archived_20260609120000_pkey;

ALTER TABLE mecontrola.cards
    RENAME CONSTRAINT cards_user_fk TO cards_archived_20260609120000_user_fk;

ALTER TABLE mecontrola.cards
    RENAME CONSTRAINT cards_closing_day_chk TO cards_archived_20260609120000_closing_day_chk;

ALTER TABLE mecontrola.cards
    RENAME CONSTRAINT cards_due_day_chk TO cards_archived_20260609120000_due_day_chk;

ALTER TABLE mecontrola.cards
    RENAME CONSTRAINT cards_name_len_chk TO cards_archived_20260609120000_name_len_chk;

ALTER TABLE mecontrola.cards
    RENAME CONSTRAINT cards_nickname_len_chk TO cards_archived_20260609120000_nickname_len_chk;

ALTER TABLE mecontrola.cards RENAME TO cards_archived_20260609120000;
