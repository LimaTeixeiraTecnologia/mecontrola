# Documento de Requisitos do Produto (PRD)

<!-- spec-version: 3 -->

<!--
Histórico de versões:
- v1 (2026-06-06): versão inicial derivada do bundle de brainstorming.
- v2 (2026-06-06): refinamento de negócio após confronto com codebase (internal/identity, internal/billing, migrations/) e bundle de brainstorming. Decisões de negócio fechadas: ator e autenticação, registro de canal `source`, retenção do audit log, versionamento do seed (append-only). Decisões técnicas foram explicitamente delegadas à techspec.
- v3 (2026-06-06): eliminação de todas as dúvidas de negócio via 5 rodadas de múltipla escolha. Decisões fechadas: clone idempotente (200 OK), bloqueio de homônimos sistema/usuário, clone só da raiz, evolução automática do seed, limites de quota (100/200), unicode no nome, ordenação alfabética PT-BR, parent_id imutável, audiência PT-BR exclusiva, ciclo de vida do clone (sistema reaparece após delete), busca case+accent insensitive, audit log apenas mutações e apenas para uso interno, snapshot completo no audit, idioma da API (códigos EN + mensagens PT-BR), busca limitada a top 5 com paginação.
-->

> **Origem**: Bundle de brainstorming decisório em `docs/discoveries/brainstorms/brainstorm-crud-de-categorias-e-subcategorias-de-despesas/` (validado com SUCCESS em 2026-06-06). A direção arquitetural (Adjacency List, profundidade=2, kind na raiz, outbox completo, audit log, seed via migration, canal WhatsApp) já está confirmada explicitamente pelo usuário e materializada como restrição não negociável neste PRD.

## Visão Geral

O `mecontrola` é um monolito modular Go (Postgres + outbox) consumido prioritariamente via bot WhatsApp para gestão de finanças pessoais. Hoje os módulos `internal/identity` (autenticação e entitlements) e `internal/billing` (assinaturas Kiwify) estão consolidados, mas o sistema **não classifica fluxo financeiro** — não há categorias de receitas e despesas. Sem essa fundação, ficam bloqueadas todas as features de orçamento, dashboards, metas, alertas e relatórios.

Este PRD define a entrega do módulo **`internal/categories`**, responsável por padronizar a taxonomia de fluxo financeiro pessoal (receitas e despesas) com hierarquia em dois níveis (categoria pai → subcategoria), seed inicial pré-curado a partir de referências do mercado brasileiro (Mobills, Organizze, Banco do Brasil, PayPal BR, Embracon, método 50-30-20), e personalização por usuário. A entrega inclui CRUD completo, audit log, eventos de domínio publicados via outbox e endpoint de busca fuzzy preparado para o bot WhatsApp.

O valor primário é destravar o backlog de finanças (orçamento, dashboards, metas) com um contrato estável e robusto desde a primeira onda — privilegiando consistência com `internal/billing` e `internal/identity` em vez de atalhos MVP descartáveis.

## Objetivos

- **OBJ-01**: Entregar taxonomia de fluxo financeiro out-of-the-box para qualquer usuário recém-cadastrado, sem onboarding manual de categorias.
- **OBJ-02**: Permitir personalização por usuário sem comprometer a imutabilidade do seed (catálogo de sistema editável apenas via migration).
- **OBJ-03**: Servir como **fundação** para módulos consumidores futuros (transactions, budget, dashboards) através de contrato HTTP estável e eventos de domínio idempotentes.
- **OBJ-04**: Atender o canal WhatsApp como primeiro consumidor produtivo, com endpoint de busca fuzzy que resolva intenção do usuário em linguagem natural.
- **OBJ-05**: Preservar histórico contábil: nenhuma operação de delete remove dados em hard-delete; nenhum lançamento futuro perde rastreabilidade da categoria.

### Métricas de Sucesso (Mensuráveis)
- **M-01**: 100% dos usuários ativos enxergam o seed do sistema na primeira chamada de listagem após a release.
- **M-02**: p95 de latência da listagem (`GET /v1/categories`) ≤ 100 ms em produção (medido com filtros típicos do bot: `q`, `kind`).
- **M-03**: p95 da busca fuzzy (`GET /v1/categories?q=<termo>`) ≤ 150 ms em produção.
- **M-04**: 100% das mutações (create/update/delete/clone) registradas no `category_audit_log` em mesma transação.
- **M-05**: 100% das mutações publicadas no `outbox` em mesma transação, com unicidade por `event_id`.
- **M-06**: 0 incidências de delete acidental de árvore com filhos ativos (gate `409 Conflict` validado em integration tests).
- **M-07**: Cobertura de testes ≥ 80% no domínio e ≥ 70% nos use cases (mesma régua de `internal/billing`).

## Histórias de Usuário

- **US-01** — Onboarding do usuário final
  Como **usuário recém-cadastrado**, quero **encontrar uma taxonomia padrão pronta** ao começar a usar o bot WhatsApp, para que **eu consiga categorizar minhas despesas imediatamente sem ter que criar nada do zero**.

- **US-02** — Personalização por categoria de usuário
  Como **usuário ativo**, quero **criar minhas próprias categorias e subcategorias** (ex.: "Hobby de Drone" → "Bateria", "Hélice"), para que **a taxonomia reflita meu orçamento real**.

