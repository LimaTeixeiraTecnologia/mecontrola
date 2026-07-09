SET LOCAL lock_timeout    = '5s';
SET LOCAL statement_timeout = '120s';

DELETE FROM mecontrola.category_dictionary
WHERE id IN (
    '1382e2c5-db89-5abf-9793-93fb89053937',
    '1247c8de-fec5-553c-85c6-6a28b802270b',
    '249b3871-812f-53fe-bb68-eeee80d45041',
    'cbd57f6a-b068-598b-8da8-e341f765b740'
);

DELETE FROM mecontrola.categories
WHERE id = 'a1742a1d-85ef-5f94-af85-940e27e32178';

UPDATE mecontrola.category_editorial_version SET version = version - 1, updated_at = now();
