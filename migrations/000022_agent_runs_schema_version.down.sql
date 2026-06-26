SET LOCAL lock_timeout    = '5s';
SET LOCAL statement_timeout = '120s';

ALTER TABLE mecontrola.agent_runs
    DROP COLUMN IF EXISTS schema_version;