- **US-03** — Customização do seed via clone
  Como **usuário ativo**, quero **clonar uma categoria de sistema** ("Mercado") e renomeá-la na minha visão ("Supermercado"), para que **eu não veja duplicações nem perca a referência ao item de sistema**.

- **US-04** — Busca por nome no bot WhatsApp
  Como **usuário do bot WhatsApp**, quero **enviar "gastei 50 com alug"** e o sistema **encontrar a categoria "Aluguel"** automaticamente, para que **a entrada do lançamento seja conversacional**.

- **US-05** — Soft-delete seguro
  Como **usuário ativo**, quero **deletar uma categoria** sem perder o histórico de lançamentos já categorizados nela, e quero **ser bloqueado** se houver subcategorias ativas, para **eu evitar perder uma subárvore inteira por engano**.

- **US-06** — Listagem hierárquica
  Como **usuário ativo**, quero **listar minhas categorias agrupadas pelo pai**, para **eu ter uma visão organizada das despesas vs receitas com drill-down em subcategorias**.

- **US-07** — Imutabilidade do seed
  Como **operador do sistema**, quero **garantir que o seed de sistema não possa ser alterado por nenhum usuário via API**, para que **a taxonomia padrão permaneça consistente entre todos os usuários**.

- **US-08** — Auditoria
  Como **operador do sistema**, quero **rastrear quem alterou qual categoria, quando e por qual canal (WhatsApp, API direta, seed)**, para **eu investigar uso indevido ou erros de UX do bot**.

- **US-09** — Consumo por módulos futuros
  Como **desenvolvedor de módulos consumidores (transactions, budget)**, quero **consumir eventos `categories.v1.*` do outbox** com idempotência por `event_id`, para que **eu construa projeções e reações sem polling no banco**.

## Funcionalidades Core

### F-01 — CRUD de Categorias por Usuário
Criar, atualizar, listar, obter e soft-deletar categorias de receita ou despesa pertencentes ao usuário autenticado. Toda mutação dispara registro em `category_audit_log` e publicação de evento no outbox dentro da mesma transação.

### F-02 — Hierarquia em Dois Níveis (Adjacency List)
Categoria pode ter `parent_id` referenciando outra categoria do mesmo `kind` e do mesmo `user_id` (ou de sistema, no caso de subcategoria de sistema). Profundidade máxima é 2: domínio rejeita tentativa de criar subcategoria sob uma subcategoria.

### F-03 — Discriminador `kind` (income/expense) com Herança
Toda categoria raiz declara `kind` ∈ {`income`, `expense`}. Subcategorias herdam o `kind` do pai e não podem divergir. Constraint validada no domínio e no schema (`CHECK`).

### F-04 — Seed Global Imutável
Categorias de sistema (`user_id IS NULL`) são criadas via migration SQL idempotente (`INSERT ... ON CONFLICT DO NOTHING`) e são imutáveis via API. Listadas para todos os usuários como prefixo "padrão" da experiência.

### F-05 — Clone de Categoria de Sistema
Endpoint `POST /v1/categories/{id}/clone` copia uma categoria de sistema (e suas subcategorias) para o `user_id` autenticado. A cópia é editável, e mantém referência `cloned_from_category_id` para rastreabilidade. Listagem default oculta a categoria de sistema quando há clone correspondente no mesmo usuário.

### F-06 — Soft-Delete com Proteção de Árvore
Deletar uma categoria com `deleted_at` preserva o registro. Tentativa de deletar categoria com **subcategorias ativas** retorna `409 Conflict` listando os filhos. Lançamentos vinculados (módulo futuro) continuam apontando para a categoria deletada (categoria "arqueológica").

### F-07 — Busca Fuzzy por Nome
Endpoint `GET /v1/categories?q=<termo>` retorna resultados ordenados por relevância, priorizando categorias do usuário sobre categorias de sistema. Usa `pg_trgm` para tolerância a erros ortográficos (ex.: "alug" → "Aluguel"). Mínimo 1 caractere.

### F-08 — Audit Log Dedicado
Tabela `category_audit_log` registra `actor_user_id`, `action` (`created`|`updated`|`deleted`|`cloned`), `before` (JSON), `after` (JSON), `source` (`whatsapp`|`api`|`seed`), `occurred_at`. Gravação em mesma transação da mutação.

### F-09 — Eventos de Domínio via Outbox
Eventos `categories.v1.category_created`, `categories.v1.category_updated`, `categories.v1.category_deleted`, `categories.v1.category_cloned` publicados via `outbox.Publisher` com `event_id` único (UUIDv7) e idempotência garantida por UNIQUE constraint.

### F-10 — Autorização Multi-tenant
Usuário só acessa categorias com `user_id` igual ao seu ou categorias de sistema (read-only). Tentativa de update/delete em categoria de outro usuário ou de sistema retorna `403 Forbidden`.

## Requisitos Funcionais

