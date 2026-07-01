SET LOCAL lock_timeout    = '5s';
SET LOCAL statement_timeout = '120s';

CREATE TABLE IF NOT EXISTS mecontrola.agents_write_ledger (
    id            UUID        NOT NULL DEFAULT gen_random_uuid(),
    user_id       UUID        NOT NULL,
    wamid         TEXT        NOT NULL,
    item_seq      INT         NOT NULL,
    operation     TEXT        NOT NULL,
    resource_id   UUID        NOT NULL,
    resource_kind TEXT        NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT agents_write_ledger_pkey
        PRIMARY KEY (id),

    CONSTRAINT agents_write_ledger_wamid_nonempty_check
        CHECK (length(wamid) > 0),

    CONSTRAINT agents_write_ledger_operation_nonempty_check
        CHECK (length(operation) > 0),

    CONSTRAINT agents_write_ledger_item_seq_positive_check
        CHECK (item_seq >= 0),

    CONSTRAINT agents_write_ledger_uniq
        UNIQUE (wamid, item_seq, operation)
);

CREATE INDEX IF NOT EXISTS agents_write_ledger_user_created_idx
    ON mecontrola.agents_write_ledger (user_id, created_at);
