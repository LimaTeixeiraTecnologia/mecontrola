SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '120s';

INSERT INTO mecontrola.categories (id, slug, name, kind, parent_id, allocation_type) VALUES
('66cb85a0-3266-5900-b8e3-13cdcd00ab62', 'custo-fixo', 'Custo Fixo', 'expense', NULL, 'consumption'),
('8314f021-ee9c-53b4-872f-449ac618da50', 'conhecimento', 'Conhecimento', 'expense', NULL, 'consumption'),
('ac535261-4060-56ef-b2e8-57c8cc7032d1', 'prazeres', 'Prazeres', 'expense', NULL, 'consumption'),
('f133508e-7dc3-58a3-96db-199d8fbd2987', 'metas', 'Metas', 'expense', NULL, 'asset_allocation'),
('35ced21e-b436-5cea-afb9-ffd43f98a124', 'liberdade-financeira', 'Liberdade Financeira', 'expense', NULL, 'asset_allocation')
ON CONFLICT (kind, slug) DO NOTHING;

INSERT INTO mecontrola.categories (id, slug, name, kind, parent_id, allocation_type) VALUES
('c2fda6a3-c329-52c8-81ea-771b6ea4f365', 'aluguel', 'Aluguel', 'expense', '66cb85a0-3266-5900-b8e3-13cdcd00ab62', 'consumption'),
('f9d9e5b6-1437-5204-bd64-2bd7d43583a8', 'financiamento-imobiliario', 'Financiamento Imobiliário', 'expense', '66cb85a0-3266-5900-b8e3-13cdcd00ab62', 'consumption'),
('d0b1fa13-d19f-51b9-afc7-82bf83accf79', 'condominio', 'Condomínio', 'expense', '66cb85a0-3266-5900-b8e3-13cdcd00ab62', 'consumption'),
('80a870e9-831f-5e85-b95f-0afe2f8d372a', 'iptu', 'IPTU', 'expense', '66cb85a0-3266-5900-b8e3-13cdcd00ab62', 'consumption'),
('8eaa0160-80cd-5c14-a361-d98068aab2cd', 'taxas-residenciais', 'Taxas Residenciais', 'expense', '66cb85a0-3266-5900-b8e3-13cdcd00ab62', 'consumption'),
('0abec125-fa91-5ac6-a82e-3686533c4b8d', 'seguro-residencial', 'Seguro Residencial', 'expense', '66cb85a0-3266-5900-b8e3-13cdcd00ab62', 'consumption'),
('36916fab-eacc-50a3-8a53-93671c335952', 'energia', 'Energia', 'expense', '66cb85a0-3266-5900-b8e3-13cdcd00ab62', 'consumption'),
('fa93273d-e2d9-54ed-a6aa-53b5b1830867', 'agua', 'Água', 'expense', '66cb85a0-3266-5900-b8e3-13cdcd00ab62', 'consumption'),
('4e6f8b6b-8ffb-5d38-8ac9-68464679a544', 'gas', 'Gás', 'expense', '66cb85a0-3266-5900-b8e3-13cdcd00ab62', 'consumption'),
('9391ac38-ec2c-55d0-afc8-8c0940678814', 'internet', 'Internet', 'expense', '66cb85a0-3266-5900-b8e3-13cdcd00ab62', 'consumption'),
('7319ba14-0dc7-56ff-ac5c-96024e15ec02', 'telefonia', 'Telefonia', 'expense', '66cb85a0-3266-5900-b8e3-13cdcd00ab62', 'consumption'),
('2e90fdd3-1008-5423-8215-5db1880fa60b', 'tv-por-assinatura', 'TV por Assinatura', 'expense', '66cb85a0-3266-5900-b8e3-13cdcd00ab62', 'consumption'),
('97fa4b86-d43c-5ad5-a99b-c88c8427fb30', 'supermercado', 'Supermercado', 'expense', '66cb85a0-3266-5900-b8e3-13cdcd00ab62', 'consumption'),
('0c004f2d-ad42-5855-a408-f695906cd48c', 'feira-e-hortifruti', 'Feira e Hortifruti', 'expense', '66cb85a0-3266-5900-b8e3-13cdcd00ab62', 'consumption'),
('75af9f6b-78e4-5ef3-b6ca-b84a37f8901c', 'acougue', 'Açougue', 'expense', '66cb85a0-3266-5900-b8e3-13cdcd00ab62', 'consumption'),
('0b549268-cbaf-5531-af54-ab47e14a072a', 'padaria', 'Padaria', 'expense', '66cb85a0-3266-5900-b8e3-13cdcd00ab62', 'consumption'),
('007c090e-7a6d-5645-b751-b93cabb280ed', 'transporte-publico', 'Transporte Público', 'expense', '66cb85a0-3266-5900-b8e3-13cdcd00ab62', 'consumption'),
('c13dcc6e-c37b-521d-a889-8bb02765490f', 'transporte-por-aplicativo-recorrente', 'Transporte por Aplicativo Recorrente', 'expense', '66cb85a0-3266-5900-b8e3-13cdcd00ab62', 'consumption'),
('cb13d50d-43cb-553c-99cd-8851889d7f6e', 'combustivel', 'Combustível', 'expense', '66cb85a0-3266-5900-b8e3-13cdcd00ab62', 'consumption'),
('7e647851-411c-52d7-a0f2-13535469d918', 'estacionamento-mensal', 'Estacionamento Mensal', 'expense', '66cb85a0-3266-5900-b8e3-13cdcd00ab62', 'consumption'),
('9dc2ed94-0ea2-5b72-a948-850670f2bee7', 'pedagio', 'Pedágio', 'expense', '66cb85a0-3266-5900-b8e3-13cdcd00ab62', 'consumption'),
('bf2fcca0-09c3-5dcb-a61a-87eed2860c04', 'manutencao-veicular', 'Manutenção Veicular', 'expense', '66cb85a0-3266-5900-b8e3-13cdcd00ab62', 'consumption'),
('311c7b7f-56a3-5b53-ada7-5b85734ba45f', 'ipva-e-licenciamento', 'IPVA e Licenciamento', 'expense', '66cb85a0-3266-5900-b8e3-13cdcd00ab62', 'consumption'),
('75e7909d-d816-5609-ac03-89d1c6eb31f5', 'seguro-veicular', 'Seguro Veicular', 'expense', '66cb85a0-3266-5900-b8e3-13cdcd00ab62', 'consumption'),
('c8f579ea-952b-5e24-beed-ef22fb845a4b', 'plano-de-saude', 'Plano de Saúde', 'expense', '66cb85a0-3266-5900-b8e3-13cdcd00ab62', 'consumption'),
('1af66343-7305-534f-b8de-47ebcd3d17f1', 'plano-odontologico', 'Plano Odontológico', 'expense', '66cb85a0-3266-5900-b8e3-13cdcd00ab62', 'consumption'),
('af5619e0-3683-5b8c-b9fc-0b3ddfbd2075', 'consultas-e-exames', 'Consultas e Exames', 'expense', '66cb85a0-3266-5900-b8e3-13cdcd00ab62', 'consumption'),
('157b18fe-513e-55fa-969c-c9bd785530d1', 'medicamentos-continuos', 'Medicamentos Contínuos', 'expense', '66cb85a0-3266-5900-b8e3-13cdcd00ab62', 'consumption'),
('3ca95dd5-c630-5c03-bd47-071777bde81c', 'medicamentos-e-farmacia', 'Medicamentos e Farmácia', 'expense', '66cb85a0-3266-5900-b8e3-13cdcd00ab62', 'consumption'),
('4ded7fd4-5335-5cf2-aed1-bdcead596000', 'odontologia', 'Odontologia', 'expense', '66cb85a0-3266-5900-b8e3-13cdcd00ab62', 'consumption'),
('a15cba16-23da-504e-a22b-144392ed82bc', 'terapia-e-saude-mental', 'Terapia e Saúde Mental', 'expense', '66cb85a0-3266-5900-b8e3-13cdcd00ab62', 'consumption'),
('cab69263-ac14-5ed1-ab5d-8372487c9ee8', 'escola-e-creche', 'Escola e Creche', 'expense', '66cb85a0-3266-5900-b8e3-13cdcd00ab62', 'consumption'),
('46e492a0-3909-5e0a-bd3e-16bbdf29db8d', 'faculdade-e-pos-graduacao', 'Faculdade e Pós-graduação', 'expense', '66cb85a0-3266-5900-b8e3-13cdcd00ab62', 'consumption'),
('5828e634-94c1-5800-8160-4ecb1eff1a81', 'pensao-alimenticia', 'Pensão Alimentícia', 'expense', '66cb85a0-3266-5900-b8e3-13cdcd00ab62', 'consumption'),
('6a0d56cc-f9d8-5c95-be2a-60f8f69c912c', 'seguros-pessoais', 'Seguros Pessoais', 'expense', '66cb85a0-3266-5900-b8e3-13cdcd00ab62', 'consumption'),
('178d590e-bc16-5df3-a7c8-ec7c193896d5', 'assinaturas-essenciais', 'Assinaturas Essenciais', 'expense', '66cb85a0-3266-5900-b8e3-13cdcd00ab62', 'consumption'),
('347e0488-a4a7-55e8-8882-ae868c9d749d', 'tarifas-bancarias', 'Tarifas Bancárias', 'expense', '66cb85a0-3266-5900-b8e3-13cdcd00ab62', 'consumption'),
('7d56377d-bdd0-5152-9b94-10639bc7f39b', 'impostos-e-tributos', 'Impostos e Tributos', 'expense', '66cb85a0-3266-5900-b8e3-13cdcd00ab62', 'consumption'),
('b29895dd-f1c5-5375-a5d1-082d9e2c3620', 'emprestimos-e-financiamentos', 'Empréstimos e Financiamentos', 'expense', '66cb85a0-3266-5900-b8e3-13cdcd00ab62', 'consumption'),
('5b9a1cba-b400-508c-a615-a419d9b06dcf', 'dividas-e-juros', 'Dívidas e Juros', 'expense', '66cb85a0-3266-5900-b8e3-13cdcd00ab62', 'consumption'),
('3f7c80e0-820c-5766-ba50-826a6d82b8e6', 'manutencao-da-casa', 'Manutenção da Casa', 'expense', '66cb85a0-3266-5900-b8e3-13cdcd00ab62', 'consumption'),
('ca8e4a6c-58ae-5049-8c24-826bd471e896', 'servicos-domesticos', 'Serviços Domésticos', 'expense', '66cb85a0-3266-5900-b8e3-13cdcd00ab62', 'consumption'),
('d4b74050-db29-53e6-bcee-be5c333f8817', 'pets-recorrentes', 'Pets Recorrentes', 'expense', '66cb85a0-3266-5900-b8e3-13cdcd00ab62', 'consumption'),
('d1d7dbba-1e83-596c-a4e5-d520cd06c88a', 'outros-custos-fixos', 'Outros Custos Fixos', 'expense', '66cb85a0-3266-5900-b8e3-13cdcd00ab62', 'consumption')
ON CONFLICT (kind, slug) DO NOTHING;