### CRUD e Domínio
- **RF-01**: O sistema DEVE expor `POST /v1/categories` aceitando `name` (1–80 chars unicode, trim de espaços nas pontas, colapso de espaços internos múltiplos, rejeitando `\n`, `\r`, `\t` e caracteres de controle; emojis, acentos e dígitos são permitidos), `kind` ∈ {`income`, `expense`}, `description` (opcional, 0–240 chars, mesma regra de caracteres do `name` exceto que permite múltiplos espaços), `parent_id` (UUID opcional). Retorna `201 Created` com o agregado completo. Categorias raiz cujo `name` colidir com categoria de sistema ativa do mesmo `kind` recebem `409 Conflict` com código `name_matches_system_category` (RF-10a).
- **RF-02**: O sistema DEVE expor `PATCH /v1/categories/{id}` aceitando atualizações parciais de `name` e `description`. `kind` e `parent_id` são **imutáveis após criação** (reorganizar a árvore requer delete + create). Retorna `200 OK` ou `404 Not Found`. Tentativa de alterar `kind` ou `parent_id` retorna `422 Unprocessable Entity` com código `immutable_field`.
- **RF-03**: O sistema DEVE expor `DELETE /v1/categories/{id}` aplicando soft-delete (`deleted_at = NOW()`). Retorna `204 No Content` em sucesso ou `409 Conflict` se houver subcategorias ativas (response inclui lista de filhos bloqueantes).
- **RF-04**: O sistema DEVE expor `GET /v1/categories/{id}` retornando a categoria com seus filhos diretos (`subcategories: []`). Retorna `200 OK` ou `404 Not Found`.
- **RF-05**: O sistema DEVE expor `GET /v1/categories` com filtros opcionais `q` (busca fuzzy), `kind` (`income`|`expense`), `parent_id` (UUID|`null` para raízes), `include_system` (boolean, default `true`), `include_deleted` (boolean, default `false`). Ordenação padrão: **alfabética A-Z, locale `pt-BR`** (`COLLATE "pt-BR"` ou equivalente). Quando `q` está presente, ordenação é por relevância (RF-20). Paginação cursor-based (default 50, máx 200). Retorna `200 OK`.
- **RF-05a**: Quando `q` está presente, a resposta DEVE retornar no máximo **5 resultados** por padrão (`page_size=5`) e incluir o campo `has_more: boolean` indicando se existem matches adicionais não retornados. Bot WhatsApp utiliza essa convenção para listas curtas em chat. `page_size` pode ser elevado até 200 via querystring (uso administrativo).
- **RF-06**: O sistema DEVE expor `POST /v1/categories/{id}/clone` para clonar **apenas a categoria-alvo** (não as subcategorias) para o `user_id` autenticado. Retorna `201 Created` com o clone criado e `cloned_from_category_id` preenchido. Subcategorias do clone são criadas separadamente pelo usuário via RF-01. Se a origem não for de sistema, retorna `404 Not Found`. **Idempotência**: se o usuário já possui um clone ativo da mesma origem (UNIQUE `(user_id, cloned_from_category_id) WHERE deleted_at IS NULL`), retorna `200 OK` com o clone existente (chamada idempotente, sem criar novo recurso). Se o clone anterior foi soft-deletado, novo clone PODE ser criado.

### Hierarquia e Validações de Domínio
- **RF-07**: O domínio DEVE rejeitar criação de categoria com `parent_id` apontando para uma categoria que já é subcategoria (profundidade > 2). Erro `422 Unprocessable Entity` com código `invalid_depth`.
- **RF-08**: O domínio DEVE rejeitar criação de subcategoria cujo `kind` divirja do pai. Erro `422` com código `kind_mismatch`. Constraint SQL adicional `CHECK (parent_id IS NULL OR kind = (SELECT kind FROM categories WHERE id = parent_id))` reforça invariante.
- **RF-09**: O domínio DEVE rejeitar `parent_id` que aponte para categoria de outro `user_id` (exceto categoria de sistema sendo clonada via RF-06). Erro `422` com código `parent_not_accessible`.
- **RF-10**: O domínio DEVE rejeitar nome duplicado dentro do mesmo `(user_id, parent_id, kind)`, comparando após `unaccent()` + `LOWER()` (insensível a acentos e caixa — "Mercado", "mercado", "MERCADO", "Mércado" são equivalentes). Validação por UNIQUE constraint funcional + verificação prévia. Erro `409` com código `duplicate_name`.
- **RF-10a**: O domínio DEVE rejeitar **criação de categoria raiz pelo usuário** cujo `name` (após `unaccent()` + `LOWER()`) colida com **categoria de sistema ativa do mesmo `kind`** (ex.: usuário tenta criar "Mercado" enquanto existe categoria de sistema "Mercado"). Resposta: `409 Conflict` com código `name_matches_system_category` e payload `{"system_category_id": "<uuid>"}` para que o bot sugira clone via RF-06.
- **RF-11**: O domínio DEVE rejeitar criação de categoria com ciclo (auto-referência direta ou indireta), embora a hierarquia=2 já previna isso.
- **RF-11a**: O sistema DEVE impor cotas por usuário: máximo de **100 categorias raiz** (excluindo deleted_at) e **200 subcategorias** (excluindo deleted_at) por `user_id`. Exceder retorna `422 Unprocessable Entity` com código `quota_exceeded` e payload `{"limit": <n>, "current": <n>, "kind_of_quota": "root"|"subcategory"}`. Cotas servem como gate anti-abuso e UX (sustenta seleção numerada em chat).

