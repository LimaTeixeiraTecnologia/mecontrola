CREATE TABLE IF NOT EXISTS mecontrola.budgets_abandoned_draft_signals (
    budget_id  UUID NOT NULL,
    signaled_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT budgets_abandoned_draft_signals_pk PRIMARY KEY (budget_id)
);
