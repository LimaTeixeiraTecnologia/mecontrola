SET LOCAL lock_timeout      = '5s';
SET LOCAL statement_timeout = '120s';

-- Reversao aditiva (ADR-005 item 4 Rollback, ADR-006 Plano item 2).
-- Remove a coluna resolve_path e as duas constraints. O backfill de
-- platform_runs.correlation_key NAO e desfeito: e idempotente e 'legacy:' || id
-- e um valor valido e permanente. auth_events_reason_check permanece intacta.

ALTER TABLE mecontrola.platform_runs
    DROP CONSTRAINT IF EXISTS platform_runs_correlation_len_chk;

ALTER TABLE mecontrola.auth_events
    DROP CONSTRAINT IF EXISTS auth_events_resolve_path_chk;

ALTER TABLE mecontrola.auth_events
    DROP COLUMN IF EXISTS resolve_path;
