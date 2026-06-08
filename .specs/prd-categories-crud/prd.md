# Documento de Requisitos do Produto (PRD)

<!-- spec-version: 6 -->

<!--
Histórico de versões:
- v1-v3 (2026-06-06): escopo inicial de CRUD personalizável de categorias.
- v4 (2026-06-06): despesas consolidadas nas cinco caixinhas oficiais e adição de dicionário conversacional.
- v5 (2026-06-06): escopo simplificado para catálogo global somente leitura. Removidos CRUD público, personalização, clone, preferências, classificação automática e ações do usuário. O MVP cobre exclusivamente seed, listagem de categorias/subcategorias e consulta conservadora ao dicionário.
- v6 (2026-06-06): consolidação production-ready. Fixados volumetria-alvo, SLO de disponibilidade, ETag/304, normalização via coluna gerada com `unaccent`, UUIDv5 determinístico com slug, dedup de candidatos por subcategoria com precedência editorial, rollback append-only explícito, telemetria sem termo bruto, OpenAPI versionado obrigatório, envelope de erro reusado de billing/identity e cenários canônicos de aceitação.
-->

## Visão Geral

O módulo `internal/categories` fornece a taxonomia global e imutável usada pelos módulos futuros do MeControla. O MVP possui somente três responsabilidades:

1. manter categorias, subcategorias e dicionário editorial por migrations versionadas;
2. listar o catálogo hierárquico de receitas e despesas;
3. consultar o dicionário para recuperar candidatos explicáveis.

O módulo não classifica lançamentos, não executa ações em nome do usuário e não aprende com conversas. Consumidores como WhatsApp, API ou agentes de IA recebem candidatos e são responsáveis por qualquer decisão posterior.

Despesas (`kind=expense`) possuem exatamente cinco categorias raiz: **Custo Fixo, Conhecimento, Prazeres, Metas e Liberdade Financeira**. Receitas (`kind=income`) mantêm a taxonomia previamente aprovada, com os ajustes necessários para que toda classificação futura possa apontar para uma subcategoria.

### Volumetria-alvo e SLO do MVP

Todas as metas de latência, dimensionamento e SLO presumem o seguinte cenário de proteção:

- **Catálogo**: até 400 subcategorias ativas distribuídas entre as raízes de receita e despesa.
- **Dicionário**: até 5.000 entradas ativas (média ~12 entradas por subcategoria, considerando canonical, aliases, frases, merchants e segments).
- **Carga**: até 20 RPS agregados entre `GET /v1/categories`, `GET /v1/categories/{id}`, `GET /v1/category-dictionary` e `GET /v1/category-dictionary/search`.
- **Migrações editoriais**: até 4 por mês em janela controlada.
- **Disponibilidade**: SLO mensal de 99,5% por endpoint público do módulo.

Dimensionamento, índices, cache e estratégia de capacidade devem ser planejados para essa volumetria como teto de proteção do MVP. Crescimento além desse teto exige revisão deste PRD.

## Objetivos

- **OBJ-01**: disponibilizar um catálogo global PT-BR consistente sem configuração por usuário.
- **OBJ-02**: fornecer listagem hierárquica estável para módulos consumidores.
- **OBJ-03**: oferecer dicionário amplo e explicável sem produzir classificação automática ou falso positivo.
- **OBJ-04**: garantir evolução editorial auditável por migration, pull request e testes positivos/negativos.

### Métricas de Sucesso

- **M-01**: 100% das categorias, subcategorias e entradas de dicionário ativas são criadas por migrations idempotentes.
- **M-02**: p95 de `GET /v1/categories` menor ou igual a 100 ms, medido na volumetria-alvo (até 400 subcategorias, até 20 RPS).
- **M-03**: p95 de `GET /v1/category-dictionary/search` menor ou igual a 150 ms, medido na volumetria-alvo (até 5.000 entradas, até 20 RPS).
- **M-04**: 100% dos candidatos retornam caminho completo, termo encontrado, tipo de sinal, confiança editorial e ambiguidade.
- **M-05**: 0 respostas do dicionário declaram uma categoria como decisão final.
- **M-06**: 100% dos termos ambíguos possuem testes negativos que impedem retorno como candidato inequívoco.
- **M-07**: SLO mensal de disponibilidade de 99,5% por endpoint público do módulo, com error budget de aproximadamente 3h38min/mês acompanhado em dashboard de plataforma.
- **M-08**: 100% das migrations editoriais são append-only e reversíveis exclusivamente por nova migration de depreciação + inclusão de ID novo (RF-36a).
- **M-09**: 100% dos endpoints públicos do módulo são descritos no `openapi.yaml` versionado no repositório e publicado como artifact de CI a cada release.

## Histórias de Usuário

