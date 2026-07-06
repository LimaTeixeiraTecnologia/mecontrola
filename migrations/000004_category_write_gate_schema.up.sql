SET LOCAL lock_timeout      = '5s';
SET LOCAL statement_timeout = '120s';

-- =============================================================================
-- Backfill helper: add new category_* columns as nullable first so existing
-- rows in dev/staging can be hydrated before NOT NULL and CHECK constraints
-- are applied. Production rows are empty, so the UPDATEs are no-ops there.
-- =============================================================================

ALTER TABLE mecontrola.transactions
    ADD COLUMN IF NOT EXISTS category_kind               TEXT         NULL,
    ADD COLUMN IF NOT EXISTS category_path               TEXT         NULL,
    ADD COLUMN IF NOT EXISTS category_outcome            TEXT         NULL,
    ADD COLUMN IF NOT EXISTS category_score              NUMERIC(5,4) NULL,
    ADD COLUMN IF NOT EXISTS category_confidence         TEXT         NULL,
    ADD COLUMN IF NOT EXISTS category_match_quality      TEXT         NULL,
    ADD COLUMN IF NOT EXISTS category_signal_type        TEXT         NULL,
    ADD COLUMN IF NOT EXISTS category_matched_term       TEXT         NULL,
    ADD COLUMN IF NOT EXISTS category_match_reason       TEXT         NULL,
    ADD COLUMN IF NOT EXISTS category_decision_source    TEXT         NULL,
    ADD COLUMN IF NOT EXISTS category_editorial_version  BIGINT       NULL,
    ADD COLUMN IF NOT EXISTS category_decided_at         TIMESTAMPTZ  NULL;

ALTER TABLE mecontrola.transactions_recurring_templates
    ADD COLUMN IF NOT EXISTS category_kind               TEXT         NULL,
    ADD COLUMN IF NOT EXISTS category_path               TEXT         NULL,
    ADD COLUMN IF NOT EXISTS category_outcome            TEXT         NULL,
    ADD COLUMN IF NOT EXISTS category_score              NUMERIC(5,4) NULL,
    ADD COLUMN IF NOT EXISTS category_confidence         TEXT         NULL,
    ADD COLUMN IF NOT EXISTS category_match_quality      TEXT         NULL,
    ADD COLUMN IF NOT EXISTS category_signal_type        TEXT         NULL,
    ADD COLUMN IF NOT EXISTS category_matched_term       TEXT         NULL,
    ADD COLUMN IF NOT EXISTS category_match_reason       TEXT         NULL,
    ADD COLUMN IF NOT EXISTS category_decision_source    TEXT         NULL,
    ADD COLUMN IF NOT EXISTS category_editorial_version  BIGINT       NULL,
    ADD COLUMN IF NOT EXISTS category_decided_at         TIMESTAMPTZ  NULL;

-- =============================================================================
-- Hydrate existing rows deterministically from the category tree.
-- If subcategory_id is missing, pick the first active leaf of category_id.
-- If no leaf exists, fall back to category_id itself (temporary dev/staging
-- safety valve; the subcategory_ne_category check will catch new writes).
-- =============================================================================

WITH hydrated_transactions AS (
    SELECT
        t.id AS transaction_id,
        COALESCE(
            t.subcategory_id,
            leaf.id,
            t.category_id
        ) AS new_subcategory_id,
        COALESCE(
            NULLIF(t.subcategory_name_snapshot, ''),
            leaf.name,
            'subcategoria'
        ) AS new_subcategory_name_snapshot,
        root.kind AS root_kind,
        root.slug AS root_slug,
        root.name AS root_name,
        COALESCE(leaf.slug, root.slug) AS leaf_slug,
        COALESCE(leaf.name, root.name) AS leaf_name
    FROM mecontrola.transactions t
    JOIN mecontrola.categories root ON root.id = t.category_id
    LEFT JOIN LATERAL (
        SELECT id, slug, name
        FROM mecontrola.categories
        WHERE parent_id = t.category_id
          AND deprecated_at IS NULL
        ORDER BY created_at ASC, id ASC
        LIMIT 1
    ) leaf ON true
    WHERE t.category_kind IS NULL
)
UPDATE mecontrola.transactions t
SET
    subcategory_id             = h.new_subcategory_id,
    subcategory_name_snapshot  = h.new_subcategory_name_snapshot,
    category_name_snapshot     = COALESCE(NULLIF(t.category_name_snapshot, ''), h.root_name),
    category_kind              = h.root_kind,
    category_path              = h.root_slug || '/' || h.leaf_slug,
    category_outcome           = 'matched',
    category_score             = 1.0000,
    category_confidence        = 'manual_confirmed',
    category_match_quality     = 'manual_canonical',
    category_signal_type       = 'manual_canonical',
    category_matched_term      = COALESCE(NULLIF(t.category_name_snapshot, ''), h.leaf_name),
    category_match_reason      = 'system_migration',
    category_decision_source   = 'system_migration',
    category_editorial_version = (SELECT version FROM mecontrola.category_editorial_version),
    category_decided_at        = COALESCE(t.updated_at, t.created_at, now())