### Seed e Imutabilidade
- **RF-12**: A migration `migrations/00XX_seed_system_categories.up.sql` DEVE criar categorias de sistema (`user_id = NULL`) com `INSERT ... ON CONFLICT (id) DO NOTHING`. IDs do seed são determinísticos (UUIDs versionados em git).
- **RF-13**: O sistema DEVE rejeitar qualquer mutação (UPDATE/DELETE/CLONE como destino) sobre categoria com `user_id IS NULL` via API. Erro `403 Forbidden` com código `system_category_immutable`.
- **RF-14**: A listagem default (`include_system=true`) DEVE retornar a união de (categorias do usuário, ignorando `deleted_at`) ∪ (categorias de sistema sem clone **ativo** correspondente do mesmo usuário). Quando o usuário tem um clone com `deleted_at IS NULL`, a categoria de sistema correspondente fica oculta. **Quando o usuário soft-deleta o clone (`deleted_at IS NOT NULL`), a categoria de sistema VOLTA a aparecer automaticamente** na listagem default — o usuário retorna ao padrão sem ação adicional.
- **RF-14c**: Nova categoria de sistema introduzida em release futura (via migration que estende o seed) DEVE aparecer **automaticamente** em todas as listagens default de usuários ativos e novos, sem opt-in ou opt-out. Não há flag `is_new` ou mecanismo de "dismiss" no MVP.
- **RF-14a**: Versionamento do seed é **append-only**. Quando uma categoria de sistema mudar de nome (ex.: rebrand de "Mercado" para "Supermercado"), a nova categoria DEVE ser criada com `id` novo e a antiga marcada com `deprecated_at = NOW()` em migration de evolução do seed. Categorias com `deprecated_at IS NOT NULL` NÃO aparecem em listagens default, mas permanecem na tabela para que clones existentes (`cloned_from_category_id`) preservem rastreabilidade. Nenhum nome ou ID de seed é alterado in-place.
- **RF-14b**: A coluna `deprecated_at` (sistema) e `deleted_at` (sistema/usuário) são independentes: deprecated marca obsolescência editorial do seed; deleted_at marca soft-delete operacional. Categorias de sistema só recebem `deprecated_at`; nunca `deleted_at` via API.

### Soft-Delete
- **RF-15**: A coluna `deleted_at` DEVE ser preenchida em delete; o registro permanece na tabela.
- **RF-16**: Categoria com `deleted_at IS NOT NULL` NÃO DEVE aparecer em listagens default. Visível apenas via `include_deleted=true`.
- **RF-17**: Tentativa de delete de categoria com filhos ativos (`deleted_at IS NULL`) DEVE retornar `409 Conflict` com payload `{"error":"has_active_children","children":[{"id","name"}, ...]}`.
- **RF-18**: Não há endpoint público de "restore"; recuperação requer operação administrativa via migration manual (fora do MVP).

### Busca Fuzzy
- **RF-19**: A busca fuzzy DEVE ser **case-insensitive E accent-insensitive**: "agua", "AGUA", "água", "Água", "ÁGUA" todos encontram a categoria "Água". Mecanismo concreto (`unaccent()` + `LOWER()` em coluna gerada, índice GIN com `pg_trgm`, ou outro) é definido na techspec. Comportamento de negócio é não-negociável.
- **RF-20**: Resultado da busca DEVE priorizar, em ordem: (a) match exato sobre fuzzy (após normalização accent/case), (b) categorias do usuário sobre sistema, (c) categoria raiz sobre subcategoria. Tie-break alfabético PT-BR.
- **RF-21**: Busca DEVE aceitar `q` com mínimo 1 caractere após trim; `q` vazio ou ausente retorna a listagem default (RF-05, ordem alfabética).

### Audit Log e Eventos
- **RF-22**: Toda **mutação** (create/update/delete/clone) DEVE inserir uma linha em `category_audit_log` na mesma transação. **Leituras (GET) NÃO são auditadas**. Schema: `id UUID PK`, `category_id UUID`, `actor_user_id UUID NOT NULL`, `action TEXT NOT NULL CHECK (action IN ('created','updated','deleted','cloned'))`, `before JSONB`, `after JSONB`, `source TEXT NOT NULL CHECK (source IN ('whatsapp','api','seed'))`, `occurred_at TIMESTAMPTZ NOT NULL`.
- **RF-22a**: Os campos `before` e `after` DEVEM conter **snapshot completo** do agregado Category (todos os atributos persistidos), nunca diff. Convenção por ação: `created` → `before=NULL`, `after=snapshot`; `updated` → `before=snapshot anterior`, `after=snapshot novo`; `deleted` → `before=snapshot ativo`, `after=snapshot com deleted_at preenchido`; `cloned` → `before=NULL`, `after=snapshot do clone`.
- **RF-22b**: **Audit log é de uso INTERNO** (forensics, suporte, debug). NÃO há endpoint público que exponha o histórico ao usuário final no MVP. Bot WhatsApp não consome `category_audit_log`. Acesso ao audit log se dá via console interno ou consulta direta ao Postgres por engenharia/suporte.
- **RF-22c**: **Retenção do audit log = 18 meses**. Linhas com `occurred_at < NOW() - INTERVAL '18 months'` DEVEM ser elegíveis para purga via job batch dedicado. A escolha (LGPD + 1 ciclo fiscal anual completo) atende: (a) prazo mínimo de logs de auditoria recomendado pela LGPD, (b) cobertura de pelo menos uma declaração anual de Imposto de Renda e dois dezembros completos, (c) volume gerenciável sem particionamento prematuro. Mecanismo de purga (job recorrente, particionamento por mês, etc.) é definido na techspec.
- **RF-22d**: O ciclo de vida de `outbox_events` é **independente** do `category_audit_log` e gerenciado pela plataforma (housekeeping job já existente, índice `outbox_events_housekeeping_published_idx`). Este PRD não define política específica de purga do outbox para categorias.
- **RF-23**: Toda mutação DEVE publicar evento `categories.category.{action}` (`created`|`updated`|`deleted`|`cloned`) na tabela `outbox_events` via `outbox.Publisher` na mesma transação. Namespace segue a convenção já consolidada por `billing.subscription.*` (sem sufixo `v1`). Payload inclui `category_id`, `user_id`, `before`, `after`, `actor_user_id`, `source`, `occurred_at`. `aggregate_type = "category"`, `aggregate_id = category_id`.
- **RF-24**: Eventos DEVEM ter `event_id` UUIDv7 único, gerado no domínio. UNIQUE constraint do PK de `outbox_events` garante idempotência mesmo em caso de replay.
- **RF-25**: Cada requisição DEVE carregar o canal de origem (`source` ∈ {`whatsapp`, `api`, `seed`}). O mecanismo de propagação (header, contexto da requisição, claim) é definido na techspec. Quando ausente, default é `api` (apenas chamadas administrativas diretas chegam sem `source` explícito). O valor é gravado em `category_audit_log.source` (RF-22) e no payload do evento (RF-23) para suportar análises de adoção por canal.

