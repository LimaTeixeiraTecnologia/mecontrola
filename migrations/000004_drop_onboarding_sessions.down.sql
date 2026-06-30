CREATE TABLE IF NOT EXISTS mecontrola.onboarding_sessions (
    user_id    UUID        NOT NULL,
    channel    TEXT        NOT NULL,
    state      TEXT        NOT NULL,
    payload    JSONB       NOT NULL DEFAULT '{}'::jsonb,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT onboarding_sessions_pkey                  PRIMARY KEY (user_id),
    CONSTRAINT onboarding_sessions_user_id_fk
        FOREIGN KEY (user_id) REFERENCES mecontrola.users (id) ON DELETE CASCADE,
    CONSTRAINT onboarding_sessions_channel_chk
        CHECK (channel IN ('whatsapp')),
    CONSTRAINT onboarding_sessions_state_nonempty_chk
        CHECK (length(state) > 0)
);

CREATE INDEX IF NOT EXISTS onboarding_sessions_channel_state_idx
    ON mecontrola.onboarding_sessions (channel, state);
