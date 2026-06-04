-- migration: 0004_identity_admin_seed.up.sql
-- Promove admins iniciais a partir de ADMIN_WHATSAPP_NUMBERS (CSV) via current_setting.
-- Idempotente: UPDATE só promove existentes; números ausentes ficam para promoção pós-onboarding.
-- Esta migration NÃO cria usuários — apenas promove. A criação ocorre via fluxo normal de upsert.

DO $$
DECLARE
    raw    TEXT := current_setting('app.admin_whatsapp_numbers', true);
    parts  TEXT[];
    nbr    TEXT;
BEGIN
    IF raw IS NULL OR raw = '' THEN
        RAISE NOTICE 'identity: ADMIN_WHATSAPP_NUMBERS vazio — nenhum admin promovido';
        RETURN;
    END IF;
    parts := string_to_array(raw, ',');
    FOREACH nbr IN ARRAY parts LOOP
        UPDATE users
           SET is_admin = true, updated_at = now()
         WHERE whatsapp_number = trim(nbr)
           AND deleted_at IS NULL;
    END LOOP;
END $$;