INSERT INTO mecontrola.categories (id, slug, name, kind, parent_id, allocation_type) VALUES
('b3a4824f-e481-59fe-8f9e-0c33a59b5b5f', 'cursos-e-treinamentos', 'Cursos e Treinamentos', 'expense', '8314f021-ee9c-53b4-872f-449ac618da50', 'consumption'),
('01b51d39-347e-560c-ac07-d0a700f0c24f', 'plataformas-de-ensino', 'Plataformas de Ensino', 'expense', '8314f021-ee9c-53b4-872f-449ac618da50', 'consumption'),
('bac52783-54ca-5401-92da-5afa29fc05d4', 'livros-e-ebooks', 'Livros e E-books', 'expense', '8314f021-ee9c-53b4-872f-449ac618da50', 'consumption'),
('6f70f7d5-d319-5a97-a319-c864e7567285', 'material-de-estudo', 'Material de Estudo', 'expense', '8314f021-ee9c-53b4-872f-449ac618da50', 'consumption'),
('654552ab-829d-5b4d-b0ec-4cb1463454d7', 'certificacoes', 'Certificações', 'expense', '8314f021-ee9c-53b4-872f-449ac618da50', 'consumption'),
('3c5e9972-7f59-5f6b-aea4-ace59985cce0', 'congressos-e-workshops', 'Congressos e Workshops', 'expense', '8314f021-ee9c-53b4-872f-449ac618da50', 'consumption'),
('fec9aed9-2699-538e-bbae-eb4bcdfb1ce3', 'idiomas', 'Idiomas', 'expense', '8314f021-ee9c-53b4-872f-449ac618da50', 'consumption'),
('8d114d26-b1a4-5a5f-8995-194c088c7b3f', 'mentoria-e-coaching', 'Mentoria e Coaching', 'expense', '8314f021-ee9c-53b4-872f-449ac618da50', 'consumption'),
('ce2850ad-8d51-5224-b9a4-d884361e4639', 'aulas-particulares', 'Aulas Particulares', 'expense', '8314f021-ee9c-53b4-872f-449ac618da50', 'consumption'),
('4850d076-7dea-5b73-8d32-fff55765dd2f', 'software-e-ferramentas-de-estudo', 'Software e Ferramentas de Estudo', 'expense', '8314f021-ee9c-53b4-872f-449ac618da50', 'consumption'),
('ce233b45-2c19-536e-92bd-6b43958c9363', 'outros-conhecimentos', 'Outros Conhecimentos', 'expense', '8314f021-ee9c-53b4-872f-449ac618da50', 'consumption')
ON CONFLICT (kind, slug) DO NOTHING;

