SET LOCAL lock_timeout    = '5s';
SET LOCAL statement_timeout = '120s';

-- ============================================================
-- Transactions: reverte recurring templates para dia 1..28
-- Falha intencionalmente se existir template com dia 29..31
-- ============================================================

ALTER TABLE mecontrola.transactions_recurring_templates
    DROP CONSTRAINT transactions_rt_day_of_month_chk,
    ADD CONSTRAINT transactions_rt_day_of_month_chk
        CHECK (day_of_month BETWEEN 1 AND 28);
