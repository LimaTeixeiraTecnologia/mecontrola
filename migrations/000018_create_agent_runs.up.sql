SET LOCAL lock_timeout    = '5s';
SET LOCAL statement_timeout = '120s';

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
