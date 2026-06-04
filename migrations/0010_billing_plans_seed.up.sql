-- migration: 0010_billing_plans_seed.up.sql
-- D-03 do PRD: seed dos 3 planos fixos do MVP (valores definitivos).
-- Idempotente via ON CONFLICT DO NOTHING.

INSERT INTO billing_plans (plan_code, display_name, period_length_days, price_brl_cents)
VALUES
    ('MONTHLY',   'Mensal',     30,  2990),
    ('QUARTERLY', 'Trimestral', 90,  8073),
    ('ANNUAL',    'Anual',      365, 29780)
ON CONFLICT (plan_code) DO NOTHING;
