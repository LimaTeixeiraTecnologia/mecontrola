SET LOCAL lock_timeout    = '5s';
SET LOCAL statement_timeout = '120s';

-- Descontinuacao do internal/agent: remove as 7 tabelas agent_* (prd-platform-mastra spec-version 2).
DROP TABLE IF EXISTS
    mecontrola.agent_processed_events,
    mecontrola.agent_observations,
    mecontrola.agent_working_memory,
    mecontrola.agent_runs,
    mecontrola.agent_threads,
    mecontrola.agent_decisions,
    mecontrola.agent_sessions
    CASCADE;

CREATE EXTENSION IF NOT EXISTS vector WITH SCHEMA mecontrola;

CREATE TABLE IF NOT EXISTS mecontrola.platform_threads (
    id         uuid                     NOT NULL,
    resource_id text                    NOT NULL,
    thread_id  text                     NOT NULL,
    title      text                     DEFAULT ''::text NOT NULL,
    metadata   jsonb                    DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT platform_threads_pkey PRIMARY KEY (id),
    CONSTRAINT platform_threads_resource_thread_uniq UNIQUE (resource_id, thread_id),
    CONSTRAINT platform_threads_resource_len_chk
        CHECK (((char_length(resource_id) >= 1) AND (char_length(resource_id) <= 256))),
    CONSTRAINT platform_threads_thread_len_chk
        CHECK (((char_length(thread_id) >= 1) AND (char_length(thread_id) <= 256)))
);

CREATE INDEX IF NOT EXISTS platform_threads_resource_id_idx
    ON mecontrola.platform_threads USING btree (resource_id);

CREATE TABLE IF NOT EXISTS mecontrola.platform_resources (
    resource_id    text                     NOT NULL,
    working_memory text                     DEFAULT ''::text NOT NULL,
    metadata       jsonb                    DEFAULT '{}'::jsonb NOT NULL,
    updated_at     timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT platform_resources_pkey PRIMARY KEY (resource_id),
    CONSTRAINT platform_resources_resource_len_chk
        CHECK (((char_length(resource_id) >= 1) AND (char_length(resource_id) <= 256)))
);

CREATE TABLE IF NOT EXISTS mecontrola.platform_messages (
    id          uuid                     NOT NULL,
    thread_pk   uuid                     NOT NULL,
    resource_id text                     NOT NULL,
    role        text                     NOT NULL,
    content     text                     DEFAULT ''::text NOT NULL,
    parts       jsonb                    DEFAULT '[]'::jsonb NOT NULL,
    created_at  timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT platform_messages_pkey PRIMARY KEY (id),
    CONSTRAINT platform_messages_thread_fkey
        FOREIGN KEY (thread_pk) REFERENCES mecontrola.platform_threads(id) ON DELETE CASCADE,
    CONSTRAINT platform_messages_resource_len_chk
        CHECK (((char_length(resource_id) >= 1) AND (char_length(resource_id) <= 256))),
    CONSTRAINT platform_messages_role_chk
        CHECK ((role = ANY (ARRAY['user'::text, 'assistant'::text, 'tool'::text, 'system'::text])))
);

CREATE INDEX IF NOT EXISTS platform_messages_thread_created_idx
    ON mecontrola.platform_messages USING btree (thread_pk, created_at);