### Autenticação, Autorização e Identidade do Ator
- **RF-26**: Todos os endpoints DEVEM operar com `user_id` previamente resolvido pelo gateway interno (mesmo padrão usado entre bot WhatsApp e API do mecontrola). O mecanismo concreto de propagação (header, contexto, claim) é definido na techspec. Requisição sem `user_id` retorna `401 Unauthorized`.
- **RF-27**: Usuário só acessa categorias com `user_id` igual ao seu OU categorias de sistema (`user_id IS NULL`). Acesso a categoria de outro usuário retorna `404 Not Found` (não vazar existência).
- **RF-28**: Mutações em categoria de outro usuário retornam `404 Not Found`. Mutações em categoria de sistema retornam `403 Forbidden` com código `system_category_immutable`.
- **RF-29**: Idempotência crítica do MVP é a do outbox (event_id UUIDv7 único por mutação). Idempotência HTTP no nível de request (`Idempotency-Key`) NÃO é requisito do MVP: o bot WhatsApp é o gateway responsável por controlar retries, e o efeito colateral (publicação duplicada em consumers) já é absorvido pela unicidade do `event_id`. Endpoint de clone (RF-06) é idempotente por natureza via UNIQUE (`user_id`, `cloned_from_category_id`).

### Seed — Conteúdo Mínimo (Curadoria MVP)
- **RF-30**: O seed DEVE conter as categorias raiz e subcategorias listadas abaixo (PT-BR), exatamente como nomeadas. Alteração de seed exige nova migration.

**DESPESAS** (`kind=expense`):
- Moradia → Aluguel, Financiamento Imobiliário, Condomínio, IPTU, Energia, Água, Gás, Internet, Telefone Fixo, Manutenção da Casa, Móveis e Decoração
- Alimentação → Supermercado, Feira, Padaria, Açougue, Restaurantes, Delivery, Lanches
- Transporte → Combustível, Transporte Público, Aplicativos de Transporte, Estacionamento, Pedágio, Manutenção Veicular, Lavagem Automotiva, IPVA, Licenciamento, Seguro Veicular, Multas
- Saúde → Plano de Saúde, Consultas Médicas, Exames, Medicamentos, Odontologia, Terapia, Academia, Suplementos
- Educação → Mensalidade Escolar, Faculdade, Cursos, Livros, Material Escolar, Idiomas
- Lazer → Cinema, Streaming, Viagens, Hobbies, Bares, Eventos, Jogos
- Vestuário → Roupas, Calçados, Acessórios
- Cuidados Pessoais → Cabeleireiro, Estética, Cosméticos, Perfumaria
- Pets → Ração, Veterinário, Banho e Tosa, Acessórios Pet
- Família → Filhos, Mesada, Presentes Familiares
- Tarifas e Impostos → Tarifas Bancárias, Anuidade de Cartão, Imposto de Renda, Outros Impostos
- Dívidas → Empréstimo Pessoal, Juros de Cartão, Cheque Especial, Financiamentos Diversos
- Investimentos e Poupança → Reserva de Emergência, Aportes em Investimentos, Previdência Privada
- Doações → Caridade, Igreja, ONGs
- Outros → Diversos, Sem Categoria

**RECEITAS** (`kind=income`):
- Salário → Salário, Décimo Terceiro, Férias, PLR e Bônus, Vale-Alimentação, Vale-Refeição
- Renda Variável → Freelance, Trabalho Extra, Consultoria
- Investimentos → Rendimentos, Dividendos, Juros, Resgates
- Aluguel Recebido (sem subcategorias)
- Restituições e Estornos → Restituição de IR, Estorno, Cashback, Reembolsos
- Presentes Recebidos → Presentes em Dinheiro, Mesada Recebida
- Vendas → Vendas Diversas, Marketplace
- Outras Receitas → Outros

### Observabilidade
- **RF-31**: O módulo DEVE expor métricas Prometheus: `categories_create_total{kind,source}`, `categories_update_total{kind,source}`, `categories_delete_total{kind,source,outcome}`, `categories_clone_total{kind}`, `categories_list_latency_seconds{outcome}`, `categories_search_latency_seconds{outcome}`. Outcome ∈ {`ok`, `blocked_has_children`, `not_found`, `forbidden`, `validation_error`}.
- **RF-32**: Todo log de mutação DEVE conter `category_id`, `actor_user_id`, `source`, `event_id`, `action` (campos estruturados).
- **RF-33**: Use cases DEVEM ser envolvidos por span OTEL (mesma esteira de `internal/billing`).

