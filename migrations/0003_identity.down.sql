-- migration: 0003_identity.down.sql
DROP INDEX IF EXISTS idx_user_whatsapp_history_number;
DROP INDEX IF EXISTS idx_user_whatsapp_history_user_id_active;
DROP TABLE IF EXISTS user_whatsapp_history;
DROP INDEX IF EXISTS idx_users_status;
DROP INDEX IF EXISTS uq_users_email;
DROP INDEX IF EXISTS uq_users_whatsapp_number;
DROP TABLE IF EXISTS users;