INSERT INTO mecontrola.categories (id, slug, name, kind, parent_id, allocation_type) VALUES
('ddbb0dc7-8b85-5177-8cfc-3bb2aed6c75c', 'delivery', 'Delivery', 'expense', 'ac535261-4060-56ef-b2e8-57c8cc7032d1', 'consumption'),
('d539672d-961f-5553-b807-0e0156a63163', 'restaurantes', 'Restaurantes', 'expense', 'ac535261-4060-56ef-b2e8-57c8cc7032d1', 'consumption'),
('a371851d-56cb-551d-addb-022575b8d6e9', 'bares-e-lanches', 'Bares e Lanches', 'expense', 'ac535261-4060-56ef-b2e8-57c8cc7032d1', 'consumption'),
('a20b4072-23b7-53e8-8d03-8146e0473218', 'cafeterias', 'Cafeterias', 'expense', 'ac535261-4060-56ef-b2e8-57c8cc7032d1', 'consumption'),
('85e56497-2e31-55d3-9516-376e61860708', 'streaming-de-video', 'Streaming de Vídeo', 'expense', 'ac535261-4060-56ef-b2e8-57c8cc7032d1', 'consumption'),
('8580a31d-041d-5fa4-b86e-af90108af0cb', 'musica-e-audio', 'Música e Áudio', 'expense', 'ac535261-4060-56ef-b2e8-57c8cc7032d1', 'consumption'),
('514c00a0-ca41-5798-85d0-39992fbc223c', 'games-e-assinaturas-de-jogos', 'Games e Assinaturas de Jogos', 'expense', 'ac535261-4060-56ef-b2e8-57c8cc7032d1', 'consumption'),
('5190df3d-8e6d-59bc-9e5b-d7e85e45154c', 'cinema-e-teatro', 'Cinema e Teatro', 'expense', 'ac535261-4060-56ef-b2e8-57c8cc7032d1', 'consumption'),
('09073cdd-4d58-5073-ae16-53ba2c3a4209', 'shows-e-eventos', 'Shows e Eventos', 'expense', 'ac535261-4060-56ef-b2e8-57c8cc7032d1', 'consumption'),
('aed45dcf-8fbe-5828-8fb6-87babd271d6c', 'passeios-e-parques', 'Passeios e Parques', 'expense', 'ac535261-4060-56ef-b2e8-57c8cc7032d1', 'consumption'),
('6feeb8fd-8faa-56d1-a0d0-d9d746e45f21', 'transporte-de-lazer', 'Transporte de Lazer', 'expense', 'ac535261-4060-56ef-b2e8-57c8cc7032d1', 'consumption'),
('0134668f-785b-5ac1-bcf5-e6c4f566de64', 'viagens-de-lazer', 'Viagens de Lazer', 'expense', 'ac535261-4060-56ef-b2e8-57c8cc7032d1', 'consumption'),
('7a69762f-6016-593a-9e62-f56f508ec9e1', 'hospedagem-de-lazer', 'Hospedagem de Lazer', 'expense', 'ac535261-4060-56ef-b2e8-57c8cc7032d1', 'consumption'),
('a2af4429-8e17-559f-bba4-f790c7732776', 'compras-pessoais', 'Compras Pessoais', 'expense', 'ac535261-4060-56ef-b2e8-57c8cc7032d1', 'consumption'),
('14416063-f271-53e2-8a58-6682461ec532', 'roupas-e-calcados', 'Roupas e Calçados', 'expense', 'ac535261-4060-56ef-b2e8-57c8cc7032d1', 'consumption'),
('f9656739-8d1c-5675-8eaf-63a057137307', 'beleza-e-estetica', 'Beleza e Estética', 'expense', 'ac535261-4060-56ef-b2e8-57c8cc7032d1', 'consumption'),
('671873dc-f403-5315-877c-d6d46d0f5a8f', 'hobbies', 'Hobbies', 'expense', 'ac535261-4060-56ef-b2e8-57c8cc7032d1', 'consumption'),
('c0e10d9f-b0fe-59e7-8fb9-22a3bebd4784', 'esportes-e-academia', 'Esportes e Academia', 'expense', 'ac535261-4060-56ef-b2e8-57c8cc7032d1', 'consumption'),
('09e7cd05-40bf-5100-92e9-439a7baf0c0c', 'presentes', 'Presentes', 'expense', 'ac535261-4060-56ef-b2e8-57c8cc7032d1', 'consumption'),
('481d2d82-a013-5991-8210-0bfcb44af4fa', 'pets-nao-recorrentes', 'Pets Não Recorrentes', 'expense', 'ac535261-4060-56ef-b2e8-57c8cc7032d1', 'consumption'),
('c2470946-ebf3-5baf-86cd-696b11baf497', 'doacoes', 'Doações', 'expense', 'ac535261-4060-56ef-b2e8-57c8cc7032d1', 'consumption'),
('0016763e-655c-571a-90cb-bec5a18d4969', 'outros-prazeres', 'Outros Prazeres', 'expense', 'ac535261-4060-56ef-b2e8-57c8cc7032d1', 'consumption')
ON CONFLICT (kind, slug) DO NOTHING;