### Wiring e Estrutura
- **RF-34**: O módulo DEVE seguir o "Padrão Obrigatório de Módulo" (`AGENTS.md`): `application/usecases`, `domain/{entities,valueobjects,repositories}`, `infrastructure/{http/{server,handlers,middleware},repositories/postgres,messaging/database/consumers}`, `module.go` único ponto de wiring.
- **RF-35**: Wiring DEVE registrar handlers em `cmd/api/api.go`, jobs (se houver consumer) em `cmd/worker/worker.go`. Não introduzir `init()` (R0 da skill `go-implementation`).

## Restrições Técnicas de Alto Nível

- **RT-01**: Stack Go conforme `go.mod`. Sem `init()`, sem `panic` em produção, `context.Context` em fronteiras de IO (R0–R7 da skill `go-implementation`).
- **RT-02**: Padrão Obrigatório de Módulo (`AGENTS.md`) — não inventar wiring, routers, jobs, consumers ou adapters fora da estrutura canônica.
- **RT-03**: Postgres é o único storage. Migrations em `migrations/` (mesma esteira de billing/identity) com `*_integration_test.go` validando schema e seed.
- **RT-04**: Outbox via `outbox.Publisher` da plataforma (tabela `outbox_events` já existente, criada por `migrations/000001_create_platform_outbox_events.up.sql`). Idempotência via PK UUID (= `event_id`). Padrão de `event_type` é `<contexto>.<aggregate>.<action>` (mesmo namespace de `billing.subscription.*`).
- **RT-05**: Autenticação por **gateway interno confiável** (mesma esteira hoje utilizada para integrar bot WhatsApp ↔ API). MVP NÃO introduz JWT externo; `user_id` chega pré-resolvido pelo gateway. A reuso vs criação de middleware é definido na techspec.
- **RT-06**: Sem abstração de tempo: usar `time.Now().UTC()` no ponto de uso (memory `feedback_no_time_abstraction`).
- **RT-07**: Sem `var _ Interface = (*Type)(nil)` (memory `feedback_no_interface_assertion`).
- **RT-08**: Cobertura de testes: unitários em `domain/` e `application/usecases/`; integration tests em `infrastructure/repositories/postgres/` com Postgres real (mesma esteira de billing).
- **RT-09**: Canal de consumo do MVP é WhatsApp; sem cor/ícone/branding visual no agregado.
- **RT-10**: Idempotência HTTP é obrigação de quem chama (header `Idempotency-Key`); reaproveitar middleware existente (se houver) ou criar local minimalista.
- **RT-11**: Performance: cardinalidade alvo ~30–50 categorias de sistema + ~10–30 por usuário. Índices obrigatórios: `(user_id, kind, parent_id)`, `(user_id, deleted_at)`, GIN sobre `name` para fuzzy.
- **RT-12**: Compliance: dados não são sensíveis (taxonomia pura); mas `actor_user_id` no audit log é PII e segue política de retenção do projeto (default 12 meses, a confirmar com técnico).
- **RT-13**: API REST sob `/v1/categories` (versionada). Respostas em JSON. **Códigos de erro em EN snake_case** (`duplicate_name`, `kind_mismatch`, `quota_exceeded`, `name_matches_system_category`, `immutable_field`, `system_category_immutable`, `invalid_depth`, `parent_not_accessible`, `clone_already_exists`, `has_active_children`). **Mensagens em PT-BR** human-readable repassáveis ao usuário do bot WhatsApp. Estrutura: `{"error": "code_em_en", "message": "mensagem em pt-BR", "details": {...}}`.
- **RT-14**: Idioma do produto: **PT-BR exclusivo no MVP**. Não há campo `locale` no agregado nem suporte a i18n. Lançamentos futuros assumirão `BRL` por convenção do domínio (definido no módulo de transactions).

## Fora de Escopo

- **OUT-01**: Front-end gráfico próprio (web/mobile). Canal MVP é exclusivamente WhatsApp.
- **OUT-02**: Cor, ícone, ordenação manual ou metadados visuais na entidade Category.
- **OUT-03**: Hierarquia com profundidade > 2 níveis. Não há suporte a "subsubcategoria" mesmo em casos limítrofes.
- **OUT-04**: Read-model materializado (`category_tree_view` projetado por consumer). Mantida como porta de saída para o futuro se relatórios pesados surgirem.
- **OUT-05**: Reatribuição automática de lançamentos durante delete. Módulo de lançamentos ainda não existe — quando existir, lançamentos manterão FK estável para a categoria mesmo soft-deletada.
- **OUT-06**: Versionamento completo do agregado (event sourcing). Audit log dedicado em tabela é suficiente para o MVP.
- **OUT-07**: Endpoint de "restore" público para categoria soft-deletada. Recuperação só via migration administrativa.
- **OUT-08**: Endpoint de "diff/sync" entre categoria clonada e versão de sistema atualizada (drift entre clone e seed evoluído).
- **OUT-09**: Categorias compartilhadas entre usuários (família compartilhando taxonomia). Cada `user_id` tem namespace isolado.
- **OUT-10**: Hard-delete administrativo via API. Limpeza, se necessária, vem por job batch separado em release futura.
- **OUT-11**: Import/export de categorias (CSV, JSON). Customização via API individual no MVP.
- **OUT-12**: Suporte multi-moeda na categoria. Moeda é atributo do lançamento, não da categoria.
- **OUT-13**: Idempotência HTTP no nível de request (`Idempotency-Key` por chamada). A idempotência do MVP vive na camada de evento (`event_id` UUIDv7 + UNIQUE em `outbox_events`) e na unicidade de clone (`UNIQUE (user_id, cloned_from_category_id)`). Retry no nível HTTP é responsabilidade do bot WhatsApp.
- **OUT-14**: Autenticação JWT ou OAuth público. MVP usa apenas gateway interno entre bot WhatsApp e API. Exposição pública da API com auth completo é evolução de release futura.
- **OUT-15**: Endpoint "diff/sync" entre clone de usuário e categoria de sistema que evoluiu. Append-only do seed (RF-14a) é a estratégia escolhida; clone mantém o ID antigo e segue válido. Evoluir nome do clone é responsabilidade explícita do usuário.