- **US-01 — Listagem hierárquica**
  Como módulo consumidor, quero listar categorias e subcategorias globais por `kind`, para apresentar uma taxonomia consistente.

- **US-02 — Consulta ao dicionário**
  Como módulo consumidor, quero consultar termos conversacionais e estabelecimentos, para obter até três candidatos explicáveis sem receber uma classificação automática.

- **US-03 — Evolução editorial**
  Como operador, quero evoluir catálogo e dicionário por migration revisada e testada, para preservar rastreabilidade e evitar regressões.

## Funcionalidades Core

### F-01 — Catálogo Global Somente Leitura

Todas as categorias e subcategorias são globais, pertencem ao sistema e são imutáveis via API. Não existem categorias personalizadas, aliases pessoais, clones, ocultação individual ou mutações por usuário.

### F-02 — Hierarquia de Dois Níveis

O catálogo possui somente categoria raiz e subcategoria. Toda subcategoria possui exatamente uma raiz do mesmo `kind`. Resultados sempre exibem o caminho completo `Raiz > Subcategoria`.

### F-03 — Seed Editorial Versionado

Catálogo e dicionário são criados e evoluídos exclusivamente por migrations idempotentes. Alterações exigem pull request, revisão obrigatória e testes positivos/negativos.

### F-04 — Listagem e Consulta

A API somente leitura permite listar o catálogo, obter uma categoria específica e consultar entradas do dicionário por subcategoria.

### F-05 — Busca Conservadora no Dicionário

A busca retorna no máximo três candidatos. Cada candidato explica por que foi recuperado. Termos ambíguos permanecem marcados como ambíguos. Sem correspondência segura, retorna `no_match`; o módulo nunca escolhe uma categoria.

## Requisitos Funcionais

### Catálogo e Hierarquia

- **RF-01**: Toda categoria DEVE possuir `id` determinístico, `slug` PT-BR estável, `name`, `kind` (`income`|`expense`), `parent_id` opcional, `allocation_type` (`consumption`|`asset_allocation`) e `deprecated_at` opcional. O `id` DEVE ser UUIDv5 calculado sobre o namespace fixo do módulo `categories` e o par `(kind, slug)`; o `slug` é PT-BR em kebab-case e é a representação editorial humana para revisão em pull request. O `id` é opaco para consumidores e nunca aparece na URL como slug.
- **RF-02**: Toda categoria raiz DEVE possuir `parent_id=NULL`; toda subcategoria DEVE apontar para exatamente uma raiz do mesmo `kind`.
- **RF-03**: A profundidade máxima DEVE ser dois níveis. Subcategoria não pode possuir filhos.
- **RF-04**: Toda classificação futura DEVE apontar para uma subcategoria; raízes servem somente para agrupamento.
- **RF-05**: Todas as subcategorias de `Metas` e `Liberdade Financeira` DEVEM possuir `allocation_type=asset_allocation`. As demais possuem `allocation_type=consumption`.
- **RF-06**: O catálogo global NÃO DEVE possuir `user_id`, propriedade por usuário ou estado de visibilidade individual.
- **RF-07**: Não DEVEM existir endpoints de create, update, delete, clone, restore ou hide no MVP.

### API Somente Leitura

- **RF-08**: `GET /v1/categories` DEVE listar categorias e subcategorias ativas.
- **RF-09**: `GET /v1/categories` DEVE aceitar filtros opcionais `kind`, `parent_id` e `include_deprecated=false`.
- **RF-10**: A listagem sem `parent_id` DEVE retornar árvore hierárquica; cada raiz inclui `subcategories`.
- **RF-11**: A ordenação DEVE ser alfabética PT-BR por raiz e subcategoria.
- **RF-12**: `GET /v1/categories/{id}` DEVE retornar o item, seu caminho completo e, quando raiz, suas subcategorias.
- **RF-13**: Categoria inexistente ou descontinuada sem `include_deprecated=true` DEVE retornar `404 Not Found`.
- **RF-14**: `GET /v1/category-dictionary` DEVE listar entradas ativas com filtros opcionais `category_id`, `kind` e `signal_type`.
- **RF-14a**: A listagem do dicionário DEVE usar paginação cursor-based, com `page_size=50` por padrão e máximo de 200.
- **RF-15**: `GET /v1/category-dictionary/search?q=<termo>&kind=<kind>` DEVE retornar até três candidatos e `has_more`.
- **RF-15a**: A busca DEVE ignorar entradas com `deprecated_at` preenchido em qualquer cenário. A flag `include_deprecated` NÃO se aplica à busca, vale exclusivamente para listagens (`GET /v1/categories` e `GET /v1/category-dictionary`). Substituição editorial corrige o falso positivo imediatamente após a migration ser aplicada.
- **RF-16**: `kind` é obrigatório na busca do dicionário. Ausência ou valor inválido retorna `422 invalid_kind`.
- **RF-16a**: A busca DEVE rejeitar `q` cujo comprimento, após normalização (RF-20) e trim, seja menor que 3, retornando `422 invalid_query`. `q` ausente, vazio, composto apenas por espaços ou apenas por pontuação são tratados como `q` normalizado vazio e produzem `422 invalid_query`.
- **RF-17**: Todo candidato DEVE retornar `category_id`, `root_category_id`, `path`, `matched_term`, `signal_type`, `confidence`, `is_ambiguous` e `match_reason`.
- **RF-18**: A resposta da busca DEVE ser `candidates` ou `no_match`; nunca deve conter decisão final, categoria selecionada ou instrução para persistir lançamento.
- **RF-18a**: Respostas de catálogo e dicionário DEVEM informar uma versão editorial monotônica para permitir cache e invalidação por consumidores. A versão DEVE ser exposta de duas formas equivalentes em toda resposta de leitura: (a) header HTTP `ETag: "v<N>"`, onde `N` é inteiro monotônico crescente incrementado a cada migration editorial aplicada com sucesso; (b) campo `version` no corpo JSON com o mesmo valor de `N`. O módulo DEVE aceitar `If-None-Match: "v<N>"` em qualquer endpoint de leitura e responder `304 Not Modified` sem corpo quando `N` for igual à versão atual. A versão NÃO regride; rollback editorial cria nova versão (RF-36a). Cache HTTP via `Cache-Control` fica a critério da plataforma/gateway e não é responsabilidade do módulo.

