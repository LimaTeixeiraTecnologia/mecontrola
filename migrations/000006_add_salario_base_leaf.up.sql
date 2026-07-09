SET LOCAL lock_timeout    = '5s';
SET LOCAL statement_timeout = '120s';

INSERT INTO mecontrola.categories (id, slug, name, kind, parent_id, allocation_type) VALUES
('a1742a1d-85ef-5f94-af85-940e27e32178', 'salario-base', 'Salário', 'income', '86dd34b0-7342-525a-9a30-b1b5a76b109f', 'consumption')
ON CONFLICT (kind, slug) DO NOTHING;

INSERT INTO mecontrola.category_dictionary (id, category_id, kind, term, signal_type, confidence, is_ambiguous) VALUES
    ('1382e2c5-db89-5abf-9793-93fb89053937', 'a1742a1d-85ef-5f94-af85-940e27e32178', 'income', 'salário', 'canonical_name', 'high', false),
    ('1247c8de-fec5-553c-85c6-6a28b802270b', 'a1742a1d-85ef-5f94-af85-940e27e32178', 'income', 'meu salário', 'phrase', 'high', false),
    ('249b3871-812f-53fe-bb68-eeee80d45041', 'a1742a1d-85ef-5f94-af85-940e27e32178', 'income', 'recebi salário', 'phrase', 'high', false),
    ('cbd57f6a-b068-598b-8da8-e341f765b740', 'a1742a1d-85ef-5f94-af85-940e27e32178', 'income', 'recebi meu salário', 'phrase', 'high', false)
ON CONFLICT (id) DO UPDATE SET
    category_id = EXCLUDED.category_id,
    kind = EXCLUDED.kind,
    term = EXCLUDED.term,
    signal_type = EXCLUDED.signal_type,
    confidence = EXCLUDED.confidence,
    is_ambiguous = EXCLUDED.is_ambiguous,
    deprecated_at = NULL;

UPDATE mecontrola.category_editorial_version SET version = version + 1, updated_at = now();
