SET LOCAL lock_timeout    = '5s';
SET LOCAL statement_timeout = '120s';

INSERT INTO mecontrola.category_dictionary (id, category_id, kind, term, signal_type, confidence, is_ambiguous) VALUES
    ('a8c90968-385e-501b-aff2-f393de0e1299', '98455e74-b1f3-5b9c-a8d8-05db0cdb465d', 'income', 'recebi meu 13º salário', 'phrase', 'high', false),
    ('e3e95984-da6d-5678-9ab5-f92de3e34dfc', '98455e74-b1f3-5b9c-a8d8-05db0cdb465d', 'income', 'recebi meu 13 salario', 'phrase', 'high', false),
    ('88aed13a-f390-5d0f-9804-cc840ede53cd', '98455e74-b1f3-5b9c-a8d8-05db0cdb465d', 'income', '13o salario', 'alias', 'high', false),
    ('fea79d03-8aed-5c5c-bcc2-c0e15e7d6cae', '98455e74-b1f3-5b9c-a8d8-05db0cdb465d', 'income', 'recebi 13º salário', 'phrase', 'high', false),
    ('1bf7dcd2-90a5-50e0-9b28-d34e261bc6a9', '98455e74-b1f3-5b9c-a8d8-05db0cdb465d', 'income', 'recebi decimo terceiro', 'phrase', 'high', false)
ON CONFLICT (id) DO UPDATE SET
    category_id = EXCLUDED.category_id,
    kind = EXCLUDED.kind,
    term = EXCLUDED.term,
    signal_type = EXCLUDED.signal_type,
    confidence = EXCLUDED.confidence,
    is_ambiguous = EXCLUDED.is_ambiguous,
    deprecated_at = NULL;

UPDATE mecontrola.category_dictionary
SET signal_type = 'alias'
WHERE id = '1382e2c5-db89-5abf-9793-93fb89053937'
  AND category_id = 'a1742a1d-85ef-5f94-af85-940e27e32178'
  AND term_normalized = 'salario';

UPDATE mecontrola.category_editorial_version SET version = version + 1, updated_at = now();
