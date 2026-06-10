SET LOCAL lock_timeout    = '5s';
SET LOCAL statement_timeout = '120s';

DROP INDEX IF EXISTS mecontrola.budgets_pending_identity_idx;
DROP INDEX IF EXISTS mecontrola.budgets_pending_state_received_idx;
DROP TABLE IF EXISTS mecontrola.budgets_expense_events_pending;

DROP INDEX IF EXISTS mecontrola.budgets_alerts_listing_idx;
DROP INDEX IF EXISTS mecontrola.budgets_alerts_user_comp_idx;
DROP TABLE IF EXISTS mecontrola.budgets_alerts;

DROP TABLE IF EXISTS mecontrola.budgets_threshold_states;

DROP INDEX IF EXISTS mecontrola.budgets_expenses_deleted_at_idx;
DROP INDEX IF EXISTS mecontrola.budgets_expenses_summary_root_idx;
DROP INDEX IF EXISTS mecontrola.budgets_expenses_summary_idx;
DROP TABLE IF EXISTS mecontrola.budgets_expenses;

DROP TABLE IF EXISTS mecontrola.budgets_allocations;

DROP INDEX IF EXISTS mecontrola.budgets_competence_idx;
DROP TABLE IF EXISTS mecontrola.budgets;