INSERT INTO mecontrola.categories (id, slug, name, kind, parent_id, allocation_type) VALUES
('3ff5e6b5-b958-5848-9092-73eb541598fc', 'tecnologia', 'Tecnologia', 'expense', 'f133508e-7dc3-58a3-96db-199d8fbd2987', 'asset_allocation'),
('ef1a26ec-e12d-5b3c-b7ba-3634bb89647c', 'veiculo', 'Veículo', 'expense', 'f133508e-7dc3-58a3-96db-199d8fbd2987', 'asset_allocation'),
('61698c19-7281-5016-8cd3-b3799ddb575c', 'casa-e-reforma', 'Casa e Reforma', 'expense', 'f133508e-7dc3-58a3-96db-199d8fbd2987', 'asset_allocation'),
('8a4228f0-bc77-5d24-949d-5a7afa8063dc', 'viagem-planejada', 'Viagem Planejada', 'expense', 'f133508e-7dc3-58a3-96db-199d8fbd2987', 'asset_allocation'),
('6752f218-cbf9-5108-94e5-6732fdb6a0c6', 'casamento-e-festa', 'Casamento e Festa', 'expense', 'f133508e-7dc3-58a3-96db-199d8fbd2987', 'asset_allocation'),
('e91062ea-8bc9-5d30-a317-260faaf14e56', 'familia-e-enxoval', 'Família e Enxoval', 'expense', 'f133508e-7dc3-58a3-96db-199d8fbd2987', 'asset_allocation'),
('480b8f7d-6dc2-5d62-b154-669818123f65', 'empreendedorismo', 'Empreendedorismo', 'expense', 'f133508e-7dc3-58a3-96db-199d8fbd2987', 'asset_allocation'),
('1c178224-bd1b-51a0-bc6a-a8f12efa54c1', 'educacao-planejada', 'Educação Planejada', 'expense', 'f133508e-7dc3-58a3-96db-199d8fbd2987', 'asset_allocation'),
('568b9200-dae4-512c-a93c-192192d2ee4f', 'saude-planejada', 'Saúde Planejada', 'expense', 'f133508e-7dc3-58a3-96db-199d8fbd2987', 'asset_allocation'),
('946643a8-9e00-5bad-a860-f74ed74cf246', 'quitacao-de-dividas', 'Quitação de Dívidas', 'expense', 'f133508e-7dc3-58a3-96db-199d8fbd2987', 'asset_allocation'),
('ab070648-d71b-5920-a1dd-060f1f542371', 'compra-planejada', 'Compra Planejada', 'expense', 'f133508e-7dc3-58a3-96db-199d8fbd2987', 'asset_allocation'),
('8c1c3dd1-6b38-5b85-a37c-e7c9a769ff94', 'outras-metas', 'Outras Metas', 'expense', 'f133508e-7dc3-58a3-96db-199d8fbd2987', 'asset_allocation')
ON CONFLICT (kind, slug) DO NOTHING;

