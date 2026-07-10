SET LOCAL lock_timeout      = '5s';
SET LOCAL statement_timeout = '120s';

-- Migration aditiva e reentrante (RF-15, RF-21).
--
-- Bloco 1 (auth_events): coluna aditiva resolve_path para trilha de auditoria do
-- caminho de resolucao de principal (ADR-006, item 2). ADD COLUMN de coluna nullable
-- e operacao de metadados (nao reescreve a tabela). A constraint auth_events_reason_check
-- existente permanece intacta: motivo de falha (reason) e caminho de resolucao
-- (resolve_path) sao eixos ortogonais em colunas separadas.
--
-- Bloco 2 (platform_runs): backfill idempotente dos runs legados com correlation_key
-- vazio (pre-regressao, ADR-005 item 4 / R1) executado ANTES do ADD CONSTRAINT de
-- comprimento; caso contrario o CHECK validado abortaria contra os runs legados vazios.
--
-- Ref: postgresql.org/docs/17/sql-altertable.html
-- Ref: postgresql.org/docs/17/ddl-constraints.html#DDL-CONSTRAINTS-CHECK-CONSTRAINTS

ALTER TABLE mecontrola.auth_events
    ADD COLUMN IF NOT EXISTS resolve_path TEXT NULL;

ALTER TABLE mecontrola.auth_events
    DROP CONSTRAINT IF EXISTS auth_events_resolve_path_chk;

ALTER TABLE mecontrola.auth_events
    ADD CONSTRAINT auth_events_resolve_path_chk
    CHECK (resolve_path IS NULL OR resolve_path IN ('identity', 'legacy', 'backfill'));

UPDATE mecontrola.platform_runs
    SET correlation_key = 'legacy:' || id::text
    WHERE correlation_key = '';

ALTER TABLE mecontrola.platform_runs
    DROP CONSTRAINT IF EXISTS platform_runs_correlation_len_chk;

ALTER TABLE mecontrola.platform_runs
    ADD CONSTRAINT platform_runs_correlation_len_chk
    CHECK (char_length(correlation_key) BETWEEN 1 AND 256);
