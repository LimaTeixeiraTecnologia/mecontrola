SET LOCAL lock_timeout    = '5s';
SET LOCAL statement_timeout = '120s';

-- ============================================================
-- Alertas Proativos Release 1 (.specs/prd-alertas-proativos)
-- Amplia o dominio de `kind` para os 4 alertas do Release 1:
--   category_threshold_80, category_threshold_100,
--   budget_missing_month_start, budget_not_reviewed_day_3
-- Kinds legados permanecem aceitos para nao invalidar linhas historicas.
-- `category_threshold_90` NAO entra: ADR-003 mantem 90 bloqueado ate
-- migration e modelagem propria (espelha budgets_threshold_states_threshold_chk).
-- `budget_missing_month_start` nao possui budget: usa budget_id = uuid nulo
-- como sentinela, coerente com a PK (user_id, budget_id, kind, ref_day).
-- ============================================================

ALTER TABLE mecontrola.budget_alerts_sent
    DROP CONSTRAINT IF EXISTS budget_alerts_sent_kind_chk;

ALTER TABLE mecontrola.budget_alerts_sent
    ADD CONSTRAINT budget_alerts_sent_kind_chk
        CHECK (kind IN (
            'category_threshold',
            'goal_achieved',
            'card_limit_near',
            'category_threshold_80',
            'category_threshold_100',
            'budget_missing_month_start',
            'budget_not_reviewed_day_3'
        ));
