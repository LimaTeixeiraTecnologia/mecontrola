SET LOCAL lock_timeout    = '5s';
SET LOCAL statement_timeout = '120s';

DELETE FROM mecontrola.category_dictionary
WHERE id IN (
    'a1b00001-0000-5007-0000-000000000067',
    'a1b00001-0000-5007-0000-000000000068',
    'a1b00001-0000-5007-0000-000000000069'
);

UPDATE mecontrola.category_dictionary
SET deprecated_at = NULL
WHERE id = 'ef1a26ec-e12d-5b3c-b7ba-3634bb89647c'
  AND category_id = 'ef1a26ec-e12d-5b3c-b7ba-3634bb89647c'
  AND term_normalized = 'veiculo';

UPDATE mecontrola.category_editorial_version SET version = version + 1, updated_at = now();
