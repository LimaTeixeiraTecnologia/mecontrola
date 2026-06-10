SET LOCAL lock_timeout    = '5s';
SET LOCAL statement_timeout = '120s';

CREATE OR REPLACE FUNCTION mecontrola.immutable_unaccent(text)
RETURNS text
LANGUAGE sql
IMMUTABLE
PARALLEL SAFE
AS $$
    SELECT unaccent($1);
$$;

CREATE TABLE mecontrola.category_editorial_version (
    version     BIGINT      NOT NULL PRIMARY KEY DEFAULT 1,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO mecontrola.category_editorial_version (version) VALUES (1)
ON CONFLICT DO NOTHING;

CREATE TABLE mecontrola.categories (
    id                UUID        NOT NULL PRIMARY KEY,
    slug              TEXT        NOT NULL,
    name              TEXT        NOT NULL,
    kind              TEXT        NOT NULL,
    parent_id         UUID        NULL REFERENCES mecontrola.categories(id),
    allocation_type   TEXT        NOT NULL DEFAULT 'consumption',
    deprecated_at     TIMESTAMPTZ NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT categories_kind_check CHECK (kind IN ('income', 'expense')),
    CONSTRAINT categories_allocation_type_check CHECK (allocation_type IN ('consumption', 'asset_allocation')),
    CONSTRAINT categories_no_cycles CHECK (parent_id IS NULL OR parent_id <> id)
);

CREATE OR REPLACE FUNCTION mecontrola.categories_parent_same_kind()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
    parent_kind TEXT;
BEGIN
    IF NEW.parent_id IS NULL THEN
        RETURN NEW;
    END IF;
    SELECT kind INTO parent_kind FROM mecontrola.categories WHERE id = NEW.parent_id;
    IF parent_kind IS NULL THEN
        RAISE EXCEPTION 'categories_parent_same_kind: parent_id % not found', NEW.parent_id;
    END IF;
    IF parent_kind <> NEW.kind THEN
        RAISE EXCEPTION 'categories_parent_same_kind: child kind % does not match parent kind %', NEW.kind, parent_kind;
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER categories_parent_same_kind_trg
    BEFORE INSERT OR UPDATE OF parent_id, kind ON mecontrola.categories
    FOR EACH ROW
    EXECUTE FUNCTION mecontrola.categories_parent_same_kind();

CREATE UNIQUE INDEX categories_kind_slug_uniq_idx
    ON mecontrola.categories (kind, slug);

CREATE INDEX categories_kind_parent_idx
    ON mecontrola.categories (kind, parent_id)
    WHERE deprecated_at IS NULL;

CREATE INDEX categories_parent_sort_idx
    ON mecontrola.categories (parent_id, name COLLATE "pt-BR-x-icu")
    WHERE deprecated_at IS NULL;

CREATE TABLE mecontrola.category_dictionary (
    id                UUID        NOT NULL PRIMARY KEY,
    category_id       UUID        NOT NULL REFERENCES mecontrola.categories(id),
    kind              TEXT        NOT NULL,
    term              TEXT        NOT NULL,
    term_normalized   TEXT        GENERATED ALWAYS AS (lower(mecontrola.immutable_unaccent(term))) STORED,
    signal_type       TEXT        NOT NULL,
    confidence        TEXT        NOT NULL,
    is_ambiguous      BOOLEAN     NOT NULL DEFAULT false,
    deprecated_at     TIMESTAMPTZ NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT dictionary_kind_check CHECK (kind IN ('income', 'expense')),
    CONSTRAINT dictionary_signal_type_check CHECK (signal_type IN ('canonical_name', 'alias', 'phrase', 'merchant', 'segment')),
    CONSTRAINT dictionary_confidence_check CHECK (confidence IN ('high', 'medium', 'low'))
);

CREATE UNIQUE INDEX dictionary_active_term_uniq_idx
    ON mecontrola.category_dictionary (kind, category_id, term_normalized)
    WHERE deprecated_at IS NULL;

CREATE INDEX dictionary_term_normalized_idx
    ON mecontrola.category_dictionary (term_normalized)
    WHERE deprecated_at IS NULL;

CREATE INDEX dictionary_kind_term_normalized_idx
    ON mecontrola.category_dictionary (kind, term_normalized)
    WHERE deprecated_at IS NULL;