FROM hydrated_transactions h
WHERE t.id = h.transaction_id;

WITH hydrated_templates AS (
    SELECT
        t.id AS template_id,
        COALESCE(
            t.subcategory_id,
            leaf.id,
            t.category_id
        ) AS new_subcategory_id,
        COALESCE(
            NULLIF(t.subcategory_name_snapshot, ''),
            leaf.name,
            'subcategoria'
        ) AS new_subcategory_name_snapshot,
        root.kind AS root_kind,
        root.slug AS root_slug,
        root.name AS root_name,
        COALESCE(leaf.slug, root.slug) AS leaf_slug,
        COALESCE(leaf.name, root.name) AS leaf_name
    FROM mecontrola.transactions_recurring_templates t
    JOIN mecontrola.categories root ON root.id = t.category_id
    LEFT JOIN LATERAL (
        SELECT id, slug, name
        FROM mecontrola.categories
        WHERE parent_id = t.category_id
          AND deprecated_at IS NULL
        ORDER BY created_at ASC, id ASC
        LIMIT 1
    ) leaf ON true
    WHERE t.category_kind IS NULL
)
UPDATE mecontrola.transactions_recurring_templates t
SET
    subcategory_id             = h.new_subcategory_id,
    subcategory_name_snapshot  = h.new_subcategory_name_snapshot,
    category_name_snapshot     = COALESCE(NULLIF(t.category_name_snapshot, ''), h.root_name),
    category_kind              = h.root_kind,
    category_path              = h.root_slug || '/' || h.leaf_slug,
    category_outcome           = 'matched',
    category_score             = 1.0000,
    category_confidence        = 'manual_confirmed',
    category_match_quality     = 'manual_canonical',
    category_signal_type       = 'manual_canonical',
    category_matched_term      = COALESCE(NULLIF(t.category_name_snapshot, ''), h.leaf_name),
    category_match_reason      = 'system_migration',
    category_decision_source   = 'system_migration',
    category_editorial_version = (SELECT version FROM mecontrola.category_editorial_version),
    category_decided_at        = COALESCE(t.updated_at, t.created_at, now())
FROM hydrated_templates h
WHERE t.id = h.template_id;

-- =============================================================================
-- Enforce NOT NULL on all category columns (existing and new).
-- =============================================================================

ALTER TABLE mecontrola.transactions
    ALTER COLUMN subcategory_id              SET NOT NULL,
    ALTER COLUMN subcategory_name_snapshot   SET NOT NULL,
    ALTER COLUMN category_kind               SET NOT NULL,
    ALTER COLUMN category_path               SET NOT NULL,
    ALTER COLUMN category_outcome            SET NOT NULL,
    ALTER COLUMN category_score              SET NOT NULL,
    ALTER COLUMN category_confidence         SET NOT NULL,
    ALTER COLUMN category_match_quality      SET NOT NULL,
    ALTER COLUMN category_signal_type        SET NOT NULL,
    ALTER COLUMN category_matched_term       SET NOT NULL,
    ALTER COLUMN category_match_reason       SET NOT NULL,
    ALTER COLUMN category_decision_source    SET NOT NULL,
    ALTER COLUMN category_editorial_version  SET NOT NULL,
    ALTER COLUMN category_decided_at         SET NOT NULL;

