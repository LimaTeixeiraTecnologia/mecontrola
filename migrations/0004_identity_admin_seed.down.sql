-- migration: 0004_identity_admin_seed.down.sql
-- No-op explícito: a promoção de admins não é revertida automaticamente.
-- Admins podem ser revogados manualmente via UPDATE ou por fluxo administrativo futuro.
DO $$
BEGIN
    RAISE NOTICE 'identity: down de 0004_identity_admin_seed é no-op — promoção de admins não revertida automaticamente';
END $$;