INSERT INTO mecontrola.categories (id, slug, name, kind, parent_id, allocation_type) VALUES
('45c7e533-fb00-50d9-aeb3-71bdb99098bd', 'reserva-de-emergencia', 'Reserva de Emergência', 'expense', '35ced21e-b436-5cea-afb9-ffd43f98a124', 'asset_allocation'),
('e79c7c54-c8c5-5b9f-9cbb-4bff3c98e429', 'reserva-de-oportunidade', 'Reserva de Oportunidade', 'expense', '35ced21e-b436-5cea-afb9-ffd43f98a124', 'asset_allocation'),
('9103a0e6-366b-5c77-a31d-e3ed58991d14', 'tesouro-direto', 'Tesouro Direto', 'expense', '35ced21e-b436-5cea-afb9-ffd43f98a124', 'asset_allocation'),
('ee26c4d9-ca74-5537-80b9-4d90815b9c06', 'cdb-e-rdb', 'CDB e RDB', 'expense', '35ced21e-b436-5cea-afb9-ffd43f98a124', 'asset_allocation'),
('d35da3b9-65c5-55b8-9915-13354e202644', 'lci-e-lca', 'LCI e LCA', 'expense', '35ced21e-b436-5cea-afb9-ffd43f98a124', 'asset_allocation'),
('b0fba509-bd7d-5f1c-9845-2288bee6c276', 'debentures-e-credito-privado', 'Debêntures e Crédito Privado', 'expense', '35ced21e-b436-5cea-afb9-ffd43f98a124', 'asset_allocation'),
('eb6c008a-2fe5-58bf-a879-a3a0d2ecf6cb', 'fundos-de-renda-fixa', 'Fundos de Renda Fixa', 'expense', '35ced21e-b436-5cea-afb9-ffd43f98a124', 'asset_allocation'),
('e1266272-eb97-5a9f-857d-e6b7b261cf9e', 'acoes', 'Ações', 'expense', '35ced21e-b436-5cea-afb9-ffd43f98a124', 'asset_allocation'),
('e917b351-60d6-53c5-ab5b-a92e663d700b', 'etfs', 'ETFs', 'expense', '35ced21e-b436-5cea-afb9-ffd43f98a124', 'asset_allocation'),
('1e5b4db2-b186-5524-b955-32553307d81c', 'fundos-imobiliarios', 'Fundos Imobiliários', 'expense', '35ced21e-b436-5cea-afb9-ffd43f98a124', 'asset_allocation'),
('abc00654-7b1d-5587-9de8-506710c42da4', 'bdrs', 'BDRs', 'expense', '35ced21e-b436-5cea-afb9-ffd43f98a124', 'asset_allocation'),
('e130a310-4a8d-5f0f-b050-405165e28966', 'fundos-de-investimento', 'Fundos de Investimento', 'expense', '35ced21e-b436-5cea-afb9-ffd43f98a124', 'asset_allocation'),
('b1ac9b12-0b4d-5791-87d5-6628c9bbfa9a', 'previdencia-privada', 'Previdência Privada', 'expense', '35ced21e-b436-5cea-afb9-ffd43f98a124', 'asset_allocation'),
('9747b1c4-f9dd-5565-ad6d-0f3476ebab9e', 'criptoativos', 'Criptoativos', 'expense', '35ced21e-b436-5cea-afb9-ffd43f98a124', 'asset_allocation'),
('da4f4c4c-864e-577b-9f4d-d7800f7a85ab', 'investimentos-internacionais', 'Investimentos Internacionais', 'expense', '35ced21e-b436-5cea-afb9-ffd43f98a124', 'asset_allocation'),
('866793cb-4059-54b0-9ee7-8f539ddebede', 'aportes-em-corretora', 'Aportes em Corretora', 'expense', '35ced21e-b436-5cea-afb9-ffd43f98a124', 'asset_allocation'),
('33191a7c-77d1-5fc8-bb8e-65268997cc65', 'outros-investimentos', 'Outros Investimentos', 'expense', '35ced21e-b436-5cea-afb9-ffd43f98a124', 'asset_allocation')
ON CONFLICT (kind, slug) DO NOTHING;

