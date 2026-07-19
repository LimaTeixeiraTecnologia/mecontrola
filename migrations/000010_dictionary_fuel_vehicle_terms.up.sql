SET LOCAL lock_timeout    = '5s';
SET LOCAL statement_timeout = '120s';

INSERT INTO mecontrola.category_dictionary (id, category_id, kind, term, signal_type, confidence, is_ambiguous) VALUES
    ('a1b00001-0000-5007-0000-000000000067', 'cb13d50d-43cb-553c-99cd-8851889d7f6e', 'expense', 'abastecimento', 'alias', 'high', false),
    ('a1b00001-0000-5007-0000-000000000068', 'cb13d50d-43cb-553c-99cd-8851889d7f6e', 'expense', 'abasteci', 'alias', 'high', false),
    ('a1b00001-0000-5007-0000-000000000069', 'cb13d50d-43cb-553c-99cd-8851889d7f6e', 'expense', 'posto de gasolina', 'alias', 'high', false)
ON CONFLICT (id) DO UPDATE SET
    category_id = EXCLUDED.category_id,
    kind = EXCLUDED.kind,
    term = EXCLUDED.term,
    signal_type = EXCLUDED.signal_type,
    confidence = EXCLUDED.confidence,
    is_ambiguous = EXCLUDED.is_ambiguous,
    deprecated_at = NULL;

UPDATE mecontrola.category_dictionary
SET deprecated_at = now()
WHERE id = 'ef1a26ec-e12d-5b3c-b7ba-3634bb89647c'
  AND category_id = 'ef1a26ec-e12d-5b3c-b7ba-3634bb89647c'
  AND term_normalized = 'veiculo'
  AND deprecated_at IS NULL;

UPDATE mecontrola.category_editorial_version SET version = version + 1, updated_at = now();