### Dicionário e Proteção Contra Falso Positivo

- **RF-19**: Cada entrada de dicionário DEVE apontar para exatamente uma subcategoria e possuir `id` determinístico, `term`, `signal_type` (`canonical_name`|`alias`|`phrase`|`merchant`|`segment`), `confidence` (`high`|`medium`|`low`), `is_ambiguous`, `version` e `deprecated_at` opcional.
- **RF-20**: Busca e unicidade editorial DEVEM ser case-insensitive e accent-insensitive. A normalização DEVE ser persistida em coluna gerada `term_normalized GENERATED ALWAYS AS (lower(unaccent(term))) STORED`, com índice B-tree de unicidade no par `(kind, category_id, term_normalized)` para entradas ativas e índice de busca em `term_normalized`. A extensão Postgres `unaccent` é restrição técnica obrigatória do módulo (RT-09). Toda comparação de igualdade contra `q` DEVE usar `term_normalized = lower(unaccent($1))` aplicado no servidor; a aplicação não computa normalização própria e não há paridade Go/SQL para manter.
- **RF-21**: Nome canônico, alias e frase somente podem usar `confidence=high` quando forem inequívocos dentro do mesmo `kind`.
- **RF-22**: Toda entrada `merchant` ou `segment` DEVE possuir `is_ambiguous=true`. Estabelecimento ou segmento isolado nunca é candidato inequívoco.
- **RF-23**: Um mesmo termo que aponta para múltiplas subcategorias DEVE marcar todas as entradas correspondentes como ambíguas.
- **RF-24**: Termos ambíguos, fuzzy ou de baixa confiança nunca podem ser promovidos para alta confiança em runtime.
- **RF-25**: O MVP NÃO DEVE usar fuzzy matching, inferência semântica, IA, histórico do usuário ou contexto conversacional para criar candidatos adicionais. A busca usa somente correspondência normalizada exata sobre entradas editoriais.
- **RF-26**: A correspondência normalizada exata DEVE considerar o valor completo de `q`; o módulo não extrai tokens, substrings ou entidades de uma frase. Sem correspondência exata do termo completo, a busca retorna `no_match`.
- **RF-27**: O conjunto de candidatos retornados pela busca DEVE ser deduplicado por `category_id`: cada subcategoria aparece no máximo uma vez. Quando múltiplas entradas do dicionário apontam para a mesma subcategoria e casam com `q`, prevalece a entrada de maior precedência editorial, na ordem `canonical_name > alias > phrase > merchant > segment`; empates dentro do mesmo `signal_type` resolvem por caminho alfabético PT-BR. O `matched_term` exposto reflete a entrada vencedora. Após a deduplicação, se o conjunto contiver mais de um `category_id`, TODOS os candidatos DEVEM ser marcados com `is_ambiguous=true` na resposta, independentemente da confiança individual da entrada vencedora — a coexistência de subcategorias distintas é, por si só, ambiguidade no MVP. A ordenação final do array de candidatos DEVE seguir a mesma precedência de `signal_type` e depois caminho alfabético PT-BR. `has_more=true` DEVE ser definido quando existir ao menos um `category_id` adicional, ativo e correspondente a `q`, que tenha sido descartado pelo limite de três candidatos.
- **RF-28**: O dicionário NÃO DEVE conter meios de pagamento, valores, datas, identificadores de transação, stopwords ou termos de uma ou duas letras como aliases.
- **RF-29**: Toda subcategoria DEVE possuir uma entrada `canonical_name`. Aliases adicionais só podem existir quando houver termo real e útil; é proibido inventar sinônimos para atingir volume.

