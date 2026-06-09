SET LOCAL lock_timeout    = '5s';
SET LOCAL statement_timeout = '120s';

DROP INDEX IF EXISTS mecontrola.idempotency_keys_expires_idx;

DO $$
DECLARE
    v_keys_exists     BOOLEAN;
    v_archived_exists BOOLEAN;
BEGIN
    SELECT EXISTS (
        SELECT 1 FROM information_schema.tables
        WHERE table_schema = 'mecontrola' AND table_name = 'idempotency_keys'
    ) INTO v_keys_exists;

    SELECT EXISTS (
        SELECT 1 FROM information_schema.tables
        WHERE table_schema = 'mecontrola' AND table_name = 'idempotency_keys_archived_20260609120000'
    ) INTO v_archived_exists;

    IF v_keys_exists AND NOT v_archived_exists THEN
        ALTER TABLE mecontrola.idempotency_keys
            RENAME CONSTRAINT idempotency_keys_pkey TO idempotency_keys_archived_20260609120000_pkey;
        ALTER TABLE mecontrola.idempotency_keys
            RENAME CONSTRAINT idempotency_keys_key_len_chk TO idempotency_keys_archived_20260609120000_key_len_chk;
        ALTER TABLE mecontrola.idempotency_keys
            RENAME CONSTRAINT idempotency_keys_request_hash_len_chk TO idempotency_keys_archived_20260609120000_request_hash_len_chk;
        ALTER TABLE mecontrola.idempotency_keys
            RENAME CONSTRAINT idempotency_keys_status_chk TO idempotency_keys_archived_20260609120000_status_chk;
        ALTER TABLE mecontrola.idempotency_keys
            RENAME CONSTRAINT idempotency_keys_body_size_chk TO idempotency_keys_archived_20260609120000_body_size_chk;
        ALTER TABLE mecontrola.idempotency_keys RENAME TO idempotency_keys_archived_20260609120000;
    END IF;
END $$;
