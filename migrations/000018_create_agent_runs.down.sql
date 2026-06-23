SET LOCAL lock_timeout    = '5s';
SET LOCAL statement_timeout = '120s';

DROP INDEX IF EXISTS mecontrola.agent_runs_user_started_idx;
DROP INDEX IF EXISTS mecontrola.agent_runs_thread_started_idx;

DROP TABLE IF EXISTS mecontrola.agent_runs;