### Seed de Despesas

- **RF-30**: O seed de despesas DEVE possuir exatamente as cinco raízes e subcategorias abaixo.

- **Custo Fixo** → Aluguel, Financiamento Imobiliário, Condomínio, IPTU, Taxas Residenciais, Seguro Residencial, Energia, Água, Gás, Internet, Telefonia, TV por Assinatura, Supermercado, Feira e Hortifruti, Açougue, Padaria, Transporte Público, Transporte por Aplicativo Recorrente, Combustível, Estacionamento Mensal, Pedágio, Manutenção Veicular, IPVA e Licenciamento, Seguro Veicular, Plano de Saúde, Plano Odontológico, Consultas e Exames, Medicamentos Contínuos, Medicamentos e Farmácia, Odontologia, Terapia e Saúde Mental, Escola e Creche, Faculdade e Pós-graduação, Pensão Alimentícia, Seguros Pessoais, Assinaturas Essenciais, Tarifas Bancárias, Impostos e Tributos, Empréstimos e Financiamentos, Dívidas e Juros, Manutenção da Casa, Serviços Domésticos, Pets Recorrentes, Outros Custos Fixos
- **Conhecimento** → Cursos e Treinamentos, Plataformas de Ensino, Livros e E-books, Material de Estudo, Certificações, Congressos e Workshops, Idiomas, Mentoria e Coaching, Aulas Particulares, Software e Ferramentas de Estudo, Outros Conhecimentos
- **Prazeres** → Delivery, Restaurantes, Bares e Lanches, Cafeterias, Streaming de Vídeo, Música e Áudio, Games e Assinaturas de Jogos, Cinema e Teatro, Shows e Eventos, Passeios e Parques, Transporte de Lazer, Viagens de Lazer, Hospedagem de Lazer, Compras Pessoais, Roupas e Calçados, Beleza e Estética, Hobbies, Esportes e Academia, Presentes, Pets Não Recorrentes, Doações, Outros Prazeres
- **Metas** → Tecnologia, Veículo, Casa e Reforma, Viagem Planejada, Casamento e Festa, Família e Enxoval, Empreendedorismo, Educação Planejada, Saúde Planejada, Quitação de Dívidas, Compra Planejada, Outras Metas
- **Liberdade Financeira** → Reserva de Emergência, Reserva de Oportunidade, Tesouro Direto, CDB e RDB, LCI e LCA, Debêntures e Crédito Privado, Fundos de Renda Fixa, Ações, ETFs, Fundos Imobiliários, BDRs, Fundos de Investimento, Previdência Privada, Criptoativos, Investimentos Internacionais, Aportes em Corretora, Outros Investimentos

### Seed de Receitas

- **RF-31**: O seed de receitas DEVE possuir as raízes e subcategorias abaixo.

- **Salário** → Salário, Décimo Terceiro, Férias, PLR e Bônus, Vale-Alimentação, Vale-Refeição
- **Renda Variável** → Freelance, Trabalho Extra, Consultoria
- **Investimentos** → Rendimentos, Dividendos, Juros, Resgates
- **Aluguel Recebido** → Aluguel Residencial Recebido
- **Restituições e Cashback** → Restituição de IR, Cashback
- **Presentes Recebidos** → Presentes em Dinheiro, Mesada Recebida
- **Vendas** → Vendas Diversas, Marketplace
- **Outras Receitas** → Outros

### Conteúdo Mínimo do Dicionário

- **RF-32**: O dicionário DEVE cobrir nomes canônicos de todas as subcategorias de receitas e despesas.
- **RF-33**: O dicionário DEVE cobrir, no mínimo, os aliases e frases inequívocos:

