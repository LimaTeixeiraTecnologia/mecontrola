-- Reconcilia bancos pre-existentes com os nomes canonicos declarados no baseline 000001
-- (platform_thread_id / source_message_id). Idempotente: em bancos criados ja pelo baseline
-- atual os nomes novos existem e todos os guards viram no-op.

SET LOCAL lock_timeout      = '5s';
SET LOCAL statement_timeout = '120s';

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'mecontrola' AND table_name = 'platform_messages' AND column_name = 'thread_pk'
    ) THEN
        ALTER TABLE mecontrola.platform_messages RENAME COLUMN thread_pk TO platform_thread_id;
    END IF;

    IF EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'platform_messages_thread_fkey'
          AND conrelid = 'mecontrola.platform_messages'::regclass
    ) THEN
        ALTER TABLE mecontrola.platform_messages
            RENAME CONSTRAINT platform_messages_thread_fkey TO platform_messages_platform_thread_id_fkey;
    END IF;

    IF EXISTS (
        SELECT 1 FROM pg_indexes
        WHERE schemaname = 'mecontrola' AND indexname = 'platform_messages_thread_created_idx'
    ) THEN
        ALTER INDEX mecontrola.platform_messages_thread_created_idx
            RENAME TO platform_messages_platform_thread_id_created_idx;
    END IF;

    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'mecontrola' AND table_name = 'platform_runs' AND column_name = 'thread_pk'
    ) THEN
        ALTER TABLE mecontrola.platform_runs RENAME COLUMN thread_pk TO platform_thread_id;
    END IF;

    IF EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'platform_runs_thread_fkey'
          AND conrelid = 'mecontrola.platform_runs'::regclass
    ) THEN
        ALTER TABLE mecontrola.platform_runs
            RENAME CONSTRAINT platform_runs_thread_fkey TO platform_runs_platform_thread_id_fkey;
    END IF;

    IF EXISTS (
        SELECT 1 FROM pg_indexes
        WHERE schemaname = 'mecontrola' AND indexname = 'platform_runs_thread_started_idx'
    ) THEN
        ALTER INDEX mecontrola.platform_runs_thread_started_idx
            RENAME TO platform_runs_platform_thread_id_started_idx;
    END IF;

    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'mecontrola' AND table_name = 'platform_embeddings' AND column_name = 'source_message_pk'
    ) THEN
        ALTER TABLE mecontrola.platform_embeddings RENAME COLUMN source_message_pk TO source_message_id;
    END IF;
END $$;
