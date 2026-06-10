SET LOCAL lock_timeout    = '5s';
SET LOCAL statement_timeout = '120s';

CREATE TABLE IF NOT EXISTS mecontrola.budgets (
    id           UUID        NOT NULL,
    user_id      UUID        NOT NULL,
    competence   CHAR(7)     NOT NULL,
    total_cents  BIGINT      NOT NULL DEFAULT 0,
    state        SMALLINT    NOT NULL,
    activated_at TIMESTAMPTZ NULL,
    auto_draft   BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at   TIMESTAMPTZ NOT NULL,
    updated_at   TIMESTAMPTZ NOT NULL,
    CONSTRAINT budgets_pkey            PRIMARY KEY (id),
    CONSTRAINT budgets_user_comp_uk    UNIQUE (user_id, competence)
);

CREATE INDEX IF NOT EXISTS budgets_competence_idx
    ON mecontrola.budgets (competence);

CREATE TABLE IF NOT EXISTS mecontrola.budgets_allocations (
    budget_id     UUID    NOT NULL,
    root_slug     TEXT    NOT NULL,
    basis_points  INTEGER NOT NULL,
    planned_cents BIGINT  NOT NULL,
    CONSTRAINT budgets_allocations_pkey             PRIMARY KEY (budget_id, root_slug),
    CONSTRAINT budgets_allocations_budget_fk        FOREIGN KEY (budget_id)
        REFERENCES mecontrola.budgets(id) ON DELETE CASCADE,
    CONSTRAINT budgets_allocations_basis_points_chk CHECK (basis_points >= 0 AND basis_points <= 10000),
    CONSTRAINT budgets_allocations_root_chk         CHECK (root_slug IN (
        'expense.custo_fixo',
        'expense.conhecimento',
        'expense.prazeres',
        'expense.metas',
        'expense.liberdade_financeira'
    ))
);

CREATE TABLE IF NOT EXISTS mecontrola.budgets_expenses (
    id                      UUID        NOT NULL,
    user_id                 UUID        NOT NULL,
    source                  TEXT        NOT NULL,
    external_transaction_id TEXT        NOT NULL,
    subcategory_id          UUID        NOT NULL,
    root_slug               TEXT        NOT NULL,
    competence              CHAR(7)     NOT NULL,
    amount_cents            BIGINT      NOT NULL,
    occurred_at             TIMESTAMPTZ NOT NULL,
    version                 BIGINT      NOT NULL,
    tombstone_version       BIGINT      NULL,
    deleted_at              TIMESTAMPTZ NULL,
    created_at              TIMESTAMPTZ NOT NULL,
    updated_at              TIMESTAMPTZ NOT NULL,
    CONSTRAINT budgets_expenses_pkey        PRIMARY KEY (id),
    CONSTRAINT budgets_expenses_amount_chk  CHECK (amount_cents > 0),
    CONSTRAINT budgets_expenses_identity_uk UNIQUE (user_id, source, external_transaction_id),
    CONSTRAINT budgets_expenses_root_chk    CHECK (root_slug IN (
        'expense.custo_fixo',
        'expense.conhecimento',
        'expense.prazeres',
        'expense.metas',
        'expense.liberdade_financeira'
    ))
);

CREATE INDEX IF NOT EXISTS budgets_expenses_summary_idx
    ON mecontrola.budgets_expenses (user_id, competence, subcategory_id)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS budgets_expenses_summary_root_idx
    ON mecontrola.budgets_expenses (user_id, competence, root_slug)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS budgets_expenses_deleted_at_idx
    ON mecontrola.budgets_expenses (deleted_at)
    WHERE deleted_at IS NOT NULL;

CREATE TABLE IF NOT EXISTS mecontrola.budgets_threshold_states (
    user_id                     UUID        NOT NULL,
    competence                  CHAR(7)     NOT NULL,
    root_slug                   TEXT        NOT NULL,
    threshold                   SMALLINT    NOT NULL,
    currently_crossed           BOOLEAN     NOT NULL DEFAULT FALSE,
    version                     BIGINT      NOT NULL DEFAULT 0,
    last_crossed_at             TIMESTAMPTZ NULL,
    last_uncrossed_at           TIMESTAMPTZ NULL,
    last_evaluated_committed_at TIMESTAMPTZ NULL,
    CONSTRAINT budgets_threshold_states_pkey          PRIMARY KEY (user_id, competence, root_slug, threshold),
    CONSTRAINT budgets_threshold_states_threshold_chk CHECK (threshold IN (80, 100))
);

CREATE TABLE IF NOT EXISTS mecontrola.budgets_alerts (
    id                        UUID        NOT NULL,
    user_id                   UUID        NOT NULL,
    competence                CHAR(7)     NOT NULL,
    root_slug                 TEXT        NOT NULL,
    threshold                 SMALLINT    NOT NULL,
    state                     SMALLINT    NOT NULL,
    triggered_by_committed_at TIMESTAMPTZ NOT NULL,
    spent_cents               BIGINT      NOT NULL,
    planned_cents             BIGINT      NOT NULL,
    created_at                TIMESTAMPTZ NOT NULL,
    CONSTRAINT budgets_alerts_pkey          PRIMARY KEY (id),
    CONSTRAINT budgets_alerts_threshold_chk CHECK (threshold IN (80, 100))
);

CREATE INDEX IF NOT EXISTS budgets_alerts_user_comp_idx
    ON mecontrola.budgets_alerts (user_id, competence, root_slug, threshold);

CREATE INDEX IF NOT EXISTS budgets_alerts_listing_idx
    ON mecontrola.budgets_alerts (user_id, created_at DESC)
    WHERE state IN (1, 2);

CREATE TABLE IF NOT EXISTS mecontrola.budgets_expense_events_pending (
    id                      UUID        NOT NULL,
    event_id                UUID        NOT NULL,
    source                  TEXT        NOT NULL,
    user_id                 UUID        NOT NULL,
    external_transaction_id TEXT        NOT NULL,
    expected_version        BIGINT      NOT NULL,
    mutation_kind           SMALLINT    NOT NULL,
    payload                 JSONB       NOT NULL,
    state                   SMALLINT    NOT NULL,
    received_at             TIMESTAMPTZ NOT NULL,
    transitioned_at         TIMESTAMPTZ NULL,
    reason                  TEXT        NULL,
    CONSTRAINT budgets_expense_events_pending_pkey     PRIMARY KEY (id),
    CONSTRAINT budgets_expense_events_pending_event_uk UNIQUE (event_id)
);

CREATE INDEX IF NOT EXISTS budgets_pending_state_received_idx
    ON mecontrola.budgets_expense_events_pending (state, received_at)
    WHERE state = 1;

CREATE INDEX IF NOT EXISTS budgets_pending_identity_idx
    ON mecontrola.budgets_expense_events_pending (user_id, source, external_transaction_id)
    WHERE state = 1;