| Subcategoria | Aliases e frases mínimas |
| --- | --- |
| Aluguel | alug, locação residencial, aluguel da casa, aluguel do apartamento, aluguel do apê |
| Financiamento Imobiliário | financiamento do imóvel, parcela da casa, parcela do apartamento, prestação do imóvel |
| Condomínio | taxa condominial, boleto do condomínio |
| IPTU | imposto predial |
| Energia | conta de luz, eletricidade |
| Água | conta de água, saneamento |
| Gás | botijão, gás encanado, conta de gás |
| Internet | banda larga, fibra, wi-fi residencial |
| Telefonia | plano móvel, recarga de celular |
| Supermercado | compras do mês, mercearia |
| Feira e Hortifruti | feira, hortifruti, sacolão, frutas e verduras |
| Açougue | carnes, frigorífico |
| Transporte Público | ônibus, metrô, trem, bilhete único, passagem urbana |
| Transporte por Aplicativo Recorrente | corrida para o trabalho, uber para o trabalho, 99 para o trabalho |
| Combustível | gasolina, etanol, diesel |
| Manutenção Veicular | oficina, revisão do carro, troca de óleo, borracharia |
| IPVA e Licenciamento | ipva, licenciamento, documento do carro |
| Plano de Saúde | convênio médico |
| Plano Odontológico | convênio odontológico |
| Consultas e Exames | consulta médica, exame médico, laboratório, diagnóstico por imagem |
| Medicamentos Contínuos | remédio contínuo, medicamento contínuo, remédio de uso contínuo |
| Medicamentos e Farmácia | drogaria |
| Odontologia | dentista, consulta odontológica, tratamento dentário |
| Terapia e Saúde Mental | terapia, psicólogo, psicóloga, psiquiatra |
| Escola e Creche | mensalidade escolar |
| Faculdade e Pós-graduação | universidade, pós-graduação, mensalidade da faculdade |
| Cursos e Treinamentos | treinamento, formação, bootcamp |
| Plataformas de Ensino | plataforma de cursos, assinatura de cursos |
| Livros e E-books | livro, livros, e-book, ebook, kindle, livraria, sebo |
| Certificações | certificação, prova de certificação, exame profissional |
| Congressos e Workshops | congresso, workshop, seminário, palestra, feira de negócios |
| Idiomas | inglês, espanhol, francês, alemão, curso de idiomas |
| Delivery | entrega de comida, pedir comida |
| Restaurantes | restaurante, jantar fora, almoço fora |
| Bares e Lanches | bar, boteco, lanche, hamburgueria, pizzaria |
| Cafeterias | cafeteria, coffee shop |
| Streaming de Vídeo | filme online, série online |
| Música e Áudio | streaming de música, podcast premium |
| Games e Assinaturas de Jogos | game, videogame, assinatura de jogos |
| Cinema e Teatro | ingresso de cinema, ingresso de teatro |
| Shows e Eventos | show, festival |
| Passeios e Parques | passeio, parque, zoológico, parque aquático |
| Transporte de Lazer | corrida para passeio, transporte para passeio, volta do bar |
| Viagens de Lazer | viagem de lazer, férias, passagem de férias |
| Hospedagem de Lazer | pousada, hospedagem, resort |
| Roupas e Calçados | roupa, roupas, calçado, sapato, tênis |
| Beleza e Estética | salão, cabeleireiro, manicure, pedicure, barbeiro, estética, maquiagem |
| Hobbies | hobby, artesanato, fotografia, coleção |
| Esportes e Academia | academia, esporte, futebol, corrida, ciclismo |
| Tecnologia | celular novo, iphone, smartphone, notebook, computador, tablet, smartwatch |
| Veículo | carro novo, moto nova, entrada do veículo, troca de carro |
| Casa e Reforma | reforma, móveis, geladeira, sofá, televisão, ar-condicionado |
| Viagem Planejada | viagem planejada, fundo de viagem, guardar para viajar |
| Família e Enxoval | enxoval, chá de bebê, chegada do bebê |
| Empreendedorismo | abrir empresa, capital de giro, equipamento para empresa |
| Quitação de Dívidas | quitar dívida, amortizar dívida, antecipar financiamento |
| Reserva de Emergência | reserva emergencial, fundo de emergência |
| Tesouro Direto | tesouro selic, tesouro ipca, tesouro prefixado |
| Fundos Imobiliários | fii, fiis, fundo imobiliário |
| Previdência Privada | pgbl, vgbl |
| Criptoativos | cripto, criptomoeda, bitcoin, ethereum, solana, usdt |
| Investimentos Internacionais | stocks, etf internacional |
| Décimo Terceiro | 13º salário, décimo terceiro salário |
| PLR e Bônus | plr, participação nos lucros, bônus salarial |
| Freelance | freela, trabalho freelancer |
| Rendimentos | rendimento de investimento |
| Dividendos | dividendo, provento |
| Aluguel Residencial Recebido | aluguel recebido, renda de aluguel residencial |
| Restituição de IR | restituição do imposto de renda, restituição ir |
| Cashback | dinheiro de volta |
| Presentes em Dinheiro | presente em dinheiro |
| Vendas Diversas | venda, vendas |

- **RF-34**: Os seguintes termos DEVEM existir somente como entradas ambíguas ou não existir como alias: `compra`, `pix`, `boleto`, `cartão`, `parcela`, `transferência`, `débito`, `mercado`, `farmácia`, `remédio`, `Uber`, `99`, `Amazon`, `celular`, `telefone`, `café`, `pão`, `posto`, `hotel`, `evento`, `ingresso`, `viagem`, `curso`, `presente`, `investimento`.
- **RF-35**: Marcas e estabelecimentos do documento `docs/discoveries/MeControla_Dicionario_Expandido.md` DEVEM ser cadastrados como `merchant`, sempre com `is_ambiguous=true`.

