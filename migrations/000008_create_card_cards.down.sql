SET LOCAL lock_timeout    = '5s';
SET LOCAL statement_timeout = '120s';

DROP INDEX IF EXISTS mecontrola.cards_user_pagination_idx;
DROP INDEX IF EXISTS mecontrola.cards_user_nickname_active_uniq_idx;

DO $$
DECLARE
    v_cards_exists    BOOLEAN;
    v_archived_exists BOOLEAN;
BEGIN
    SELECT EXISTS (
        SELECT 1 FROM information_schema.tables
        WHERE table_schema = 'mecontrola' AND table_name = 'cards'
    ) INTO v_cards_exists;

    SELECT EXISTS (
        SELECT 1 FROM information_schema.tables
        WHERE table_schema = 'mecontrola' AND table_name = 'cards_archived_20260609120000'
    ) INTO v_archived_exists;

    IF v_cards_exists AND NOT v_archived_exists THEN
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
    END IF;
END $$;
