SET LOCAL lock_timeout    = '5s';
SET LOCAL statement_timeout = '120s';

DROP TABLE IF EXISTS mecontrola.onboarding_activation_nomatch_throttle;

DROP TABLE IF EXISTS mecontrola.onboarding_welcome_processed;

ALTER TABLE mecontrola.onboarding_tokens
    DROP COLUMN IF EXISTS email_sent_at,
    DROP COLUMN IF EXISTS page_opened_at,
    DROP COLUMN IF EXISTS activation_started_at,
    DROP COLUMN IF EXISTS whatsapp_opened_at;

DROP INDEX IF EXISTS mecontrola.onboarding_tokens_mobile_activable_idx;