### Evolução Editorial

- **RF-36**: Seed e dicionário DEVEM ser append-only. Correção ou substituição cria novo ID e preenche `deprecated_at` no item anterior.
- **RF-36a**: Rollback editorial DEVE ser feito exclusivamente por nova migration append-only que (i) preenche `deprecated_at` no item incorreto e (ii) opcionalmente insere um novo item com ID novo (UUIDv5 a partir de novo slug). É proibido `UPDATE` destrutivo em colunas de identidade (`id`, `slug`, `kind`, `parent_id`, `category_id`), `confidence` ou `term` de itens já publicados, mesmo em incidente — a única forma de corrigir um valor publicado é deprecando e republicando com ID novo. Durante a janela de coexistência, item depreciado e item substituto convivem no banco; a busca já ignora o depreciado por RF-15a, enquanto a listagem segue exigindo `include_deprecated=true` para exibi-lo. A janela mínima de coexistência DEVE ser de 7 dias para permitir invalidação de cache por consumidores via `ETag`/`If-None-Match`. Toda migration de rollback DEVE incrementar a versão editorial monotônica (RF-18a) e registrar no PR o ID depreciado, o ID novo e a justificativa do incidente.
- **RF-37**: Item descontinuado NÃO DEVE aparecer por padrão, mas permanece consultável com `include_deprecated=true`.
- **RF-38**: Alteração editorial DEVE ocorrer exclusivamente por migration versionada em pull request com revisão obrigatória.
- **RF-39**: Cada entrada de alta confiança DEVE possuir teste positivo. Cada termo ambíguo DEVE possuir testes negativos que comprovem ausência de candidato inequívoco.
- **RF-40**: Migration DEVE validar IDs determinísticos, ausência de ciclos, profundidade, `kind`, nomes normalizados duplicados e referências do dicionário.
- **RF-40a**: Migration editorial inválida DEVE falhar integralmente, sem aplicar seed parcial.

### Observabilidade

- **RF-41**: O módulo DEVE expor métricas de latência e resultado para listagem e busca, sem labels com termo consultado, estabelecimento ou identificador de usuário. As únicas labels permitidas em métricas da busca do dicionário são `endpoint`, `kind`, `outcome` (`matched`|`ambiguous`|`no_match`|`invalid_query`|`invalid_kind`), `q_len_bucket` (`3-4`|`5-8`|`9-16`|`17-32`|`33+`) e `signal_type_top` (somente quando houver candidato vencedor).
- **RF-42**: Logs, traces e métricas NÃO DEVEM registrar a consulta bruta ao dicionário, nem sua versão normalizada, nem qualquer hash, prefixo ou substring derivada do termo. Telemetria editorial restringe-se aos counters definidos em RF-41 e RF-43.
- **RF-43**: A busca DEVE medir resultados `matched`, `ambiguous`, `no_match`, `invalid_query` e `invalid_kind` como counters distintos, agregados pelo bucket de tamanho `q_len_bucket` de RF-41. Não é permitido amostrar termos brutos nem em ambientes não produtivos servidos pelo módulo.

## Restrições Técnicas de Alto Nível

- **RT-01**: Postgres é o único storage; catálogo e dicionário são materializados por migrations.
- **RT-02**: API REST JSON somente leitura sob `/v1/categories` e `/v1/category-dictionary`.
- **RT-03**: O módulo segue o layout e o wiring manual definidos em `AGENTS.md`.
- **RT-04**: Não há dependência de identidade, `user_id`, audit log de usuário, outbox, worker, bot WhatsApp ou agente de IA no MVP.
- **RT-05**: A busca do dicionário é determinística e baseada apenas em dados editoriais persistidos.
- **RT-06**: O catálogo é PT-BR exclusivo no MVP.
- **RT-07**: Testes de integração com Postgres real DEVEM validar schema, seed, listagem, busca e migrations.
- **RT-08**: Rate limiting, autenticação entre serviços e cache HTTP são responsabilidades da plataforma/gateway; o módulo não acessa dados de usuário. O módulo NÃO implementa autenticação própria no MVP: serve requisições anônimas dentro da rede interna confiável (gateway/mesh com mTLS ou network policy garante autorização). Endpoints públicos do módulo NÃO DEVEM ser expostos diretamente à internet sem mediação do gateway.
- **RT-09**: A extensão Postgres `unaccent` é dependência obrigatória do módulo e DEVE ser instalada/habilitada por migration controlada antes de qualquer migration de seed.
- **RT-10**: O módulo DEVE manter `openapi.yaml` versionado no diretório do módulo (`internal/categories`) como fonte canônica do contrato HTTP. CI DEVE validar o arquivo a cada PR (lint OpenAPI + diff de breaking changes) e publicá-lo como artifact a cada release. Breaking changes exigem versionamento de URL (`/v2`) ou janela de deprecação documentada.
- **RT-11**: O envelope de resposta de erro HTTP DEVE seguir o padrão já adotado por `internal/billing` e `internal/identity`, reutilizando tipos compartilhados de erro/handler. Códigos exigidos pelo PRD: `invalid_query`, `invalid_kind`, `not_found`. Detalhamento do shape exato fica para a especificação técnica, desde que preservada a paridade com os módulos citados.

