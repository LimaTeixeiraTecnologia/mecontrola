SET LOCAL lock_timeout    = '5s';
SET LOCAL statement_timeout = '120s';

-- ============================================================
-- Transactions: recurring templates aceitam dia 1..31
-- Materialização faz clamp para o último dia em meses curtos
-- ============================================================

ALTER TABLE mecontrola.transactions_recurring_templates
    DROP CONSTRAINT transactions_rt_day_of_month_chk,
    ADD CONSTRAINT transactions_rt_day_of_month_chk
        CHECK (day_of_month BETWEEN 1 AND 31);