ALTER TABLE mecontrola.transactions_recurring_templates
    ALTER COLUMN subcategory_id              SET NOT NULL,
    ALTER COLUMN subcategory_name_snapshot   SET NOT NULL,
    ALTER COLUMN category_kind               SET NOT NULL,
    ALTER COLUMN category_path               SET NOT NULL,
    ALTER COLUMN category_outcome            SET NOT NULL,
    ALTER COLUMN category_score              SET NOT NULL,
    ALTER COLUMN category_confidence         SET NOT NULL,
    ALTER COLUMN category_match_quality      SET NOT NULL,
    ALTER COLUMN category_signal_type        SET NOT NULL,
    ALTER COLUMN category_matched_term       SET NOT NULL,
    ALTER COLUMN category_match_reason       SET NOT NULL,
    ALTER COLUMN category_decision_source    SET NOT NULL,
    ALTER COLUMN category_editorial_version  SET NOT NULL,
    ALTER COLUMN category_decided_at         SET NOT NULL;

-- =============================================================================
-- Idempotent constraint recreation: drop if exists, then add.
-- =============================================================================

ALTER TABLE mecontrola.transactions
    DROP CONSTRAINT IF EXISTS transactions_direction_chk,
    ADD  CONSTRAINT transactions_direction_chk CHECK (direction IN (1,2)),
    DROP CONSTRAINT IF EXISTS transactions_category_name_snapshot_chk,
    ADD  CONSTRAINT transactions_category_name_snapshot_chk CHECK (length(category_name_snapshot) > 0),
    DROP CONSTRAINT IF EXISTS transactions_subcategory_name_snapshot_chk,
    ADD  CONSTRAINT transactions_subcategory_name_snapshot_chk CHECK (length(subcategory_name_snapshot) > 0),
    DROP CONSTRAINT IF EXISTS transactions_category_kind_chk,
    ADD  CONSTRAINT transactions_category_kind_chk CHECK (category_kind IN ('expense','income')),
    DROP CONSTRAINT IF EXISTS transactions_category_path_chk,
    ADD  CONSTRAINT transactions_category_path_chk CHECK (length(category_path) > 0),
    DROP CONSTRAINT IF EXISTS transactions_category_outcome_chk,
    ADD  CONSTRAINT transactions_category_outcome_chk CHECK (category_outcome = 'matched'),
    DROP CONSTRAINT IF EXISTS transactions_category_score_chk,
    ADD  CONSTRAINT transactions_category_score_chk CHECK (category_score >= 0 AND category_score <= 1),
    DROP CONSTRAINT IF EXISTS transactions_category_confidence_chk,
    ADD  CONSTRAINT transactions_category_confidence_chk CHECK (category_confidence IN ('high','medium','low','manual_confirmed')),
    DROP CONSTRAINT IF EXISTS transactions_category_match_quality_chk,
    ADD  CONSTRAINT transactions_category_match_quality_chk CHECK (category_match_quality IN ('exact','token','fuzzy','manual_canonical')),
    DROP CONSTRAINT IF EXISTS transactions_category_signal_type_chk,
    ADD  CONSTRAINT transactions_category_signal_type_chk CHECK (category_signal_type IN ('canonical_name','alias','phrase','merchant','segment','manual_canonical')),
    DROP CONSTRAINT IF EXISTS transactions_category_matched_term_chk,
    ADD  CONSTRAINT transactions_category_matched_term_chk CHECK (length(category_matched_term) > 0),
    DROP CONSTRAINT IF EXISTS transactions_category_match_reason_chk,
    ADD  CONSTRAINT transactions_category_match_reason_chk CHECK (length(category_match_reason) > 0),
    DROP CONSTRAINT IF EXISTS transactions_category_decision_source_chk,
    ADD  CONSTRAINT transactions_category_decision_source_chk CHECK (category_decision_source IN ('auto_matched','user_selected_candidate','manual_canonical_id','system_migration')),
    DROP CONSTRAINT IF EXISTS transactions_category_editorial_version_chk,
    ADD  CONSTRAINT transactions_category_editorial_version_chk CHECK (category_editorial_version > 0),
    DROP CONSTRAINT IF EXISTS transactions_subcategory_ne_category_chk,
    ADD  CONSTRAINT transactions_subcategory_ne_category_chk CHECK (subcategory_id <> category_id);

