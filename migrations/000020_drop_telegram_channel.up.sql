SET LOCAL lock_timeout    = '5s';
SET LOCAL statement_timeout = '120s';

DROP INDEX IF EXISTS mecontrola.onboarding_tokens_telegram_external_id_idx;

ALTER TABLE mecontrola.onboarding_tokens DROP COLUMN IF EXISTS telegram_external_id;

DELETE FROM mecontrola.channel_processed_messages WHERE channel = 'telegram';

ALTER TABLE mecontrola.channel_processed_messages
    DROP CONSTRAINT IF EXISTS channel_processed_messages_channel_check;

ALTER TABLE mecontrola.channel_processed_messages
    ADD CONSTRAINT channel_processed_messages_channel_check
        CHECK (channel IN ('whatsapp'));

ALTER TABLE mecontrola.user_identities
    DROP CONSTRAINT IF EXISTS user_identities_channel_check;

ALTER TABLE mecontrola.user_identities
    ADD CONSTRAINT user_identities_channel_check
        CHECK (channel IN ('whatsapp'));

ALTER TABLE mecontrola.onboarding_sessions
    DROP CONSTRAINT IF EXISTS onboarding_sessions_channel_chk;

ALTER TABLE mecontrola.onboarding_sessions
    ADD CONSTRAINT onboarding_sessions_channel_chk
        CHECK (channel IN ('whatsapp'));
