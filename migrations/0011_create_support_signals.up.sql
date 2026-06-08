CREATE TABLE onboarding.support_signals (
    id           UUID         PRIMARY KEY,
    kind         TEXT         NOT NULL CHECK (kind IN ('orphan_expired_subscription','paid_without_token','token_reuse_attempt')),
    payload      JSONB        NOT NULL,
    occurred_at  TIMESTAMPTZ  NOT NULL DEFAULT now(),
    resolved_at  TIMESTAMPTZ  NULL,
    resolved_by  TEXT         NULL,
    notes        TEXT         NULL
);

CREATE INDEX idx_support_signals_kind_open ON onboarding.support_signals (kind, occurred_at)
    WHERE resolved_at IS NULL;
