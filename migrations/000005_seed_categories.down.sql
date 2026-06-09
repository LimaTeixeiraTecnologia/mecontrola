SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '120s';

UPDATE mecontrola.category_editorial_version SET version = 1, updated_at = now()
WHERE version = 2;

DELETE FROM mecontrola.category_dictionary WHERE kind IN ('income', 'expense');
DELETE FROM mecontrola.categories WHERE kind = 'income';
DELETE FROM mecontrola.categories WHERE kind = 'expense';
