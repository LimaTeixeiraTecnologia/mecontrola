SET LOCAL lock_timeout    = '5s';
SET LOCAL statement_timeout = '120s';

DROP TRIGGER IF EXISTS categories_parent_kind_change_blocks_children_trg ON mecontrola.categories;
DROP FUNCTION IF EXISTS mecontrola.categories_parent_kind_change_blocks_children();

DROP INDEX IF EXISTS mecontrola.dictionary_kind_term_normalized_idx;
DROP INDEX IF EXISTS mecontrola.dictionary_term_normalized_idx;

CREATE INDEX dictionary_term_normalized_idx
    ON mecontrola.category_dictionary (term_normalized)
    WHERE deprecated_at IS NULL;

CREATE INDEX dictionary_kind_term_normalized_idx
    ON mecontrola.category_dictionary (kind, term_normalized)
    WHERE deprecated_at IS NULL;
