SET LOCAL lock_timeout    = '5s';
SET LOCAL statement_timeout = '120s';

DROP INDEX IF EXISTS mecontrola.dictionary_term_normalized_idx;
DROP INDEX IF EXISTS mecontrola.dictionary_kind_term_normalized_idx;

CREATE INDEX dictionary_term_normalized_idx
    ON mecontrola.category_dictionary (term_normalized COLLATE "pt-BR-x-icu")
    WHERE deprecated_at IS NULL;

CREATE INDEX dictionary_kind_term_normalized_idx
    ON mecontrola.category_dictionary (kind, term_normalized COLLATE "pt-BR-x-icu")
    WHERE deprecated_at IS NULL;

CREATE OR REPLACE FUNCTION mecontrola.categories_parent_kind_change_blocks_children()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
    child_count INT;
BEGIN
    IF NEW.kind = OLD.kind THEN
        RETURN NEW;
    END IF;
    SELECT count(*) INTO child_count FROM mecontrola.categories WHERE parent_id = NEW.id;
    IF child_count > 0 THEN
        RAISE EXCEPTION 'categories_parent_kind_change_blocks_children: cannot change kind of parent % with % active children', NEW.id, child_count;
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER categories_parent_kind_change_blocks_children_trg
    BEFORE UPDATE OF kind ON mecontrola.categories
    FOR EACH ROW
    EXECUTE FUNCTION mecontrola.categories_parent_kind_change_blocks_children();
