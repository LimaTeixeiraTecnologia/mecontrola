CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE INDEX IF NOT EXISTS dictionary_term_trgm_idx
    ON mecontrola.category_dictionary
    USING gin (term_normalized gin_trgm_ops)
    WHERE deprecated_at IS NULL;