## Confronto com o Codebase (auditoria de premissas — v2)

A versão v2 deste PRD foi refinada após inspeção direta de `internal/identity/`, `internal/billing/` e `migrations/`. Os achados abaixo confirmam ou rejeitam premissas de v1 e estão integrados aos RFs/RTs acima:

- **A-01 (Outbox da plataforma — confirmado)**: a tabela `outbox_events` já existe em `migrations/000001_create_platform_outbox_events.up.sql`, é compartilhada por todos os contextos via coluna `aggregate_type`, e seu PK `id UUID` serve como `event_id` único. Categorias adotam essa mesma esteira (RT-04). Nenhuma migration de outbox novo é necessária.
- **A-02 (Padrão de event_type — confirmado e ajustado)**: o módulo `internal/identity` já consome eventos no padrão `<contexto>.<aggregate>.<action>` (ex.: `billing.subscription.activated`). PRD v1 propunha `categories.v1.*`; v2 ajusta para `categories.category.{created|updated|deleted|cloned}` (RF-23).
- **A-03 (Sem JWT no MVP — premissa rejeitada e substituída)**: `internal/identity` resolve usuário via `UpsertUserByWhatsApp` (entrada por número de WhatsApp). Não há middleware JWT. PRD v1 supunha JWT; v2 substitui por "gateway interno confiável" (RF-26, RT-05).
- **A-04 (Sem middleware Idempotency-Key — premissa rejeitada)**: a idempotência hoje vive no domínio (tabela `billing_processed_events` para webhooks e PK do outbox para eventos). Bot WhatsApp controla retries no lado dele. PRD v1 propunha header `Idempotency-Key`; v2 move para fora de escopo (OUT-13) sem perda funcional.
- **A-05 (Identidade do ator — confirmado)**: `actor_user_id` é UUID já resolvido pelo gateway; auditoria registra esse UUID, não o número WhatsApp em si (que vive em `internal/identity`).

## Definições Técnicas Delegadas à Techspec

Itens estritamente técnicos, sem ambiguidade de negócio, que serão decididos na techspec:

- **TS-01**: Estratégia de paginação (cursor opaco vs `(updated_at, id)`); tamanho default e máximo do `page_size`; serialização do cursor.
- **TS-02**: Particionamento de `category_audit_log` (por mês vs por user_id vs sem particionamento), e mecanismo de purga após 18 meses (job batch recorrente, política de `pg_partman`, ou outra).
- **TS-03**: Mecanismo concreto de injeção do `source` na requisição (header HTTP, claim em token interno, contexto do gateway) e do `user_id` pré-resolvido — reuso vs criação de middleware.
- **TS-04**: Parâmetros do ranking de busca fuzzy: similarity threshold mínimo de `pg_trgm`, peso relativo entre user_id/sistema/match exato, limite máximo de resultados, comportamento quando `q` está vazio vs ausente.
- **TS-05**: Geração de UUIDv7 (biblioteca, fallback para v4 se necessário) e mecanismo exato de "uniqueness" da entrada do clone (`UNIQUE (user_id, cloned_from_category_id)`).
- **TS-06**: Habilitação da extensão `pg_trgm` (via migration do módulo vs assumida como pré-instalada em ambientes).
- **TS-07**: Estratégia de transação para clone com subcategorias (single UoW vs por nível) preservando atomicidade.
- **TS-08**: Reuso da estrutura existente (`devkit-go/pkg/database/uow`, `manager`, `observability`) — confirmar wiring e DI.

## Decisões de Negócio Fechadas (v3)

Todas as questões de negócio foram resolvidas em 5 rodadas de múltipla escolha (transcript completo no histórico desta sessão). Resumo das decisões:

