SET LOCAL lock_timeout    = '5s';
SET LOCAL statement_timeout = '120s';

ALTER TABLE mecontrola.onboarding_sessions
    DROP CONSTRAINT IF EXISTS onboarding_sessions_channel_chk;

ALTER TABLE mecontrola.onboarding_sessions
    ADD CONSTRAINT onboarding_sessions_channel_chk
        CHECK (channel IN ('whatsapp', 'telegram'));

ALTER TABLE mecontrola.user_identities
    DROP CONSTRAINT IF EXISTS user_identities_channel_check;

ALTER TABLE mecontrola.user_identities
    ADD CONSTRAINT user_identities_channel_check
        CHECK (channel IN ('whatsapp', 'telegram'));

ALTER TABLE mecontrola.channel_processed_messages
    DROP CONSTRAINT IF EXISTS channel_processed_messages_channel_check;

ALTER TABLE mecontrola.channel_processed_messages
    ADD CONSTRAINT channel_processed_messages_channel_check
        CHECK (channel IN ('whatsapp', 'telegram'));

ALTER TABLE mecontrola.onboarding_tokens
    ADD COLUMN IF NOT EXISTS telegram_external_id TEXT NULL;

CREATE INDEX IF NOT EXISTS onboarding_tokens_telegram_external_id_idx
    ON mecontrola.onboarding_tokens (telegram_external_id)
    WHERE telegram_external_id IS NOT NULL;