INSERT INTO mecontrola.categories (id, slug, name, kind, parent_id, allocation_type) VALUES
('86dd34b0-7342-525a-9a30-b1b5a76b109f', 'salario', 'Salário', 'income', NULL, 'consumption'),
('275ef473-b41d-5162-8488-0abf88a5e6f4', 'renda-variavel', 'Renda Variável', 'income', NULL, 'consumption'),
('1c801292-d1a0-56a9-8d05-a28f39f5e6dd', 'investimentos', 'Investimentos', 'income', NULL, 'asset_allocation'),
('6044ffc4-b869-598b-b7e9-8361ab7ee2f6', 'aluguel-recebido', 'Aluguel Recebido', 'income', NULL, 'consumption'),
('c0c8b110-d3de-5e7c-8080-0de827e67332', 'restituicoes-e-cashback', 'Restituições e Cashback', 'income', NULL, 'consumption'),
('be5c5726-10e7-5a39-b149-3ae784121cdd', 'presentes-recebidos', 'Presentes Recebidos', 'income', NULL, 'consumption'),
('8dba4d69-834f-5bdb-8c8c-9f86a9b56858', 'vendas', 'Vendas', 'income', NULL, 'consumption'),
('b01019ae-37b0-5dab-bc2e-3a000843c7bb', 'outras-receitas', 'Outras Receitas', 'income', NULL, 'consumption')
ON CONFLICT (kind, slug) DO NOTHING;