CREATE TABLE IF NOT EXISTS mecontrola.platform_runs (
    id              uuid                     NOT NULL,
    thread_pk       uuid                     NOT NULL,
    resource_id     text                     NOT NULL,
    thread_id       text                     NOT NULL,
    agent_id        text                     DEFAULT ''::text NOT NULL,
    workflow        text                     DEFAULT ''::text NOT NULL,
    correlation_key text                     DEFAULT ''::text NOT NULL,
    status          text                     NOT NULL,
    outcome         text                     DEFAULT ''::text NOT NULL,
    error           text                     DEFAULT ''::text NOT NULL,
    started_at      timestamp with time zone DEFAULT now() NOT NULL,
    ended_at        timestamp with time zone,
    duration_ms     bigint                   DEFAULT 0 NOT NULL,
    CONSTRAINT platform_runs_pkey PRIMARY KEY (id),
    CONSTRAINT platform_runs_thread_fkey
        FOREIGN KEY (thread_pk) REFERENCES mecontrola.platform_threads(id) ON DELETE CASCADE,
    CONSTRAINT platform_runs_duration_chk CHECK ((duration_ms >= 0)),
    CONSTRAINT platform_runs_resource_len_chk
        CHECK (((char_length(resource_id) >= 1) AND (char_length(resource_id) <= 256))),
    CONSTRAINT platform_runs_status_chk
        CHECK ((status = ANY (ARRAY['running'::text, 'succeeded'::text, 'failed'::text]))),
    CONSTRAINT platform_runs_thread_len_chk
        CHECK (((char_length(thread_id) >= 1) AND (char_length(thread_id) <= 256)))
);

CREATE INDEX IF NOT EXISTS platform_runs_resource_started_idx
    ON mecontrola.platform_runs USING btree (resource_id, started_at DESC);

CREATE INDEX IF NOT EXISTS platform_runs_thread_started_idx
    ON mecontrola.platform_runs USING btree (thread_pk, started_at DESC);

CREATE TABLE IF NOT EXISTS mecontrola.platform_embeddings (
    id                uuid                     NOT NULL,
    resource_id       text                     NOT NULL,
    thread_id         text                     NOT NULL,
    source_message_pk uuid,
    content           text                     NOT NULL,
    embedding         mecontrola.vector(1536)  NOT NULL,
    model             text                     DEFAULT ''::text NOT NULL,
    created_at        timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT platform_embeddings_pkey PRIMARY KEY (id),
    CONSTRAINT platform_embeddings_resource_len_chk
        CHECK (((char_length(resource_id) >= 1) AND (char_length(resource_id) <= 256))),
    CONSTRAINT platform_embeddings_thread_len_chk
        CHECK (((char_length(thread_id) >= 1) AND (char_length(thread_id) <= 256)))
);

CREATE INDEX IF NOT EXISTS platform_embeddings_hnsw_idx
    ON mecontrola.platform_embeddings USING hnsw (embedding mecontrola.vector_cosine_ops);

CREATE INDEX IF NOT EXISTS platform_embeddings_resource_idx
    ON mecontrola.platform_embeddings USING btree (resource_id);

CREATE UNIQUE INDEX IF NOT EXISTS platform_embeddings_source_model_uniq
    ON mecontrola.platform_embeddings USING btree (source_message_pk, model)
    WHERE (source_message_pk IS NOT NULL);

CREATE TABLE IF NOT EXISTS mecontrola.platform_scorer_results (
    id         uuid                     NOT NULL,
    run_id     uuid                     NOT NULL,
    scorer_id  text                     NOT NULL,
    kind       text                     NOT NULL,
    score      double precision         DEFAULT 0 NOT NULL,
    reason     text                     DEFAULT ''::text NOT NULL,
    metadata   jsonb                    DEFAULT '{}'::jsonb NOT NULL,
    sampled    boolean                  DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT platform_scorer_results_pkey PRIMARY KEY (id),
    CONSTRAINT platform_scorer_results_run_fkey
        FOREIGN KEY (run_id) REFERENCES mecontrola.platform_runs(id) ON DELETE CASCADE,
    CONSTRAINT platform_scorer_results_kind_chk
        CHECK ((kind = ANY (ARRAY['code_based'::text, 'llm_judged'::text]))),
    CONSTRAINT platform_scorer_results_score_chk
        CHECK (((score >= (0)::double precision) AND (score <= (1)::double precision)))
);

CREATE INDEX IF NOT EXISTS platform_scorer_results_run_idx
    ON mecontrola.platform_scorer_results USING btree (run_id);
