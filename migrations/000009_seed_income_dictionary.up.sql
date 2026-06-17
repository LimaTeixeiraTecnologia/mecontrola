INSERT INTO mecontrola.category_dictionary (id, category_id, kind, term, signal_type, confidence, is_ambiguous) VALUES
    ('86dd34b0-7342-525a-9a30-b1b5a76b109f', '86dd34b0-7342-525a-9a30-b1b5a76b109f', 'income', 'salario', 'canonical_name', 'high', false),
    ('275ef473-b41d-5162-8488-0abf88a5e6f4', '275ef473-b41d-5162-8488-0abf88a5e6f4', 'income', 'renda-variavel', 'canonical_name', 'high', false),
    ('1c801292-d1a0-56a9-8d05-a28f39f5e6dd', '1c801292-d1a0-56a9-8d05-a28f39f5e6dd', 'income', 'investimentos', 'canonical_name', 'high', false),
    ('6044ffc4-b869-598b-b7e9-8361ab7ee2f6', '6044ffc4-b869-598b-b7e9-8361ab7ee2f6', 'income', 'aluguel-recebido', 'canonical_name', 'high', false),
    ('c0c8b110-d3de-5e7c-8080-0de827e67332', 'c0c8b110-d3de-5e7c-8080-0de827e67332', 'income', 'restituicoes-e-cashback', 'canonical_name', 'high', false),
    ('be5c5726-10e7-5a39-b149-3ae784121cdd', 'be5c5726-10e7-5a39-b149-3ae784121cdd', 'income', 'presentes-recebidos', 'canonical_name', 'high', false),
    ('8dba4d69-834f-5bdb-8c8c-9f86a9b56858', '8dba4d69-834f-5bdb-8c8c-9f86a9b56858', 'income', 'vendas', 'canonical_name', 'high', false),
    ('b01019ae-37b0-5dab-bc2e-3a000843c7bb', 'b01019ae-37b0-5dab-bc2e-3a000843c7bb', 'income', 'outras-receitas', 'canonical_name', 'high', false),
    ('a9100001-0000-4000-8000-000000000001', '86dd34b0-7342-525a-9a30-b1b5a76b109f', 'income', 'salario mensal', 'alias', 'high', false),
    ('a9100001-0000-4000-8000-000000000002', '86dd34b0-7342-525a-9a30-b1b5a76b109f', 'income', 'meu salario', 'alias', 'high', false),
    ('a9100001-0000-4000-8000-000000000003', '86dd34b0-7342-525a-9a30-b1b5a76b109f', 'income', 'pagamento de salario', 'alias', 'high', false),
    ('a9100001-0000-4000-8000-000000000004', '275ef473-b41d-5162-8488-0abf88a5e6f4', 'income', 'freela', 'alias', 'high', false),
    ('a9100001-0000-4000-8000-000000000005', '275ef473-b41d-5162-8488-0abf88a5e6f4', 'income', 'freelance', 'alias', 'high', false),
    ('a9100001-0000-4000-8000-000000000006', 'b01019ae-37b0-5dab-bc2e-3a000843c7bb', 'income', 'pix recebido', 'alias', 'high', false)
ON CONFLICT (id) DO NOTHING;
