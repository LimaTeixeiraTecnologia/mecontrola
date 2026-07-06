SET LOCAL lock_timeout      = '5s';
SET LOCAL statement_timeout = '120s';

-- =============================================================================
-- Revert 000004: drop triggers, function, constraints, FKs and category columns.
-- subcategory_id and subcategory_name_snapshot return to nullable state.
-- =============================================================================

DROP TRIGGER IF EXISTS transactions_category_write_gate_trg ON mecontrola.transactions;
DROP TRIGGER IF EXISTS transactions_recurring_templates_category_write_gate_trg ON mecontrola.transactions_recurring_templates;

DROP FUNCTION IF EXISTS mecontrola.validate_category_write_gate();

ALTER TABLE mecontrola.transactions
    DROP CONSTRAINT IF EXISTS transactions_category_fk,
    DROP CONSTRAINT IF EXISTS transactions_subcategory_fk,
    DROP CONSTRAINT IF EXISTS transactions_subcategory_ne_category_chk,
    DROP CONSTRAINT IF EXISTS transactions_direction_chk,
    DROP CONSTRAINT IF EXISTS transactions_category_name_snapshot_chk,
    DROP CONSTRAINT IF EXISTS transactions_subcategory_name_snapshot_chk,
    DROP CONSTRAINT IF EXISTS transactions_category_kind_chk,
    DROP CONSTRAINT IF EXISTS transactions_category_path_chk,
    DROP CONSTRAINT IF EXISTS transactions_category_outcome_chk,
    DROP CONSTRAINT IF EXISTS transactions_category_score_chk,
    DROP CONSTRAINT IF EXISTS transactions_category_confidence_chk,
    DROP CONSTRAINT IF EXISTS transactions_category_match_quality_chk,
    DROP CONSTRAINT IF EXISTS transactions_category_signal_type_chk,
    DROP CONSTRAINT IF EXISTS transactions_category_matched_term_chk,
    DROP CONSTRAINT IF EXISTS transactions_category_match_reason_chk,
    DROP CONSTRAINT IF EXISTS transactions_category_decision_source_chk,
    DROP CONSTRAINT IF EXISTS transactions_category_editorial_version_chk,
    ALTER COLUMN subcategory_id DROP NOT NULL,
    ALTER COLUMN subcategory_name_snapshot DROP NOT NULL,
    DROP COLUMN IF EXISTS category_kind,
    DROP COLUMN IF EXISTS category_path,
    DROP COLUMN IF EXISTS category_outcome,
    DROP COLUMN IF EXISTS category_score,
    DROP COLUMN IF EXISTS category_confidence,
    DROP COLUMN IF EXISTS category_match_quality,
    DROP COLUMN IF EXISTS category_signal_type,
    DROP COLUMN IF EXISTS category_matched_term,
    DROP COLUMN IF EXISTS category_match_reason,
    DROP COLUMN IF EXISTS category_decision_source,
    DROP COLUMN IF EXISTS category_editorial_version,
    DROP COLUMN IF EXISTS category_decided_at;

ALTER TABLE mecontrola.transactions_recurring_templates
    DROP CONSTRAINT IF EXISTS transactions_rt_category_fk,
    DROP CONSTRAINT IF EXISTS transactions_rt_subcategory_fk,
    DROP CONSTRAINT IF EXISTS transactions_rt_subcategory_ne_category_chk,
    DROP CONSTRAINT IF EXISTS transactions_rt_direction_chk,
    DROP CONSTRAINT IF EXISTS transactions_rt_category_name_snapshot_chk,
    DROP CONSTRAINT IF EXISTS transactions_rt_subcategory_name_snapshot_chk,
    DROP CONSTRAINT IF EXISTS transactions_rt_category_kind_chk,
    DROP CONSTRAINT IF EXISTS transactions_rt_category_path_chk,
    DROP CONSTRAINT IF EXISTS transactions_rt_category_outcome_chk,
    DROP CONSTRAINT IF EXISTS transactions_rt_category_score_chk,
    DROP CONSTRAINT IF EXISTS transactions_rt_category_confidence_chk,
    DROP CONSTRAINT IF EXISTS transactions_rt_category_match_quality_chk,
    DROP CONSTRAINT IF EXISTS transactions_rt_category_signal_type_chk,
    DROP CONSTRAINT IF EXISTS transactions_rt_category_matched_term_chk,
    DROP CONSTRAINT IF EXISTS transactions_rt_category_match_reason_chk,
    DROP CONSTRAINT IF EXISTS transactions_rt_category_decision_source_chk,
    DROP CONSTRAINT IF EXISTS transactions_rt_category_editorial_version_chk,
    ALTER COLUMN subcategory_id DROP NOT NULL,
    ALTER COLUMN subcategory_name_snapshot DROP NOT NULL,
    DROP COLUMN IF EXISTS category_kind,
    DROP COLUMN IF EXISTS category_path,
    DROP COLUMN IF EXISTS category_outcome,
    DROP COLUMN IF EXISTS category_score,
    DROP COLUMN IF EXISTS category_confidence,
    DROP COLUMN IF EXISTS category_match_quality,
    DROP COLUMN IF EXISTS category_signal_type,
    DROP COLUMN IF EXISTS category_matched_term,
    DROP COLUMN IF EXISTS category_match_reason,
    DROP COLUMN IF EXISTS category_decision_source,
    DROP COLUMN IF EXISTS category_editorial_version,
    DROP COLUMN IF EXISTS category_decided_at;