ALTER TABLE mecontrola.transactions_recurring_templates
    DROP CONSTRAINT IF EXISTS transactions_rt_direction_chk,
    ADD  CONSTRAINT transactions_rt_direction_chk CHECK (direction IN (1,2)),
    DROP CONSTRAINT IF EXISTS transactions_rt_category_name_snapshot_chk,
    ADD  CONSTRAINT transactions_rt_category_name_snapshot_chk CHECK (length(category_name_snapshot) > 0),
    DROP CONSTRAINT IF EXISTS transactions_rt_subcategory_name_snapshot_chk,
    ADD  CONSTRAINT transactions_rt_subcategory_name_snapshot_chk CHECK (length(subcategory_name_snapshot) > 0),
    DROP CONSTRAINT IF EXISTS transactions_rt_category_kind_chk,
    ADD  CONSTRAINT transactions_rt_category_kind_chk CHECK (category_kind IN ('expense','income')),
    DROP CONSTRAINT IF EXISTS transactions_rt_category_path_chk,
    ADD  CONSTRAINT transactions_rt_category_path_chk CHECK (length(category_path) > 0),
    DROP CONSTRAINT IF EXISTS transactions_rt_category_outcome_chk,
    ADD  CONSTRAINT transactions_rt_category_outcome_chk CHECK (category_outcome = 'matched'),
    DROP CONSTRAINT IF EXISTS transactions_rt_category_score_chk,
    ADD  CONSTRAINT transactions_rt_category_score_chk CHECK (category_score >= 0 AND category_score <= 1),
    DROP CONSTRAINT IF EXISTS transactions_rt_category_confidence_chk,
    ADD  CONSTRAINT transactions_rt_category_confidence_chk CHECK (category_confidence IN ('high','medium','low','manual_confirmed')),
    DROP CONSTRAINT IF EXISTS transactions_rt_category_match_quality_chk,
    ADD  CONSTRAINT transactions_rt_category_match_quality_chk CHECK (category_match_quality IN ('exact','token','fuzzy','manual_canonical')),
    DROP CONSTRAINT IF EXISTS transactions_rt_category_signal_type_chk,
    ADD  CONSTRAINT transactions_rt_category_signal_type_chk CHECK (category_signal_type IN ('canonical_name','alias','phrase','merchant','segment','manual_canonical')),
    DROP CONSTRAINT IF EXISTS transactions_rt_category_matched_term_chk,
    ADD  CONSTRAINT transactions_rt_category_matched_term_chk CHECK (length(category_matched_term) > 0),
    DROP CONSTRAINT IF EXISTS transactions_rt_category_match_reason_chk,
    ADD  CONSTRAINT transactions_rt_category_match_reason_chk CHECK (length(category_match_reason) > 0),
    DROP CONSTRAINT IF EXISTS transactions_rt_category_decision_source_chk,
    ADD  CONSTRAINT transactions_rt_category_decision_source_chk CHECK (category_decision_source IN ('auto_matched','user_selected_candidate','manual_canonical_id','system_migration')),
    DROP CONSTRAINT IF EXISTS transactions_rt_category_editorial_version_chk,
    ADD  CONSTRAINT transactions_rt_category_editorial_version_chk CHECK (category_editorial_version > 0),
    DROP CONSTRAINT IF EXISTS transactions_rt_subcategory_ne_category_chk,
    ADD  CONSTRAINT transactions_rt_subcategory_ne_category_chk CHECK (subcategory_id <> category_id);

-- =============================================================================
-- Foreign keys (idempotent recreation).
-- =============================================================================

ALTER TABLE mecontrola.transactions
    DROP CONSTRAINT IF EXISTS transactions_category_fk,
    ADD  CONSTRAINT transactions_category_fk FOREIGN KEY (category_id) REFERENCES mecontrola.categories(id) ON DELETE RESTRICT,
    DROP CONSTRAINT IF EXISTS transactions_subcategory_fk,
    ADD  CONSTRAINT transactions_subcategory_fk FOREIGN KEY (subcategory_id) REFERENCES mecontrola.categories(id) ON DELETE RESTRICT;

