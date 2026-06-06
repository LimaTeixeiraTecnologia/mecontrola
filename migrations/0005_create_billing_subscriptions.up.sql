CREATE TABLE billing_subscriptions (
    id                     UUID PRIMARY KEY,
    funnel_token           TEXT NOT NULL,
    user_id                UUID NULL,
    kiwify_order_id        TEXT NOT NULL,
    kiwify_subscription_id TEXT NULL,
    plan_code              TEXT NOT NULL REFERENCES billing_plans(code),
    status                 TEXT NOT NULL CHECK (status IN ('TRIALING', 'ACTIVE', 'PAST_DUE', 'CANCELED_PENDING', 'EXPIRED', 'REFUNDED')),
    period_start           TIMESTAMPTZ NOT NULL,
    period_end             TIMESTAMPTZ NOT NULL,
    grace_end              TIMESTAMPTZ NULL,
    last_event_at          TIMESTAMPTZ NOT NULL,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX billing_subscriptions_user_active_uniq_idx
    ON billing_subscriptions (user_id)
    WHERE user_id IS NOT NULL
      AND status IN ('ACTIVE', 'PAST_DUE', 'CANCELED_PENDING');

CREATE UNIQUE INDEX billing_subscriptions_kiwify_order_uniq_idx
    ON billing_subscriptions (kiwify_order_id);

CREATE INDEX billing_subscriptions_funnel_token_idx
    ON billing_subscriptions (funnel_token);
