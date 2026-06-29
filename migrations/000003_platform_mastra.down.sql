SET LOCAL lock_timeout    = '5s';
SET LOCAL statement_timeout = '120s';

DROP TABLE IF EXISTS
    mecontrola.platform_scorer_results,
    mecontrola.platform_embeddings,
    mecontrola.platform_runs,
    mecontrola.platform_messages,
    mecontrola.platform_resources,
    mecontrola.platform_threads
    CASCADE;

DROP EXTENSION IF EXISTS vector;

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
    step_index         INT         NOT NULL DEFAULT 0,
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

CREATE UNIQUE INDEX IF NOT EXISTS agent_decisions_user_channel_message_step_idx
    ON mecontrola.agent_decisions (user_id, channel, message_id, step_index);

CREATE INDEX IF NOT EXISTS agent_decisions_user_created_idx
    ON mecontrola.agent_decisions (user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS agent_decisions_status_idx
    ON mecontrola.agent_decisions (status);

CREATE TABLE IF NOT EXISTS mecontrola.agent_working_memory (
    user_id    UUID        NOT NULL,
    content    TEXT        NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT agent_working_memory_pk PRIMARY KEY (user_id)
);

CREATE EXTENSION IF NOT EXISTS pgcrypto WITH SCHEMA mecontrola;

CREATE TABLE IF NOT EXISTS mecontrola.agent_observations (
    id         UUID        NOT NULL DEFAULT gen_random_uuid(),
    user_id    UUID        NOT NULL,
    channel    TEXT        NOT NULL,
    content    TEXT        NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL DEFAULT now() + interval '90 days',
    CONSTRAINT agent_observations_pk PRIMARY KEY (id)
);

CREATE INDEX IF NOT EXISTS agent_observations_user_channel_idx
    ON mecontrola.agent_observations (user_id, channel, created_at DESC);
CREATE INDEX IF NOT EXISTS agent_observations_expires_at_idx
    ON mecontrola.agent_observations (expires_at);

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

CREATE TABLE IF NOT EXISTS mecontrola.agent_runs (
    id          UUID        NOT NULL,
    thread_id   UUID        NOT NULL,
    user_id     UUID        NOT NULL,
    channel     TEXT        NOT NULL,
    message_id  TEXT        NOT NULL DEFAULT '',
    agent_id    TEXT        NOT NULL DEFAULT '',
    workflow    TEXT        NOT NULL DEFAULT '',
    tool_name   TEXT        NOT NULL DEFAULT '',
    intent_kind TEXT        NOT NULL DEFAULT '',
    outcome     TEXT        NOT NULL DEFAULT '',
    status      TEXT        NOT NULL,
    error       TEXT        NOT NULL DEFAULT '',
    decision_id UUID,
    started_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    ended_at    TIMESTAMPTZ,
    duration_ms BIGINT      NOT NULL DEFAULT 0,
    CONSTRAINT agent_runs_pkey PRIMARY KEY (id),
    CONSTRAINT agent_runs_thread_id_fkey
        FOREIGN KEY (thread_id) REFERENCES mecontrola.agent_threads (id) ON DELETE CASCADE,
    CONSTRAINT agent_runs_decision_id_fkey
        FOREIGN KEY (decision_id) REFERENCES mecontrola.agent_decisions (id) ON DELETE SET NULL,
    CONSTRAINT agent_runs_channel_len_chk
        CHECK (char_length(channel) BETWEEN 1 AND 32),
    CONSTRAINT agent_runs_status_chk
        CHECK (status IN ('running', 'succeeded', 'failed'))
);

CREATE INDEX IF NOT EXISTS agent_runs_thread_started_idx
    ON mecontrola.agent_runs (thread_id, started_at DESC);

CREATE INDEX IF NOT EXISTS agent_runs_user_started_idx
    ON mecontrola.agent_runs (user_id, started_at DESC);

CREATE TABLE IF NOT EXISTS mecontrola.agent_processed_events (
    event_id          UUID        NOT NULL,
    event_type        TEXT        NOT NULL,
    aggregate_user_id UUID        NULL,
    processed_at      TIMESTAMPTZ NOT NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT agent_processed_events_pkey PRIMARY KEY (event_id)
);

CREATE INDEX IF NOT EXISTS agent_processed_events_aggregate_user_id_idx
    ON mecontrola.agent_processed_events (aggregate_user_id);
