SET LOCAL lock_timeout    = '5s';
SET LOCAL statement_timeout = '120s';

INSERT INTO mecontrola.category_dictionary (id, category_id, kind, term, signal_type, confidence, is_ambiguous) VALUES
    ('a1b00001-0000-5007-0000-000000000110', 'c0e10d9f-b0fe-59e7-8fb9-22a3bebd4784', 'expense', 'busterfit',  'merchant', 'high',   false),
    ('a1b00001-0000-5007-0000-000000000111', 'c0e10d9f-b0fe-59e7-8fb9-22a3bebd4784', 'expense', 'smartfit',   'merchant', 'high',   false),
    ('a1b00001-0000-5007-0000-000000000112', 'c0e10d9f-b0fe-59e7-8fb9-22a3bebd4784', 'expense', 'bluefit',    'merchant', 'high',   false),
    ('a1b00001-0000-5007-0000-000000000113', 'c0e10d9f-b0fe-59e7-8fb9-22a3bebd4784', 'expense', 'selfit',     'merchant', 'high',   false),
    ('a1b00001-0000-5007-0000-000000000114', 'c0e10d9f-b0fe-59e7-8fb9-22a3bebd4784', 'expense', 'bodytech',   'merchant', 'high',   false),
    ('a1b00001-0000-5007-0000-000000000115', 'c0e10d9f-b0fe-59e7-8fb9-22a3bebd4784', 'expense', 'bioritmo',   'merchant', 'high',   false),
    ('a1b00001-0000-5007-0000-000000000116', '178d590e-bc16-5df3-a7c8-ec7c193896d5', 'expense', 'openai',     'merchant', 'high',   false),
    ('a1b00001-0000-5007-0000-000000000117', '178d590e-bc16-5df3-a7c8-ec7c193896d5', 'expense', 'open.ai',    'merchant', 'high',   false),
    ('a1b00001-0000-5007-0000-000000000118', '178d590e-bc16-5df3-a7c8-ec7c193896d5', 'expense', 'chatgpt',    'merchant', 'high',   false),
    ('a1b00001-0000-5007-0000-000000000119', '9dc2ed94-0ea2-5b72-a948-850670f2bee7', 'expense', 'sem parar',  'merchant', 'high',   false),
    ('a1b00001-0000-5007-0000-00000000011a', '9dc2ed94-0ea2-5b72-a948-850670f2bee7', 'expense', 'semparar',   'merchant', 'high',   false),
    ('a1b00001-0000-5007-0000-00000000011b', '9dc2ed94-0ea2-5b72-a948-850670f2bee7', 'expense', 'conectcar',  'merchant', 'high',   false),
    ('a1b00001-0000-5007-0000-00000000011c', '9dc2ed94-0ea2-5b72-a948-850670f2bee7', 'expense', 'veloe',      'merchant', 'high',   false),
    ('a1b00001-0000-5007-0000-00000000011d', '9dc2ed94-0ea2-5b72-a948-850670f2bee7', 'expense', 'move mais',  'merchant', 'high',   false),
    ('a1b00001-0000-5007-0000-00000000011e', '9dc2ed94-0ea2-5b72-a948-850670f2bee7', 'expense', 'taggy',      'merchant', 'medium', true)
ON CONFLICT (id) DO UPDATE SET
    category_id = EXCLUDED.category_id,
    kind = EXCLUDED.kind,
    term = EXCLUDED.term,
    signal_type = EXCLUDED.signal_type,
    confidence = EXCLUDED.confidence,
    is_ambiguous = EXCLUDED.is_ambiguous,
    deprecated_at = NULL;

UPDATE mecontrola.category_editorial_version SET version = version + 1, updated_at = now();
