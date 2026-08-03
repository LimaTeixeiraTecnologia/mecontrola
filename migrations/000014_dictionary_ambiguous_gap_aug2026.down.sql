SET LOCAL lock_timeout    = '5s';
SET LOCAL statement_timeout = '120s';

DELETE FROM mecontrola.category_dictionary
WHERE id IN (
    'a1b00001-0000-5007-0000-00000000011f',
    'a1b00001-0000-5007-0000-000000000120',
    'a1b00001-0000-5007-0000-000000000121',
    'a1b00001-0000-5007-0000-000000000122',
    'a1b00001-0000-5007-0000-000000000123',
    'a1b00001-0000-5007-0000-000000000124',
    'a1b00001-0000-5007-0000-000000000125'
);

UPDATE mecontrola.category_editorial_version SET version = version + 1, updated_at = now();
