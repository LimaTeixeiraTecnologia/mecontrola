SET LOCAL lock_timeout    = '5s';
SET LOCAL statement_timeout = '120s';

CREATE TABLE IF NOT EXISTS mecontrola.agent_threads (
    id         UUID        NOT NULL,
    user_id    UUID        NOT NULL,
    channel    TEXT        NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT agent_threads_pkey PRIMARY KEY (id),
    CONSTRAINT agent_threads_user_id_fkey
        FOREIGN KEY (user_id) REFERENCES mecontrola.users (id) ON DELETE CASCADE,
    CONSTRAINT agent_threads_channel_len_chk
        CHECK (char_length(channel) BETWEEN 1 AND 32),
    CONSTRAINT agent_threads_user_channel_uniq
        UNIQUE (user_id, channel)
);
