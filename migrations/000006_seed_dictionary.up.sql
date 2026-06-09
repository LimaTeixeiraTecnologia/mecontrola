SET LOCAL lock_timeout    = '5s';
SET LOCAL statement_timeout = '120s';

DO $$
DECLARE
    v_version BIGINT;
BEGIN
    SELECT version INTO v_version FROM mecontrola.category_editorial_version;
    IF v_version IS NULL THEN
        RAISE EXCEPTION 'category_editorial_version não inicializada';
    END IF;
END $$;

INSERT INTO mecontrola.category_dictionary (id, category_id, kind, term, signal_type, confidence, is_ambiguous) VALUES
    -- Custo Fixo - canonical names (IDs são os mesmos das categorias)
    ('c2fda6a3-c329-52c8-81ea-771b6ea4f365', 'c2fda6a3-c329-52c8-81ea-771b6ea4f365', 'expense', 'aluguel', 'canonical_name', 'high', false),
    ('f9d9e5b6-1437-5204-bd64-2bd7d43583a8', 'f9d9e5b6-1437-5204-bd64-2bd7d43583a8', 'expense', 'financiamento-imobiliario', 'canonical_name', 'high', false),
    ('d0b1fa13-d19f-51b9-afc7-82bf83accf79', 'd0b1fa13-d19f-51b9-afc7-82bf83accf79', 'expense', 'condominio', 'canonical_name', 'high', false),
    ('80a870e9-831f-5e85-b95f-0afe2f8d372a', '80a870e9-831f-5e85-b95f-0afe2f8d372a', 'expense', 'iptu', 'canonical_name', 'high', false),
    ('36916fab-eacc-50a3-8a53-93671c335952', '36916fab-eacc-50a3-8a53-93671c335952', 'expense', 'energia', 'canonical_name', 'high', false),
    ('fa93273d-e2d9-54ed-a6aa-53b5b1830867', 'fa93273d-e2d9-54ed-a6aa-53b5b1830867', 'expense', 'agua', 'canonical_name', 'high', false),
    ('4e6f8b6b-8ffb-5d38-8ac9-68464679a544', '4e6f8b6b-8ffb-5d38-8ac9-68464679a544', 'expense', 'gas', 'canonical_name', 'high', false),
    ('9391ac38-ec2c-55d0-afc8-8c0940678814', '9391ac38-ec2c-55d0-afc8-8c0940678814', 'expense', 'internet', 'canonical_name', 'high', false),
    ('7319ba14-0dc7-56ff-ac5c-96024e15ec02', '7319ba14-0dc7-56ff-ac5c-96024e15ec02', 'expense', 'telefonia', 'canonical_name', 'high', false),
    ('97fa4b86-d43c-5ad5-a99b-c88c8427fb30', '97fa4b86-d43c-5ad5-a99b-c88c8427fb30', 'expense', 'supermercado', 'canonical_name', 'high', false),
    ('0c004f2d-ad42-5855-a408-f695906cd48c', '0c004f2d-ad42-5855-a408-f695906cd48c', 'expense', 'feira-e-hortifruti', 'canonical_name', 'high', false),
    ('75af9f6b-78e4-5ef3-b6ca-b84a37f8901c', '75af9f6b-78e4-5ef3-b6ca-b84a37f8901c', 'expense', 'acougue', 'canonical_name', 'high', false),
    ('007c090e-7a6d-5645-b751-b93cabb280ed', '007c090e-7a6d-5645-b751-b93cabb280ed', 'expense', 'transporte-publico', 'canonical_name', 'high', false),
    ('c13dcc6e-c37b-521d-a889-8bb02765490f', 'c13dcc6e-c37b-521d-a889-8bb02765490f', 'expense', 'transporte-por-aplicativo-recorrente', 'canonical_name', 'high', false),
    ('c13dcc6e-c37b-521d-a889-8bb02765491a', 'c13dcc6e-c37b-521d-a889-8bb02765490f', 'expense', 'uber', 'merchant', 'medium', true),
    ('c13dcc6e-c37b-521d-a889-8bb02765491b', 'c13dcc6e-c37b-521d-a889-8bb02765490f', 'expense', '99', 'merchant', 'medium', true),
    ('6feeb8fd-8faa-56d1-a0d0-d9d746e45f21', '6feeb8fd-8faa-56d1-a0d0-d9d746e45f21', 'expense', 'transporte-de-lazer', 'canonical_name', 'high', false),
    ('6feeb8fd-8faa-56d1-a0d0-d9d746e45f2a', '6feeb8fd-8faa-56d1-a0d0-d9d746e45f21', 'expense', 'uber', 'merchant', 'medium', true),
    ('6feeb8fd-8faa-56d1-a0d0-d9d746e45f2b', '6feeb8fd-8faa-56d1-a0d0-d9d746e45f21', 'expense', '99', 'merchant', 'medium', true),
    ('cb13d50d-43cb-553c-99cd-8851889d7f6e', 'cb13d50d-43cb-553c-99cd-8851889d7f6e', 'expense', 'combustivel', 'canonical_name', 'high', false),
    ('bf2fcca0-09c3-5dcb-a61a-87eed2860c04', 'bf2fcca0-09c3-5dcb-a61a-87eed2860c04', 'expense', 'manutencao-veicular', 'canonical_name', 'high', false),
    ('311c7b7f-56a3-5b53-ada7-5b85734ba45f', '311c7b7f-56a3-5b53-ada7-5b85734ba45f', 'expense', 'ipva-e-licenciamento', 'canonical_name', 'high', false),
    ('c8f579ea-952b-5e24-beed-ef22fb845a4b', 'c8f579ea-952b-5e24-beed-ef22fb845a4b', 'expense', 'plano-de-saude', 'canonical_name', 'high', false),
    ('1af66343-7305-534f-b8de-47ebcd3d17f1', '1af66343-7305-534f-b8de-47ebcd3d17f1', 'expense', 'plano-odontologico', 'canonical_name', 'high', false),
    ('af5619e0-3683-5b8c-b9fc-0b3ddfbd2075', 'af5619e0-3683-5b8c-b9fc-0b3ddfbd2075', 'expense', 'consultas-e-exames', 'canonical_name', 'high', false),
    ('157b18fe-513e-55fa-969c-c9bd785530d1', '157b18fe-513e-55fa-969c-c9bd785530d1', 'expense', 'medicamentos-continuos', 'canonical_name', 'high', false),
    ('3ca95dd5-c630-5c03-bd47-071777bde81c', '3ca95dd5-c630-5c03-bd47-071777bde81c', 'expense', 'medicamentos-e-farmacia', 'canonical_name', 'high', false),
    ('4ded7fd4-5335-5cf2-aed1-bdcead596000', '4ded7fd4-5335-5cf2-aed1-bdcead596000', 'expense', 'odontologia', 'canonical_name', 'high', false),
    ('a15cba16-23da-504e-a22b-144392ed82bc', 'a15cba16-23da-504e-a22b-144392ed82bc', 'expense', 'terapia-e-saude-mental', 'canonical_name', 'high', false),
    ('cab69263-ac14-5ed1-ab5d-8372487c9ee8', 'cab69263-ac14-5ed1-ab5d-8372487c9ee8', 'expense', 'escola-e-creche', 'canonical_name', 'high', false),
    ('46e492a0-3909-5e0a-bd3e-16bbdf29db8d', '46e492a0-3909-5e0a-bd3e-16bbdf29db8d', 'expense', 'faculdade-e-pos-graduacao', 'canonical_name', 'high', false),
    -- Conhecimento
    ('b3a4824f-e481-59fe-8f9e-0c33a59b5b5f', 'b3a4824f-e481-59fe-8f9e-0c33a59b5b5f', 'expense', 'cursos-e-treinamentos', 'canonical_name', 'high', false),
    ('01b51d39-347e-560c-ac07-d0a700f0c24f', '01b51d39-347e-560c-ac07-d0a700f0c24f', 'expense', 'plataformas-de-ensino', 'canonical_name', 'high', false),
    ('bac52783-54ca-5401-92da-5afa29fc05d4', 'bac52783-54ca-5401-92da-5afa29fc05d4', 'expense', 'livros-e-ebooks', 'canonical_name', 'high', false),
    ('654552ab-829d-5b4d-b0ec-4cb1463454d7', '654552ab-829d-5b4d-b0ec-4cb1463454d7', 'expense', 'certificacoes', 'canonical_name', 'high', false),
    ('3c5e9972-7f59-5f6b-aea4-ace59985cce0', '3c5e9972-7f59-5f6b-aea4-ace59985cce0', 'expense', 'congressos-e-workshops', 'canonical_name', 'high', false),
    ('fec9aed9-2699-538e-bbae-eb4bcdfb1ce3', 'fec9aed9-2699-538e-bbae-eb4bcdfb1ce3', 'expense', 'idiomas', 'canonical_name', 'high', false),
    -- Prazeres
    ('ddbb0dc7-8b85-5177-8cfc-3bb2aed6c75c', 'ddbb0dc7-8b85-5177-8cfc-3bb2aed6c75c', 'expense', 'delivery', 'canonical_name', 'high', false),
    ('d539672d-961f-5553-b807-0e0156a63163', 'd539672d-961f-5553-b807-0e0156a63163', 'expense', 'restaurantes', 'canonical_name', 'high', false),
    ('a371851d-56cb-551d-addb-022575b8d6e9', 'a371851d-56cb-551d-addb-022575b8d6e9', 'expense', 'bares-e-lanches', 'canonical_name', 'high', false),
    ('a20b4072-23b7-53e8-8d03-8146e0473218', 'a20b4072-23b7-53e8-8d03-8146e0473218', 'expense', 'cafeterias', 'canonical_name', 'high', false),
    ('85e56497-2e31-55d3-9516-376e61860708', '85e56497-2e31-55d3-9516-376e61860708', 'expense', 'streaming-de-video', 'canonical_name', 'high', false),
    ('8580a31d-041d-5fa4-b86e-af90108af0cb', '8580a31d-041d-5fa4-b86e-af90108af0cb', 'expense', 'musica-e-audio', 'canonical_name', 'high', false),
    ('514c00a0-ca41-5798-85d0-39992fbc223c', '514c00a0-ca41-5798-85d0-39992fbc223c', 'expense', 'games-e-assinaturas-de-jogos', 'canonical_name', 'high', false),
    ('5190df3d-8e6d-59bc-9e5b-d7e85e45154c', '5190df3d-8e6d-59bc-9e5b-d7e85e45154c', 'expense', 'cinema-e-teatro', 'canonical_name', 'high', false),
    ('09073cdd-4d58-5073-ae16-53ba2c3a4209', '09073cdd-4d58-5073-ae16-53ba2c3a4209', 'expense', 'shows-e-eventos', 'canonical_name', 'high', false),
    ('aed45dcf-8fbe-5828-8fb6-87babd271d6c', 'aed45dcf-8fbe-5828-8fb6-87babd271d6c', 'expense', 'passeios-e-parques', 'canonical_name', 'high', false),
    ('0134668f-785b-5ac1-bcf5-e6c4f566de64', '0134668f-785b-5ac1-bcf5-e6c4f566de64', 'expense', 'viagens-de-lazer', 'canonical_name', 'high', false),
    ('7a69762f-6016-593a-9e62-f56f508ec9e1', '7a69762f-6016-593a-9e62-f56f508ec9e1', 'expense', 'hospedagem-de-lazer', 'canonical_name', 'high', false),
    ('14416063-f271-53e2-8a58-6682461ec532', '14416063-f271-53e2-8a58-6682461ec532', 'expense', 'roupas-e-calcados', 'canonical_name', 'high', false),
    ('f9656739-8d1c-5675-8eaf-63a057137307', 'f9656739-8d1c-5675-8eaf-63a057137307', 'expense', 'beleza-e-estetica', 'canonical_name', 'high', false),
    ('671873dc-f403-5315-877c-d6d46d0f5a8f', '671873dc-f403-5315-877c-d6d46d0f5a8f', 'expense', 'hobbies', 'canonical_name', 'high', false),
    ('c0e10d9f-b0fe-59e7-8fb9-22a3bebd4784', 'c0e10d9f-b0fe-59e7-8fb9-22a3bebd4784', 'expense', 'esportes-e-academia', 'canonical_name', 'high', false),
    -- Metas
    ('3ff5e6b5-b958-5848-9092-73eb541598fc', '3ff5e6b5-b958-5848-9092-73eb541598fc', 'expense', 'tecnologia', 'canonical_name', 'high', false),
    ('ef1a26ec-e12d-5b3c-b7ba-3634bb89647c', 'ef1a26ec-e12d-5b3c-b7ba-3634bb89647c', 'expense', 'veiculo', 'canonical_name', 'high', false),
    ('61698c19-7281-5016-8cd3-b3799ddb575c', '61698c19-7281-5016-8cd3-b3799ddb575c', 'expense', 'casa-e-reforma', 'canonical_name', 'high', false),
    ('8a4228f0-bc77-5d24-949d-5a7afa8063dc', '8a4228f0-bc77-5d24-949d-5a7afa8063dc', 'expense', 'viagem-planejada', 'canonical_name', 'high', false),
    ('e91062ea-8bc9-5d30-a317-260faaf14e56', 'e91062ea-8bc9-5d30-a317-260faaf14e56', 'expense', 'familia-e-enxoval', 'canonical_name', 'high', false),
    ('480b8f7d-6dc2-5d62-b154-669818123f65', '480b8f7d-6dc2-5d62-b154-669818123f65', 'expense', 'empreendedorismo', 'canonical_name', 'high', false),
    ('946643a8-9e00-5bad-a860-f74ed74cf246', '946643a8-9e00-5bad-a860-f74ed74cf246', 'expense', 'quitacao-de-dividas', 'canonical_name', 'high', false),
    -- Liberdade Financeira
    ('45c7e533-fb00-50d9-aeb3-71bdb99098bd', '45c7e533-fb00-50d9-aeb3-71bdb99098bd', 'expense', 'reserva-de-emergencia', 'canonical_name', 'high', false),
    ('9103a0e6-366b-5c77-a31d-e3ed58991d14', '9103a0e6-366b-5c77-a31d-e3ed58991d14', 'expense', 'tesouro-direto', 'canonical_name', 'high', false),
    ('1e5b4db2-b186-5524-b955-32553307d81c', '1e5b4db2-b186-5524-b955-32553307d81c', 'expense', 'fundos-imobiliarios', 'canonical_name', 'high', false),
    ('b1ac9b12-0b4d-5791-87d5-6628c9bbfa9a', 'b1ac9b12-0b4d-5791-87d5-6628c9bbfa9a', 'expense', 'previdencia-privada', 'canonical_name', 'high', false),
    ('9747b1c4-f9dd-5565-ad6d-0f3476ebab9e', '9747b1c4-f9dd-5565-ad6d-0f3476ebab9e', 'expense', 'criptoativos', 'canonical_name', 'high', false),
    ('da4f4c4c-864e-577b-9f4d-d7800f7a85ab', 'da4f4c4c-864e-577b-9f4d-d7800f7a85ab', 'expense', 'investimentos-internacionais', 'canonical_name', 'high', false),
    -- Receitas
    ('98455e74-b1f3-5b9c-a8d8-05db0cdb465d', '98455e74-b1f3-5b9c-a8d8-05db0cdb465d', 'income', 'decimo-terceiro', 'canonical_name', 'high', false),
    ('98455e74-b1f3-5b9c-a8d8-05db0cdb465a', '98455e74-b1f3-5b9c-a8d8-05db0cdb465d', 'income', '13 salario', 'alias', 'high', false),
    ('98455e74-b1f3-5b9c-a8d8-05db0cdb465e', '98455e74-b1f3-5b9c-a8d8-05db0cdb465d', 'income', '13º salário', 'alias', 'high', false),
    ('98455e74-b1f3-5b9c-a8d8-05db0cdb465b', '98455e74-b1f3-5b9c-a8d8-05db0cdb465d', 'income', 'decimo terceiro salario', 'alias', 'high', false),
    ('4b61504d-9cc2-579f-b927-d1963bd1e0ca', '4b61504d-9cc2-579f-b927-d1963bd1e0ca', 'income', 'plr-e-bonus', 'canonical_name', 'high', false),
    ('403192d5-5e85-54d3-a4b0-d4029e754c5c', '403192d5-5e85-54d3-a4b0-d4029e754c5c', 'income', 'freelance', 'canonical_name', 'high', false),
    ('c8276187-8320-5be9-9519-8b6d2a4620b2', 'c8276187-8320-5be9-9519-8b6d2a4620b2', 'income', 'rendimentos', 'canonical_name', 'high', false),
    ('5b983987-3b1d-5bd5-80d3-017416c3f0f8', '5b983987-3b1d-5bd5-80d3-017416c3f0f8', 'income', 'dividendos', 'canonical_name', 'high', false),
    ('00b886a7-d221-592e-8068-fa296924b333', '00b886a7-d221-592e-8068-fa296924b333', 'income', 'aluguel-residencial-recebido', 'canonical_name', 'high', false),
    ('7a17fe1c-900c-57d8-a1dc-22bf9139cf83', '7a17fe1c-900c-57d8-a1dc-22bf9139cf83', 'income', 'restituicao-de-ir', 'canonical_name', 'high', false),
    ('3791836d-bc96-57ae-87b4-fae12c1c111b', '3791836d-bc96-57ae-87b4-fae12c1c111b', 'income', 'cashback', 'canonical_name', 'high', false),
    ('06373332-7fb8-52b6-940e-f0c5699c6114', '06373332-7fb8-52b6-940e-f0c5699c6114', 'income', 'presentes-em-dinheiro', 'canonical_name', 'high', false),
    ('6650a195-013b-5808-8845-22a0657da9ba', '6650a195-013b-5808-8845-22a0657da9ba', 'income', 'vendas-diversas', 'canonical_name', 'high', false)
ON CONFLICT (id) DO UPDATE SET
    category_id = EXCLUDED.category_id,
    kind = EXCLUDED.kind,
    term = EXCLUDED.term,
    signal_type = EXCLUDED.signal_type,
    confidence = EXCLUDED.confidence,
    is_ambiguous = EXCLUDED.is_ambiguous,
    deprecated_at = NULL
WHERE mecontrola.category_dictionary.deprecated_at IS NOT NULL;

UPDATE mecontrola.category_editorial_version
SET version = version + 1, updated_at = now();
