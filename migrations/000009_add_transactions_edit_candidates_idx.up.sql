SET LOCAL lock_timeout      = '5s';
SET LOCAL statement_timeout = '120s';

-- Indice de suporte para SearchEditCandidates (ADR-007, prd-operacao-conversacional-diaria):
-- a busca de candidatos de edicao filtra por user_id + ref_month e ordena por created_at DESC,
-- casando por amount_cents ou description ILIKE. Sem este indice composto, o planner recai em
-- transactions_user_month_idx (user_id, ref_month) seguido de sort explicito.
--
-- Ref: postgresql.org/docs/17/indexes-multicolumn.html
-- Ref: postgresql.org/docs/17/indexes-ordering.html

CREATE INDEX IF NOT EXISTS transactions_user_month_created_idx
    ON mecontrola.transactions (user_id, ref_month, created_at DESC);
