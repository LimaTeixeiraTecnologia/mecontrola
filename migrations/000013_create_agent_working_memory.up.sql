CREATE TABLE mecontrola.agent_working_memory (
    user_id    UUID        NOT NULL,
    content    TEXT        NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT agent_working_memory_pk PRIMARY KEY (user_id)
);
