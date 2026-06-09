-- Seed condicional: só executa em databases cujo nome contém 'staging'.
DO $$
DECLARE
    smoke_wa TEXT := current_setting('app.smoke_wa', true);
    smoke_id UUID := '00000000-0000-0000-0000-00005a17c8e7'::uuid;
BEGIN
    IF current_database() !~ 'staging' THEN
        RAISE NOTICE 'skipped seed: not staging database';
        RETURN;
    END IF;
    IF smoke_wa IS NULL OR smoke_wa = '' THEN
        RAISE EXCEPTION 'STAGING_SMOKE_WA não configurado (use ALTER DATABASE ... SET app.smoke_wa = ''<E164>'')';
    END IF;
    INSERT INTO users (id, whatsapp_number, status, created_at, updated_at)
    VALUES (smoke_id, smoke_wa, 'ACTIVE', now(), now())
    ON CONFLICT (id) DO NOTHING;
END $$;
