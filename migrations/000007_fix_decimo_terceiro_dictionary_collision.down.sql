SET LOCAL lock_timeout    = '5s';
SET LOCAL statement_timeout = '120s';

UPDATE mecontrola.category_dictionary
SET signal_type = 'canonical_name'
WHERE id = '1382e2c5-db89-5abf-9793-93fb89053937'
  AND category_id = 'a1742a1d-85ef-5f94-af85-940e27e32178'
  AND term_normalized = 'salario';

DELETE FROM mecontrola.category_dictionary
WHERE id IN (
    'a8c90968-385e-501b-aff2-f393de0e1299',
    'e3e95984-da6d-5678-9ab5-f92de3e34dfc',
    '88aed13a-f390-5d0f-9804-cc840ede53cd',
    'fea79d03-8aed-5c5c-bcc2-c0e15e7d6cae',
    '1bf7dcd2-90a5-50e0-9b28-d34e261bc6a9'
);

UPDATE mecontrola.category_editorial_version SET version = version - 1, updated_at = now();