| Decisão | Resposta fechada | Refletida em |
| --- | --- | --- |
| Clone duplicado (mesma origem) | **200 OK idempotente** retornando o clone existente; não cria novo | RF-06 |
| Categoria de usuário homônima a categoria de sistema | **Bloquear com `409`** e sugerir clone via `system_category_id` no payload | RF-10a |
| Clone de subcategorias junto da raiz | **Não** — clone copia apenas a categoria-alvo; usuário recria filhas | RF-06 |
| Nova categoria de sistema (release futura) | **Aparece automaticamente** em todas as listagens default; sem opt-in/out | RF-14c |
| Limite por usuário | **100 categorias raiz + 200 subcategorias** ativas | RF-11a |
| Caracteres permitidos no `name` | **Unicode geral** (acentos, dígitos, emojis), trim de pontas, colapso interno, sem `\n\r\t` e controle | RF-01 |
| Categorias de sistema "Outros"/"Sem Categoria"/"Diversos" cloneáveis | **Sim**, qualquer categoria de sistema é clonável | RF-06 |
| Ciclo de vida do outbox | **Independente** do audit log; gerenciado pela plataforma | RF-22d, RT-04 |
| Reparenting (mover entre pais) | **Não permitido** — `kind` e `parent_id` imutáveis após criação | RF-02 |
| Ordenação default da listagem | **Alfabética A-Z em locale `pt-BR`**; relevância apenas quando `q` presente | RF-05 |
| Audiência MVP | **PT-BR exclusivo**; sem `locale` no agregado | RT-14, SUP-02 (legado) |
| Sistema reaparece após delete do clone | **Sim**, automaticamente quando `clone.deleted_at IS NOT NULL` | RF-14 |
| Busca case+accent insensitive | **Sim** — "agua", "AGUA", "Água" todos batem "Água" | RF-19 |
| Escopo da auditoria | **Apenas mutações**; GETs não são auditados | RF-22 |
| Histórico do audit log para usuário final | **Não no MVP** — uso exclusivamente interno (forensics/suporte) | RF-22b |
| Formato `before`/`after` no audit | **Snapshot completo** do agregado, nunca diff | RF-22a |
| Tamanhos | **name 1-80, description 0-240** | RF-01 |
| Idioma da API | **Códigos EN snake_case + mensagens PT-BR** | RT-13 |
| Curadoria do seed | **Final como RF-30** (~80 categorias) | RF-30 |
| Limite de resultados em busca | **Top 5 + flag `has_more: bool`** quando `q` presente | RF-05a |
| Retenção do audit log | **18 meses** | RF-22c |
| Versionamento do seed | **Append-only** com `deprecated_at`; clones preservam ID antigo | RF-14a, RF-14b |

## Confronto Final com Bundle de Brainstorming

Foi verificada coerência entre o PRD v3 e `docs/discoveries/brainstorms/brainstorm-crud-de-categorias-e-subcategorias-de-despesas/`:

- **Decisões D1–D12** do bundle estão todas refletidas neste PRD.
- **Scorecard A (Adjacency List)** é a única alternativa materializada como restrição arquitetural (RT-01, RT-02).
- **Riscos do brief** estão endereçados:
  - Drift de clone vs seed → coberto por RF-14a/RF-14b (append-only) + RF-22 (audit) + OUT-15 (sem sync automático).
  - Crescimento da listagem → coberto por RF-11a (quotas) + RF-05a (top 5).
  - Homônimos no bot → coberto por RF-10a (bloqueio) + RF-19/RF-20 (priorização).
  - Crescimento do audit log → coberto por RF-22c (retenção 18 meses).
  - Idempotência do outbox → coberto por RF-24 + RT-04.

## Suposições Operacionais Remanescentes (não bloqueantes)

Estas suposições NÃO requerem confirmação adicional de produto — são contratos operacionais delegáveis à técnica ou ao módulo do bot:

- **SUP-01**: O módulo do bot WhatsApp (fora deste PRD) propaga `user_id` previamente resolvido e o canal `source=whatsapp` para o módulo de categorias. Contrato técnico exato fica na techspec do bot e do gateway.
- **SUP-02**: A categoria de sistema **"Sem Categoria"** (no seed de despesas) serve de fallback quando o bot não consegue resolver intenção via RF-19/RF-20. Esse comportamento é do bot; aqui apenas garantimos a existência permanente da categoria via seed.
- **SUP-03**: A esteira de `outbox_events` da plataforma (`migrations/000001`) acomoda os novos `event_type` `categories.category.*` sem mudança de schema. Confirmação técnica fica na techspec.

## Definições Técnicas Delegadas à Techspec

Itens estritamente técnicos, sem ambiguidade de negócio, que serão decididos na techspec:

- **TS-01**: Estratégia de paginação (cursor opaco vs `(updated_at, id)`); serialização do cursor; comportamento quando filtros mudam mid-pagination.
- **TS-02**: Particionamento de `category_audit_log` (por mês vs por user_id vs sem particionamento), e mecanismo de purga após 18 meses (job batch recorrente, `pg_partman`, etc.).
- **TS-03**: Mecanismo concreto de injeção do `source` na requisição e do `user_id` pré-resolvido pelo gateway (header HTTP, claim em token interno, contexto). Reuso vs criação de middleware.
- **TS-04**: Parâmetros do ranking de busca fuzzy: similarity threshold mínimo de `pg_trgm`, índice de combinação (GIN trigram vs btree), implementação concreta de accent-insensitive (`unaccent()` em coluna gerada, expressão indexada, ou normalização no domínio).
- **TS-05**: Geração de UUIDv7 (biblioteca, fallback para v4 se necessário).
- **TS-06**: Habilitação das extensões `pg_trgm` e `unaccent` via migration do módulo vs assumidas como pré-instaladas.
- **TS-07**: Estratégia de transação para clone (single UoW) preservando atomicidade do INSERT + audit + outbox.
- **TS-08**: Reuso da estrutura existente (`devkit-go/pkg/database/uow`, `manager`, `observability`) — confirmar wiring e DI.
- **TS-09**: Implementação concreta da UNIQUE parcial `(user_id, cloned_from_category_id) WHERE deleted_at IS NULL` e seu comportamento sob race condition.
