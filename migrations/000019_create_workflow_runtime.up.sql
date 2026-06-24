SET LOCAL lock_timeout    = '5s';
SET LOCAL statement_timeout = '120s';

CREATE TABLE IF NOT EXISTS mecontrola.workflow_runs (
    id              UUID        NOT NULL,
    workflow        TEXT        NOT NULL,
    correlation_key TEXT        NOT NULL,
    status          TEXT        NOT NULL,
    suspend_reason  TEXT        NOT NULL DEFAULT '',
    cursor          INT         NOT NULL DEFAULT 0,
    state           JSONB       NOT NULL DEFAULT '{}',
    attempts        INT         NOT NULL DEFAULT 0,
    max_attempts    INT         NOT NULL,
    version         BIGINT      NOT NULL DEFAULT 1,
    last_error      TEXT        NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    ended_at        TIMESTAMPTZ,
    CONSTRAINT workflow_runs_pkey PRIMARY KEY (id),
    CONSTRAINT workflow_runs_status_check
        CHECK (status IN ('running','suspended','succeeded','failed')),
    CONSTRAINT workflow_runs_attempts_check     CHECK (attempts >= 0),
    CONSTRAINT workflow_runs_max_attempts_check CHECK (max_attempts > 0)
) WITH (fillfactor = 70);

CREATE UNIQUE INDEX IF NOT EXISTS workflow_runs_active_key_uidx
    ON mecontrola.workflow_runs (workflow, correlation_key)
    WHERE status IN ('running','suspended');

CREATE INDEX IF NOT EXISTS workflow_runs_status_updated_idx
    ON mecontrola.workflow_runs (status, updated_at);

CREATE TABLE IF NOT EXISTS mecontrola.workflow_steps (
    id          UUID        NOT NULL,
    run_id      UUID        NOT NULL,
    step_id     TEXT        NOT NULL,
    seq         INT         NOT NULL,
    status      TEXT        NOT NULL,
    attempt     INT         NOT NULL DEFAULT 1,
    duration_ms BIGINT      NOT NULL DEFAULT 0,
    error       TEXT        NOT NULL DEFAULT '',
    started_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    ended_at    TIMESTAMPTZ,
    CONSTRAINT workflow_steps_pkey PRIMARY KEY (id),
    CONSTRAINT workflow_steps_run_fkey
        FOREIGN KEY (run_id) REFERENCES mecontrola.workflow_runs (id) ON DELETE CASCADE,
    CONSTRAINT workflow_steps_status_check
        CHECK (status IN ('completed','suspended','failed','skipped'))
);

CREATE INDEX IF NOT EXISTS workflow_steps_run_seq_idx
    ON mecontrola.workflow_steps (run_id, seq);
