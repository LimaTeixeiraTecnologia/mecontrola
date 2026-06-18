SET LOCAL lock_timeout    = '5s';
SET LOCAL statement_timeout = '120s';

CREATE TABLE IF NOT EXISTS mecontrola.agent_sessions (
    id             UUID        NOT NULL,
    user_id        UUID        NOT NULL,
    channel        TEXT        NOT NULL,
    pending_action JSONB       NOT NULL DEFAULT '{}'::jsonb,
    recent_turns   JSONB       NOT NULL DEFAULT '[]'::jsonb,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at     TIMESTAMPTZ NOT NULL,
    CONSTRAINT agent_sessions_pkey PRIMARY KEY (id),
    CONSTRAINT agent_sessions_user_id_fkey
        FOREIGN KEY (user_id) REFERENCES mecontrola.users (id) ON DELETE CASCADE,
    CONSTRAINT agent_sessions_channel_len_chk
        CHECK (char_length(channel) BETWEEN 1 AND 32),
    CONSTRAINT agent_sessions_pending_action_size_chk
        CHECK (octet_length(pending_action::text) <= 16384),
    CONSTRAINT agent_sessions_recent_turns_size_chk
        CHECK (octet_length(recent_turns::text) <= 65536)
);

CREATE UNIQUE INDEX IF NOT EXISTS agent_sessions_user_channel_uniq_idx
    ON mecontrola.agent_sessions (user_id, channel);

CREATE INDEX IF NOT EXISTS agent_sessions_expires_idx
    ON mecontrola.agent_sessions (expires_at);
