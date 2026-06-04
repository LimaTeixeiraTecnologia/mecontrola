-- migration: 0010_billing_plans_seed.down.sql
-- Remove os planos do MVP inseridos por 0010_billing_plans_seed.up.sql.

DELETE FROM billing_plans WHERE plan_code IN ('MONTHLY', 'QUARTERLY', 'ANNUAL');
