SET LOCAL lock_timeout    = '5s';
SET LOCAL statement_timeout = '120s';

CREATE INDEX IF NOT EXISTS onboarding_tokens_mobile_activable_idx
    ON mecontrola.onboarding_tokens (customer_mobile_e164, paid_at)
    WHERE status = 'PAID';

ALTER TABLE mecontrola.onboarding_tokens
    ADD COLUMN IF NOT EXISTS email_sent_at         TIMESTAMPTZ NULL,
    ADD COLUMN IF NOT EXISTS page_opened_at        TIMESTAMPTZ NULL,
    ADD COLUMN IF NOT EXISTS activation_started_at TIMESTAMPTZ NULL,
    ADD COLUMN IF NOT EXISTS whatsapp_opened_at    TIMESTAMPTZ NULL;

CREATE TABLE IF NOT EXISTS mecontrola.onboarding_activation_nomatch_throttle (
    mobile_e164  TEXT        NOT NULL,
    window_start TIMESTAMPTZ NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT onboarding_activation_nomatch_throttle_pkey PRIMARY KEY (mobile_e164, window_start)
);

CREATE TABLE IF NOT EXISTS mecontrola.onboarding_welcome_processed (
    event_id     TEXT        NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT onboarding_welcome_processed_pkey PRIMARY KEY (event_id),
    CONSTRAINT onboarding_welcome_processed_event_id_nonempty_check CHECK (length(event_id) > 0)
);
