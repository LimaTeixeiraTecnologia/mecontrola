SET LOCAL lock_timeout    = '5s';
SET LOCAL statement_timeout = '120s';

UPDATE mecontrola.category_dictionary
SET deprecated_at = now()
WHERE signal_type = 'canonical_name'
AND deprecated_at IS NULL;

UPDATE mecontrola.category_editorial_version
SET version = GREATEST(1, version - 1), updated_at = now();
