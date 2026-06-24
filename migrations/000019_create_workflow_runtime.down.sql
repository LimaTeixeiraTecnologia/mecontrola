SET LOCAL lock_timeout    = '5s';
SET LOCAL statement_timeout = '120s';

DROP INDEX IF EXISTS mecontrola.workflow_steps_run_seq_idx;

DROP TABLE IF EXISTS mecontrola.workflow_steps;

DROP INDEX IF EXISTS mecontrola.workflow_runs_status_updated_idx;
DROP INDEX IF EXISTS mecontrola.workflow_runs_active_key_uidx;

DROP TABLE IF EXISTS mecontrola.workflow_runs;