ALTER TABLE mecontrola.transactions_recurring_templates
    DROP CONSTRAINT IF EXISTS transactions_rt_category_fk,
    ADD  CONSTRAINT transactions_rt_category_fk FOREIGN KEY (category_id) REFERENCES mecontrola.categories(id) ON DELETE RESTRICT,
    DROP CONSTRAINT IF EXISTS transactions_rt_subcategory_fk,
    ADD  CONSTRAINT transactions_rt_subcategory_fk FOREIGN KEY (subcategory_id) REFERENCES mecontrola.categories(id) ON DELETE RESTRICT;

-- =============================================================================
-- Trigger function and BEFORE INSERT OR UPDATE triggers.
-- =============================================================================

CREATE OR REPLACE FUNCTION mecontrola.validate_category_write_gate()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
    v_root    mecontrola.categories%ROWTYPE;
    v_leaf    mecontrola.categories%ROWTYPE;
    v_version BIGINT;
BEGIN
    SELECT * INTO v_root FROM mecontrola.categories WHERE id = NEW.category_id;
    IF NOT FOUND OR v_root.parent_id IS NOT NULL THEN
        RAISE EXCEPTION 'category_must_be_root: category_id=% is not a root category', NEW.category_id;
    END IF;

    SELECT * INTO v_leaf FROM mecontrola.categories WHERE id = NEW.subcategory_id;
    IF NOT FOUND OR v_leaf.parent_id IS DISTINCT FROM NEW.category_id THEN
        RAISE EXCEPTION 'subcategory_must_be_direct_leaf: subcategory_id=% is not direct child of category_id=%', NEW.subcategory_id, NEW.category_id;
    END IF;

    IF v_root.kind <> v_leaf.kind THEN
        RAISE EXCEPTION 'category_kind_mismatch: root kind=% leaf kind=%', v_root.kind, v_leaf.kind;
    END IF;

    IF NEW.direction = 1 AND v_root.kind <> 'income' THEN
        RAISE EXCEPTION 'category_direction_kind_mismatch: direction=income requires kind=income got=%', v_root.kind;
    END IF;

    IF NEW.direction = 2 AND v_root.kind <> 'expense' THEN
        RAISE EXCEPTION 'category_direction_kind_mismatch: direction=expense requires kind=expense got=%', v_root.kind;
    END IF;

    IF v_root.deprecated_at IS NOT NULL THEN
        RAISE EXCEPTION 'root_category_deprecated: category_id=%', NEW.category_id;
    END IF;

    IF v_leaf.deprecated_at IS NOT NULL THEN
        RAISE EXCEPTION 'leaf_category_deprecated: subcategory_id=%', NEW.subcategory_id;
    END IF;

    IF NEW.category_kind <> v_root.kind THEN
        RAISE EXCEPTION 'category_kind_column_drift: persisted=% real=%', NEW.category_kind, v_root.kind;
    END IF;

    SELECT version INTO v_version FROM mecontrola.category_editorial_version;
    IF NEW.category_editorial_version <> v_version THEN
        RAISE EXCEPTION 'category_editorial_version_drift: persisted=% current=%', NEW.category_editorial_version, v_version;
    END IF;

    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS transactions_category_write_gate_trg ON mecontrola.transactions;
CREATE TRIGGER transactions_category_write_gate_trg
    BEFORE INSERT OR UPDATE ON mecontrola.transactions
    FOR EACH ROW EXECUTE FUNCTION mecontrola.validate_category_write_gate();

DROP TRIGGER IF EXISTS transactions_recurring_templates_category_write_gate_trg ON mecontrola.transactions_recurring_templates;
CREATE TRIGGER transactions_recurring_templates_category_write_gate_trg
    BEFORE INSERT OR UPDATE ON mecontrola.transactions_recurring_templates
    FOR EACH ROW EXECUTE FUNCTION mecontrola.validate_category_write_gate();
