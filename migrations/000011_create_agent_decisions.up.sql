SET LOCAL lock_timeout    = '5s';
SET LOCAL statement_timeout = '120s';

CREATE TABLE IF NOT EXISTS mecontrola.agent_decisions (
    id                 UUID        NOT NULL,
    user_id            UUID        NOT NULL,
    channel            TEXT        NOT NULL,
    message_id         TEXT        NOT NULL,
    intent_kind        TEXT        NOT NULL,
    prompt_sha256      TEXT        NOT NULL,
    llm_model          TEXT        NOT NULL,
    redacted_response  JSONB       NOT NULL DEFAULT '{}'::jsonb,
    trace_id           TEXT        NOT NULL DEFAULT '',
    decided_action     TEXT        NOT NULL,
    resulting_event_id UUID,
    status             TEXT        NOT NULL,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    settled_at         TIMESTAMPTZ,
    CONSTRAINT agent_decisions_pkey PRIMARY KEY (id),
    CONSTRAINT agent_decisions_user_id_fkey
        FOREIGN KEY (user_id) REFERENCES mecontrola.users (id) ON DELETE CASCADE,
    CONSTRAINT agent_decisions_channel_len_chk
        CHECK (char_length(channel) BETWEEN 1 AND 32),
    CONSTRAINT agent_decisions_message_id_len_chk
        CHECK (char_length(message_id) BETWEEN 1 AND 256),
    CONSTRAINT agent_decisions_intent_kind_len_chk
        CHECK (char_length(intent_kind) BETWEEN 1 AND 64),
    CONSTRAINT agent_decisions_prompt_sha256_len_chk
        CHECK (char_length(prompt_sha256) = 64),
    CONSTRAINT agent_decisions_llm_model_len_chk
        CHECK (char_length(llm_model) BETWEEN 1 AND 128),
    CONSTRAINT agent_decisions_decided_action_len_chk
        CHECK (char_length(decided_action) BETWEEN 1 AND 64),
    CONSTRAINT agent_decisions_redacted_response_size_chk
        CHECK (octet_length(redacted_response::text) <= 65536),
    CONSTRAINT agent_decisions_status_chk
        CHECK (status IN ('pending', 'executed', 'rejected', 'awaiting_confirmation'))
);

CREATE UNIQUE INDEX IF NOT EXISTS agent_decisions_user_channel_message_uniq_idx
    ON mecontrola.agent_decisions (user_id, channel, message_id);

CREATE INDEX IF NOT EXISTS agent_decisions_user_created_idx
    ON mecontrola.agent_decisions (user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS agent_decisions_status_idx
    ON mecontrola.agent_decisions (status);
