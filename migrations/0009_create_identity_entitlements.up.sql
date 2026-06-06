CREATE TABLE identity_entitlements (
    user_id         UUID PRIMARY KEY REFERENCES users(id),
    subscription_id UUID NOT NULL,
    status          TEXT NOT NULL,
    period_end      TIMESTAMPTZ NOT NULL,
    grace_end       TIMESTAMPTZ NULL,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX identity_entitlements_subscription_id_idx
    ON identity_entitlements (subscription_id);

CREATE TABLE identity_entitlements_pending (
    subscription_id UUID PRIMARY KEY,
    funnel_token    TEXT NOT NULL,
    payload         JSONB NOT NULL,
    received_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX identity_entitlements_pending_funnel_token_idx
    ON identity_entitlements_pending (funnel_token);