## Fora de Escopo

- **OUT-01**: CRUD, personalização, clone, ocultação ou qualquer mutação via API.
- **OUT-02**: Preferências, aliases pessoais, aprendizado por usuário ou consentimento “usar sempre”.
- **OUT-03**: Classificação automática ou final de lançamentos.
- **OUT-04**: Criação, edição, exclusão ou categorização de lançamentos.
- **OUT-05**: Diálogo, confirmação ou qualquer ação executada pelo WhatsApp.
- **OUT-06**: Inferência por IA, fuzzy matching ou expansão semântica em runtime.
- **OUT-07**: Tratamento de transferências, faturas, estornos, reembolsos ou outros tipos de transação.
- **OUT-08**: Hierarquia com profundidade maior que dois.
- **OUT-09**: Cor, ícone, ordenação manual, multi-idioma ou multi-moeda.
- **OUT-10**: API administrativa de edição; operadores alteram o catálogo somente por migration revisada.

## Cenários Canônicos de Aceitação

Os cenários abaixo fixam o contrato observável do módulo e DEVEM ser cobertos como testes automatizados (positivos e negativos) na implementação. Inputs assumem normalização aplicada por RF-20. `version=42` é ilustrativo; o valor real reflete a versão editorial monotônica corrente.

### Cenários básicos da busca (RF-15 a RF-27)

- **CC-B1 — High inequívoco**: `GET /v1/category-dictionary/search?q=13º%20salário&kind=income`
  → `200 OK`, `candidates=[{path:"Salário > Décimo Terceiro", signal_type:"alias", confidence:"high", is_ambiguous:false, matched_term:"13º salário", match_reason:"alias inequívoco"}]`, `has_more=false`.

- **CC-B2 — Merchant ambíguo com várias subcategorias**: `GET /v1/category-dictionary/search?q=uber&kind=expense`
  → `200 OK`, `candidates` contém no máximo 3 candidatos, todos com `is_ambiguous=true` independente do `signal_type`; pelo menos `Transporte por Aplicativo Recorrente` e `Transporte de Lazer` aparecem; ordenação respeita precedência `signal_type` (RF-27); `has_more` reflete a existência de subcategorias adicionais correspondentes.

- **CC-B3 — Sem correspondência exata**: `GET /v1/category-dictionary/search?q=xyz123&kind=expense`
  → `200 OK`, corpo `{result:"no_match", version:42}`. Sem candidatos. Sem decisão.

- **CC-B4 — Kind mismatch (termo válido em outro kind)**: `GET /v1/category-dictionary/search?q=energia&kind=income`
  → `200 OK`, corpo `{result:"no_match", version:42}`. O termo existe como canônico em `expense`, mas é filtrado pelo `kind`.

- **CC-B5 — Empate em alta confiança**: dois canonicais distintos casando com `q` no mesmo `kind` (cenário sintético em testes editoriais) → todos retornam com `is_ambiguous=true`, respeitando precedência editorial e caminho alfabético.

### Cenários de input degenerado (RF-16, RF-16a)

- **CC-D1 — `q` curto**: `GET /v1/category-dictionary/search?q=ab&kind=expense`
  → `422 invalid_query` no envelope padrão de erro (RT-11).

- **CC-D2 — `q` vazio**: `GET /v1/category-dictionary/search?q=&kind=expense`
  → `422 invalid_query`.

- **CC-D3 — `q` somente espaços ou pontuação**: `GET /v1/category-dictionary/search?q=%20%20%20&kind=expense` ou `q=...`
  → `422 invalid_query`, porque o termo normalizado e trimado tem comprimento zero.

- **CC-D4 — `kind` ausente**: `GET /v1/category-dictionary/search?q=energia`
  → `422 invalid_kind`.

- **CC-D5 — `kind` inválido**: `GET /v1/category-dictionary/search?q=energia&kind=foo`
  → `422 invalid_kind`.

### Cenários de listagem (RF-08 a RF-13, RF-37)

