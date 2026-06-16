DO $$
DECLARE
    smoke_wa TEXT := current_setting('app.smoke_wa', true);
    smoke_id UUID := '00000000-0000-0000-0000-00005a17c8e7';
BEGIN
    IF smoke_wa IS NULL OR smoke_wa = '' THEN
        RAISE NOTICE 'seed skipped: app.smoke_wa not configured';
        RETURN;
    END IF;
    INSERT INTO mecontrola.users (id, whatsapp_number, status, created_at, updated_at)
    VALUES (smoke_id, smoke_wa, 'ACTIVE', now(), now())
    ON CONFLICT (id) DO NOTHING;
    RAISE NOTICE 'smoke user seeded: % -> %', smoke_wa, smoke_id;
END $$;