INSERT INTO mecontrola.categories (id, slug, name, kind, parent_id, allocation_type) VALUES
('98455e74-b1f3-5b9c-a8d8-05db0cdb465d', 'decimo-terceiro', 'Décimo Terceiro', 'income', '86dd34b0-7342-525a-9a30-b1b5a76b109f', 'consumption'),
('8f141d28-10c3-5a07-bfdf-4dfd79a049a1', 'ferias', 'Férias', 'income', '86dd34b0-7342-525a-9a30-b1b5a76b109f', 'consumption'),
('4b61504d-9cc2-579f-b927-d1963bd1e0ca', 'plr-e-bonus', 'PLR e Bônus', 'income', '86dd34b0-7342-525a-9a30-b1b5a76b109f', 'consumption'),
('bbc5809c-d567-59cf-80dd-e6f15b93b7e4', 'vale-alimentacao', 'Vale-Alimentação', 'income', '86dd34b0-7342-525a-9a30-b1b5a76b109f', 'consumption'),
('6e896533-af52-5938-bc38-2152ea443af8', 'vale-refeicao', 'Vale-Refeição', 'income', '86dd34b0-7342-525a-9a30-b1b5a76b109f', 'consumption')
ON CONFLICT (kind, slug) DO NOTHING;

INSERT INTO mecontrola.categories (id, slug, name, kind, parent_id, allocation_type) VALUES
('403192d5-5e85-54d3-a4b0-d4029e754c5c', 'freelance', 'Freelance', 'income', '275ef473-b41d-5162-8488-0abf88a5e6f4', 'consumption'),
('dc2303d9-246e-53d6-8448-2adc19993b22', 'trabalho-extra', 'Trabalho Extra', 'income', '275ef473-b41d-5162-8488-0abf88a5e6f4', 'consumption'),
('0d613676-5f32-5412-9408-fde944bed128', 'consultoria', 'Consultoria', 'income', '275ef473-b41d-5162-8488-0abf88a5e6f4', 'consumption')
ON CONFLICT (kind, slug) DO NOTHING;

