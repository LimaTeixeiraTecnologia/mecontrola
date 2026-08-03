SET LOCAL lock_timeout    = '5s';
SET LOCAL statement_timeout = '120s';

INSERT INTO mecontrola.category_dictionary (id, category_id, kind, term, signal_type, confidence, is_ambiguous) VALUES
    ('a1b00001-0000-5007-0000-00000000011f', '97fa4b86-d43c-5ad5-a99b-c88c8427fb30', 'expense', 'whey',          'segment', 'medium', false),
    ('a1b00001-0000-5007-0000-000000000120', '97fa4b86-d43c-5ad5-a99b-c88c8427fb30', 'expense', 'energetico',    'segment', 'medium', false),
    ('a1b00001-0000-5007-0000-000000000121', '3ca95dd5-c630-5c03-bd47-071777bde81c', 'expense', 'nasoar',        'alias',   'high',   false),
    ('a1b00001-0000-5007-0000-000000000122', 'bf2fcca0-09c3-5dcb-a61a-87eed2860c04', 'expense', 'lavagem',       'segment', 'medium', false),
    ('a1b00001-0000-5007-0000-000000000123', 'a371851d-56cb-551d-addb-022575b8d6e9', 'expense', 'churrasquinho', 'segment', 'medium', false),
    ('a1b00001-0000-5007-0000-000000000124', 'a371851d-56cb-551d-addb-022575b8d6e9', 'expense', 'doces',         'segment', 'medium', false),
    ('a1b00001-0000-5007-0000-000000000125', '0b549268-cbaf-5531-af54-ab47e14a072a', 'expense', 'lacake',        'merchant', 'high',  false)
ON CONFLICT (id) DO UPDATE SET
    category_id = EXCLUDED.category_id,
    kind = EXCLUDED.kind,
    term = EXCLUDED.term,
    signal_type = EXCLUDED.signal_type,
    confidence = EXCLUDED.confidence,
    is_ambiguous = EXCLUDED.is_ambiguous,
    deprecated_at = NULL;

UPDATE mecontrola.category_editorial_version SET version = version + 1, updated_at = now();
