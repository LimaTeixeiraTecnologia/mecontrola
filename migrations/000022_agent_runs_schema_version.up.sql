SET LOCAL lock_timeout    = '5s';
SET LOCAL statement_timeout = '120s';

ALTER TABLE mecontrola.agent_runs
    ADD COLUMN IF NOT EXISTS schema_version TEXT NOT NULL DEFAULT 'v1';