INSERT INTO mecontrola.categories (id, slug, name, kind, parent_id, allocation_type) VALUES
('c8276187-8320-5be9-9519-8b6d2a4620b2', 'rendimentos', 'Rendimentos', 'income', '1c801292-d1a0-56a9-8d05-a28f39f5e6dd', 'asset_allocation'),
('5b983987-3b1d-5bd5-80d3-017416c3f0f8', 'dividendos', 'Dividendos', 'income', '1c801292-d1a0-56a9-8d05-a28f39f5e6dd', 'asset_allocation'),
('8d812f21-fe17-57e5-a71d-5eb890d29bb6', 'juros', 'Juros', 'income', '1c801292-d1a0-56a9-8d05-a28f39f5e6dd', 'asset_allocation'),
('cac84f1b-70c6-5eb1-81d9-241764043d66', 'resgates', 'Resgates', 'income', '1c801292-d1a0-56a9-8d05-a28f39f5e6dd', 'asset_allocation')
ON CONFLICT (kind, slug) DO NOTHING;

INSERT INTO mecontrola.categories (id, slug, name, kind, parent_id, allocation_type) VALUES
('00b886a7-d221-592e-8068-fa296924b333', 'aluguel-residencial-recebido', 'Aluguel Residencial Recebido', 'income', '6044ffc4-b869-598b-b7e9-8361ab7ee2f6', 'consumption')
ON CONFLICT (kind, slug) DO NOTHING;

INSERT INTO mecontrola.categories (id, slug, name, kind, parent_id, allocation_type) VALUES
('7a17fe1c-900c-57d8-a1dc-22bf9139cf83', 'restituicao-de-ir', 'Restituição de IR', 'income', 'c0c8b110-d3de-5e7c-8080-0de827e67332', 'consumption'),
('3791836d-bc96-57ae-87b4-fae12c1c111b', 'cashback', 'Cashback', 'income', 'c0c8b110-d3de-5e7c-8080-0de827e67332', 'consumption')
ON CONFLICT (kind, slug) DO NOTHING;

INSERT INTO mecontrola.categories (id, slug, name, kind, parent_id, allocation_type) VALUES
('06373332-7fb8-52b6-940e-f0c5699c6114', 'presentes-em-dinheiro', 'Presentes em Dinheiro', 'income', 'be5c5726-10e7-5a39-b149-3ae784121cdd', 'consumption'),
('1722bd29-031a-57d1-b4d8-2626d1971ce3', 'mesada-recebida', 'Mesada Recebida', 'income', 'be5c5726-10e7-5a39-b149-3ae784121cdd', 'consumption')
ON CONFLICT (kind, slug) DO NOTHING;

INSERT INTO mecontrola.categories (id, slug, name, kind, parent_id, allocation_type) VALUES
('6650a195-013b-5808-8845-22a0657da9ba', 'vendas-diversas', 'Vendas Diversas', 'income', '8dba4d69-834f-5bdb-8c8c-9f86a9b56858', 'consumption'),
('52ded4b8-b082-5ec2-90fe-633c934edae7', 'marketplace', 'Marketplace', 'income', '8dba4d69-834f-5bdb-8c8c-9f86a9b56858', 'consumption')
ON CONFLICT (kind, slug) DO NOTHING;

INSERT INTO mecontrola.categories (id, slug, name, kind, parent_id, allocation_type) VALUES
('9d996f66-81f2-5250-bb4d-bd3636e00544', 'outros', 'Outros', 'income', 'b01019ae-37b0-5dab-bc2e-3a000843c7bb', 'consumption')
ON CONFLICT (kind, slug) DO NOTHING;

UPDATE mecontrola.category_editorial_version SET version = 2, updated_at = now()
WHERE version = 1;
