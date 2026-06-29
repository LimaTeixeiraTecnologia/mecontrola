DO $$
DECLARE
    r RECORD;
BEGIN
    FOR r IN
        SELECT tablename
        FROM pg_tables
        WHERE schemaname = 'mecontrola'
          AND tablename <> 'schema_migrations'
    LOOP
        EXECUTE 'DROP TABLE IF EXISTS mecontrola.' || quote_ident(r.tablename) || ' CASCADE';
    END LOOP;
END $$;

DROP FUNCTION IF EXISTS mecontrola.categories_parent_kind_change_blocks_children() CASCADE;
DROP FUNCTION IF EXISTS mecontrola.categories_parent_same_kind() CASCADE;
DROP FUNCTION IF EXISTS mecontrola.immutable_unaccent(text) CASCADE;

DROP EXTENSION IF EXISTS pg_trgm;
DROP EXTENSION IF EXISTS unaccent;
DROP EXTENSION IF EXISTS pgcrypto;