- **CC-L1 — Árvore completa por kind**: `GET /v1/categories?kind=expense`
  → `200 OK`, retorna exatamente 5 raízes (`Custo Fixo`, `Conhecimento`, `Prazeres`, `Metas`, `Liberdade Financeira`), cada uma com `subcategories` ordenadas alfabeticamente em PT-BR; nenhum item `deprecated_at` é exibido.

- **CC-L2 — Filtro por `parent_id`**: `GET /v1/categories?parent_id=<id Custo Fixo>`
  → `200 OK`, retorna somente subcategorias diretas de `Custo Fixo`, em ordem alfabética PT-BR.

- **CC-L3 — Inclusão de deprecated na listagem**: `GET /v1/categories?include_deprecated=true`
  → `200 OK`, retorna itens ativos e itens com `deprecated_at` preenchido, cada item depreciado claramente sinalizado por `deprecated_at` no JSON.

- **CC-L4 — Categoria descontinuada por path padrão**: `GET /v1/categories/{id depreciado}` sem `include_deprecated`
  → `404 not_found` no envelope padrão de erro.

- **CC-L5 — Listagem do dicionário com paginação cursor-based**: `GET /v1/category-dictionary?page_size=50`
  → `200 OK`, retorna até 50 entradas, cursor opaco em `next_cursor` quando aplicável; `page_size>200` é capado em 200.

### Cenários de cache e versionamento (RF-18a)

- **CC-V1 — Primeira requisição expõe `ETag` e `version`**: `GET /v1/categories?kind=expense`
  → `200 OK`, header `ETag: "v42"`, corpo contém `version:42`.

- **CC-V2 — Revalidação retorna 304**: `GET /v1/categories?kind=expense` com header `If-None-Match: "v42"`
  → `304 Not Modified`, sem corpo, header `ETag: "v42"`.

- **CC-V3 — Migration editorial incrementa versão**: após aplicar com sucesso uma migration editorial que altera catálogo ou dicionário, qualquer endpoint de leitura DEVE responder com `ETag: "v43"` e `version:43`. Requisições com `If-None-Match: "v42"` passam a retornar `200` com o corpo atualizado.

- **CC-V4 — Migration falha não incrementa versão**: migration que falhe na validação (RF-40, RF-40a) NÃO DEVE incrementar a versão; `ETag` permanece `"v42"`.

## Decisões Fechadas

| Decisão | Resultado |
| --- | --- |
| Escopo do MVP | Seed, listagem de categorias/subcategorias e dicionário |
| Catálogo | Global, curado, imutável e somente leitura |
| Personalização | Inexistente para despesas e receitas |
| Raízes de despesas | Exatamente cinco |
| Classificação final | Fora do módulo e fora do MVP |
| Resultado do dicionário | Até três candidatos explicáveis ou `no_match` |
| Proteção contra falso positivo | Correspondência normalizada exata; sem fuzzy, IA ou decisão automática |
| Estabelecimentos e segmentos | Sempre ambíguos |
| Evolução editorial | Migration append-only via pull request revisado e testado |
| Caminho exibido | Sempre `Raiz > Subcategoria` |
| Ambiguidade com múltiplos candidatos | Estrito: `>1 candidato → todos ambíguos` |
| `q` mínimo | `422 invalid_query` se `q` normalizado < 3 caracteres |
| Versão editorial | `ETag` + `body.version` monotônica + suporte a `If-None-Match → 304` |
| Normalização | Coluna gerada `term_normalized = lower(unaccent(term))` + extensão `unaccent` |
| Autenticação | Anônima na rede interna; gateway/mesh garante |
| Telemetria de `q` | Somente counters com `outcome` + `q_len_bucket`; sem termo bruto, sem hash |
| ID determinístico | UUIDv5 com namespace fixo + slug PT-BR estável |
| Deduplicação na busca | 1 candidato por `category_id`; precedência `canonical > alias > phrase > merchant > segment` |
| Busca + depreciado | Busca ignora `deprecated_at` sempre; flag vale só para listagem |
| Erros HTTP | Reusa envelope dos módulos `internal/billing` e `internal/identity` |
| Volumetria-alvo | ~400 subcategorias / ~5.000 entradas / 20 RPS |
| SLO mensal | 99,5% por endpoint público |
| Rollback editorial | Append-only puro: deprecia + cria ID novo; coexistência mínima de 7 dias |
| Contrato HTTP | `openapi.yaml` versionado no módulo + CI artifact por release |

## Suposições e Questões em Aberto

Não existem questões de produto abertas para este escopo. Decisões de schema físico, índices (formato físico do índice em `term_normalized`, estratégia de paginação cursor opaco), implementação HTTP (handler, envelope concreto de erro, layout do `openapi.yaml`), formato exato do header `ETag` (peso, validador forte/fraco) e estratégia operacional de aplicar migrations editoriais em produção ficam para a especificação técnica, sem alterar os requisitos acima.
