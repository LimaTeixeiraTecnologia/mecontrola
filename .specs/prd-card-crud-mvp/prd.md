# Documento de Requisitos do Produto (PRD)

<!-- spec-version: 2 -->

<!--
Histórico de versões:
- v1 (2026-06-06): escopo MVP production-ready do módulo `internal/card` consolidando brainstorming decisório (`discoveries/brainstorm-modulo-crud-de-cartoes-de-credito-com-calculo-de-fatura/`) e discovery técnico (`discoveries/technical-modulo-card-mvp-crud-com-invoicefor/`). CRUD HTTP de cartões + função pura `InvoiceFor` de domínio. Introduz pacote genérico `internal/platform/idempotency/`. Middleware transitório `RequireUser` via header `X-User-ID` enquanto módulo de autenticação não existir.
- v2 (2026-06-08): bump pós-`prd-auth-foundation` task 9.0. O middleware transitório `RequireUser` via `X-User-ID` é substituído pelo `RequireUser` canônico de `internal/identity/infrastructure/http/server/middleware` que consome `auth.Principal` do `context.Context` injetado pelo `EstablishPrincipal` usecase. Referência: `prd-auth-foundation`. ADR-003 permanece válido; a substituição é additive e não muda o contrato HTTP do módulo `card`.
-->

## Visão Geral

O módulo `internal/card` introduz a entidade **cartão de crédito** no MeControla. O MVP cobre três responsabilidades indissociáveis:

1. **CRUD de cartões** do usuário autenticado (nome, apelido, dia de fechamento, dia de vencimento) via HTTP sob `/api/v1/cards`.
2. **Função pura `InvoiceFor`** que, dada uma compra em data `D` e o ciclo de um cartão, retorna determinísticamente o par `(closing_date, due_date)` da fatura na qual a compra cai. A função é exposta como porta interna Go (`CardLookup`) para o futuro módulo de transações e como endpoint público `GET /api/v1/cards/{id}/invoices?for=<date>` para o front consultar a fatura prevista para uma data.
3. **Pacote genérico de idempotência** (`internal/platform/idempotency/`) consumido pelos endpoints de mutação do `card`, com tabela compartilhada `idempotency_keys` reusável por `billing` e `identity` em fases futuras.

O módulo **não** persiste qualquer dado sensível de cartão de pagamento (PAN, CVV/CVC, trilha magnética, PIN) — `mecontrola` é aplicação **não-PCI**. A regra de fatura é deliberadamente posicionada no MVP porque o módulo de transações já planejado a exige; sem ela, qualquer transação inserida agora teria que ser recalculada quando a regra fosse definida, comprometendo a integridade do histórico financeiro.

A direção arquitetural foi consolidada em duas skills anteriores (`decision-brainstorming` + `technical-discovery-production`) e é **inegociável** neste PRD. Detalhes de algoritmo, schema, ADRs, plano de testes, observabilidade e segurança constam no discovery técnico que serve de insumo direto para a especificação técnica (`create-technical-specification`) deste PRD.

### Volumetria-alvo e SLO do MVP

Todas as metas de capacidade, dimensionamento e SLO presumem o cenário de proteção abaixo:

- **Cartões persistidos**: até 300.000 registros (100.000 usuários × 3 cartões médios).
- **Carga de criação**: até 10.000 `POST /api/v1/cards` por dia; pico ~1.000 RPS em janela de migração de usuários.
- **Carga de leitura**: 50 RPS médio de `GET /api/v1/cards` (listagem) e até 200 RPS de `GET /api/v1/cards/{id}/invoices?for=<date>` quando o módulo de transações estiver ativo.
- **Idempotência**: ~10.000 chaves/dia × TTL 24h → ~10.000 linhas em estado estacionário em `idempotency_keys`.
- **Disponibilidade**: SLO mensal de 99,5% por endpoint público do módulo (≈3h36min de error budget/mês).
- **Latência**: p99 < 300 ms em CRUD (POST/GET/PUT/DELETE) e p99 < 10 ms em `InvoiceFor` (porta interna ou endpoint público).

Crescimento além desse teto exige revisão deste PRD.

## Objetivos

- **OBJ-01**: viabilizar o módulo de transações eliminando o bloqueio "em qual fatura uma compra cai", entregando a regra de ciclo como função pura, determinística e testada antes da existência das transações.
- **OBJ-02**: oferecer CRUD HTTP enxuto sob `/api/v1/cards` consumível por front, agentes de IA e WhatsApp, sem expor decisões de domínio implícitas.
- **OBJ-03**: garantir aderência total ao Padrão Obrigatório de Módulo do `AGENTS.md` (R0–R7) e ao layout de bounded contexts já vigente em `internal/identity` e `internal/billing`.
- **OBJ-04**: introduzir o pacote genérico `internal/platform/idempotency/` reutilizável, com escopo de uso inicial restrito ao `card` e migração futura de `billing`/`identity` planejada como dívida controlada.
- **OBJ-05**: blindar a regra financeira contra edge cases de calendário (mês com 28/29/30/31 dias, virada de ano, `due == closing`, `due > closing`, `due < closing`) por cobertura de testes table-driven + property-based.
- **OBJ-06**: preservar **histórico financeiro imutável** — alterações em `closing_day`/`due_day` de um cartão **não** recalculam faturas de transações já persistidas.

### Métricas de Sucesso

- **M-01**: 100% das criações, edições e exclusões de cartão via POST/PUT/DELETE consomem o middleware de idempotência; retentativas com a mesma `Idempotency-Key` produzem 0 (zero) duplicações no DB.
- **M-02**: p99 de `POST /api/v1/cards` ≤ 300 ms, medido na volumetria-alvo (até 1.000 RPS em janela).
- **M-03**: p99 de `GET /api/v1/cards` (lista paginada) ≤ 50 ms para 100 itens por página.
- **M-04**: p99 de `InvoiceFor` ≤ 10 ms, medido como overhead total no handler `GET /api/v1/cards/{id}/invoices?for=<date>`.
- **M-05**: cobertura de testes do domínio (`domain/services/billing_cycle.go`) ≥ 95% line coverage, com **mínimo 50 fixtures table-driven** + property-based test rodando `quick.Config{MaxCount: 10000}` sem falhas.
- **M-06**: 0 ocorrência de PAN, CVV, CVC, trilha magnética ou PIN no schema, nos logs, nos spans, nos DTOs ou no payload de qualquer endpoint do módulo.
- **M-07**: 0 ocorrência de `name` ou `nickname` em logs estruturados ou spans OTel, validado por teste de regressão que inspeciona output do logger configurado.
- **M-08**: SLO mensal de disponibilidade de 99,5% por endpoint público do módulo, acompanhado em dashboard "Card Module" no Grafana.
- **M-09**: 100% dos endpoints públicos descritos em `internal/card/infrastructure/http/server/openapi.yaml` (OpenAPI 3.1), publicado como artifact de CI e validado por contract tests via golden files em `testdata/`.
- **M-10**: 100% das migrations (`0010_create_platform_idempotency_keys` e `0011_create_card_cards`) possuem `down` que preserva dados via rename (jamais `DROP TABLE` direto), com runbook `docs/runbooks/card-rollback.md` documentando o procedimento.
- **M-11**: 0 ocorrência de `init()` ou `panic` em código de produção do módulo, validado por `go vet` + linter customizado no CI.

## Histórias de Usuário

- **US-01 — Cadastro de cartão**
  Como usuário do MeControla, quero cadastrar um cartão informando nome, apelido, dia de fechamento e dia de vencimento, para que minhas compras futuras sejam atribuídas à fatura correta automaticamente.

- **US-02 — Consulta da fatura de uma compra**
  Como usuário, quero saber em qual fatura uma compra realizada em uma data específica cairá, para planejar meu orçamento mensal sem esperar o módulo de transações ser entregue.

- **US-03 — Listagem e gestão dos meus cartões**
  Como usuário, quero listar, editar e remover meus cartões, para manter o registro atualizado quando trocar de banco, mudar apelido ou cancelar um cartão.

- **US-04 — Apelido único entre cartões ativos**
  Como usuário, quero impedir que dois cartões ativos meus tenham o mesmo apelido, para evitar ambiguidade na UI e em mensagens de WhatsApp.

- **US-05 — Idempotência em retentativas de rede**
  Como cliente HTTP (app mobile, WhatsApp bot, integração), quero reenviar uma criação/edição/exclusão de cartão com a mesma `Idempotency-Key` sem duplicar a operação, para tolerar falhas de rede sem corrupção de dados.

- **US-06 — Histórico financeiro preservado**
  Como usuário, quero que uma alteração no ciclo de um cartão **não** mexa em transações antigas, para que minha visão histórica do orçamento permaneça consistente e auditável.

- **US-07 — Porta interna para o módulo de transações**
  Como módulo `internal/transaction` (futuro), quero importar `CardLookup.InvoiceFor` como porta Go e consultar o ciclo de um cartão sem precisar de chamada HTTP interna, para resolver a fatura de uma compra com latência sub-milissegundo durante a criação da transação.

## Funcionalidades Core

### F-01 — Bounded context `internal/card`

Módulo novo seguindo o **Padrão Obrigatório de Módulo** do `AGENTS.md`, isomórfico a `internal/identity` e `internal/billing`. Construtor `NewCardModule(cfg *configs.Config, o11y observability.Observability, mgr manager.Manager) CardModule` retorna struct exportada com `CardRouter`, `RepositoryFactory` e portas públicas (`CardLookup`). Registrado em `cmd/server/server.go` via `srv.RegisterRouters(cardModule.CardRouter)` apenas quando o router não for `nil`.

### F-02 — CRUD HTTP sob `/api/v1/cards`

Seis endpoints estabilizados:

- `POST /api/v1/cards` — cria um cartão para o usuário autenticado. Consome `X-User-ID` + `Idempotency-Key`.
- `GET /api/v1/cards` — lista cartões ativos do usuário com paginação cursor opaco base64 (`?cursor=<base64>&limit=<n>`) ordenando por `created_at DESC, id DESC`.
- `GET /api/v1/cards/{id}` — obtém um cartão por ID. Responde 404 se soft-deleted.
- `PUT /api/v1/cards/{id}` — atualiza nome, apelido, `closing_day` e/ou `due_day`. Consome `Idempotency-Key`.
- `DELETE /api/v1/cards/{id}` — soft-delete (preenche `deleted_at`). Consome `Idempotency-Key`.
- `GET /api/v1/cards/{id}/invoices?for=<date>` — retorna `{closing_date, due_date}` da fatura na qual uma compra na data `for` cairia.

Todos os endpoints exigem header `X-User-ID` válido (UUID v4) via middleware `RequireUser`. Endpoints de mutação exigem header `Idempotency-Key` (qualquer string ASCII 1–128 chars).

### F-03 — Função pura `InvoiceFor` em domínio

Domain service `BillingCycle.InvoiceFor(purchase time.Time, cycle BillingCycle, tz *time.Location) Invoice` em `internal/card/domain/services/billing_cycle.go`. Stateless, sem IO, sem dependência de tempo externa (R6.7 do AGENTS.md proíbe `clock.Clock`; instante de compra chega como argumento). Reutilizável por:

- Endpoint público `GET /cards/{id}/invoices?for=<date>`.
- Porta interna `CardLookup.InvoiceFor` consumida pelo futuro módulo de transações.
- Relatórios, jobs e qualquer outro consumidor Go do `mecontrola`.

### F-04 — Pacote genérico `internal/platform/idempotency/`

Novo pacote de plataforma com três artefatos públicos:

- Interface `Storage` (Put/Get).
- Implementação `PostgresStorage` (pgx + `database.DBTX`).
- Middleware chi `Middleware(scope string, storage Storage, ttl time.Duration)`.

Tabela compartilhada `idempotency_keys(scope, key, user_id, request_hash, response_status, response_body, expires_at)` com `UNIQUE (scope, key, user_id)`. O campo `scope` evita colisão entre módulos. No MVP, somente o `card` consome (`scope = "card"`); migração de `billing`/`identity` fica para fase 2.

### F-05 — Persistência Postgres do agregado `Card`

Repositório em pgx puro (`internal/card/infrastructure/repositories/postgres/card_repository.go`) com `Insert`, `GetByID`, `ListByUser`, `UpdateByID`, `SoftDeleteByID`. Persistência em UTC; cálculo de fatura em `America/Sao_Paulo`. Migration `0011_create_card_cards.up.sql` cria a tabela `cards` e o índice parcial de unicidade. Migration `down` renomeia `cards` → `cards_archived_<timestamp>` para preservar dados.

### F-06 — Middleware `RequireUser` transitório

Middleware chi em `internal/card/infrastructure/http/server/middleware/require_user.go` extrai `X-User-ID` do request, valida que é UUID v4, e injeta `user_id` no `context.Context`. Retorna 401 com envelope de erro padronizado se ausente/inválido. Documentado como solução **transitória** até existir módulo de autenticação (JWT/OIDC) — registrado em ADR-003. Quando autenticação real existir, o contrato HTTP do `card` não muda (o `ctx` continua carregando `user_id`).

### F-07 — Observabilidade nativa

Spans OTel em todas as camadas (`card.handler.<op>`, `card.middleware.<name>`, `card.usecase.<op>`, `card.repository.pg.<query>`, `card.domain.invoice_for`). Logs estruturados JSON com `trace_id`, `span_id`, `user_id`, `card_id`, `operation`, `outcome`, `duration_ms`, `error_kind`. **Helper `redactCardLogFields`** garante que `name` e `nickname` jamais cheguem ao logger. Sem métricas Prometheus dedicadas no MVP — taxa de erro e latência derivam dos spans via tracing exporter.

### F-08 — Histórico financeiro imutável

Alteração de `closing_day` ou `due_day` de um cartão **não dispara** recálculo retroativo de faturas de transações já persistidas. O futuro agregado `Transaction` denormalizará `closing_date` e `due_date` calculados no momento da criação da transação. Esta garantia é expressa via contrato no PRD e validada por contract test do endpoint `PUT /cards/{id}` (não retorna estado alterado de transações; o módulo de transações é responsável por validar imutabilidade quando existir).

## Requisitos Funcionais

### Domínio e cálculo de fatura

- **RF-01**: O módulo DEVE expor a função pura `InvoiceFor(purchase time.Time, cycle BillingCycle, tz *time.Location) Invoice` em `internal/card/domain/services/billing_cycle.go`, sem dependências de IO, sem acesso a relógio externo e sem panic em produção.
- **RF-02**: O algoritmo DEVE converter `purchase` para o timezone canônico `America/Sao_Paulo` antes de calcular `closing_date` e `due_date`.
- **RF-03**: O algoritmo DEVE aplicar clamp `min(day, daysInMonth(year, month))` para `closing_day` e `due_day` quando o dia configurado exceder a quantidade de dias do mês de referência (ex.: `closing_day=31` em fevereiro vira 28 ou 29 conforme bissexto).
- **RF-04**: O algoritmo DEVE auto-detectar a convenção do ciclo:
  - Se `closing_day > due_day`: fechamento ocorre no **mês anterior** ao vencimento (caso Itaú/Bradesco).
  - Se `closing_day < due_day`: fechamento ocorre no **mesmo mês** do vencimento (caso Nubank tradicional).
  - Se `closing_day == due_day`: fechamento ocorre no **dia anterior** ao vencimento (convenção definida; documentada).
- **RF-05**: O algoritmo DEVE retornar a fatura **corrente** quando `purchase.date() ≤ closing_date.date()` e a fatura **seguinte** caso contrário, ambas usando a mesma convenção de clamp para os meses subsequentes.
- **RF-06**: A função DEVE ser determinística, pura e reentrante: dadas as mesmas entradas, retorna sempre o mesmo `Invoice`.
- **RF-07**: `time.LoadLocation("America/Sao_Paulo")` DEVE ser carregada uma única vez via `sync.Once` em variável de pacote (NÃO usar `init()`, R0 do AGENTS.md); falha de load DEVE encerrar o processo com `os.Exit(1)` durante o startup, jamais via `panic` em produção.
- **RF-08**: Os value objects `Nickname`, `CardName` e `BillingCycle{ClosingDay, DueDay}` DEVEM validar invariantes no construtor; `closing_day` e `due_day` DEVEM estar em `[1, 31]`; `name` DEVE ter 1–64 caracteres; `nickname` DEVE ter 1–32 caracteres.

### Persistência

- **RF-09**: O agregado `Card` DEVE persistir os campos `id (UUID v4)`, `user_id (UUID v4)`, `name (TEXT)`, `nickname (TEXT)`, `closing_day (SMALLINT 1-31)`, `due_day (SMALLINT 1-31)`, `created_at (TIMESTAMPTZ)`, `updated_at (TIMESTAMPTZ)`, `deleted_at (TIMESTAMPTZ NULL)`.
- **RF-10**: A tabela `cards` DEVE possuir constraints `CHECK closing_day BETWEEN 1 AND 31` e `CHECK due_day BETWEEN 1 AND 31`.
- **RF-11**: A tabela `cards` DEVE possuir índice parcial `UNIQUE (user_id, nickname) WHERE deleted_at IS NULL`, garantindo unicidade de apelido somente entre cartões ativos do mesmo usuário.
- **RF-12**: A tabela `cards` DEVE possuir índice composto `(user_id, created_at DESC, id DESC) WHERE deleted_at IS NULL` para suportar a listagem paginada por cursor.
- **RF-13**: Exclusão DEVE ser **soft-delete**: preenche `deleted_at` com `now() AT TIME ZONE 'UTC'`. Operações subsequentes em `GET`, `PUT`, `DELETE` ou `POST` que esbarrem em cartões soft-deleted DEVEM retornar 404.
- **RF-14**: Inserção concorrente que viole a unicidade parcial DEVE ser capturada pelo repositório via `pgerrcode.UniqueViolation` e propagada como `ErrNicknameConflict`, mapeada para HTTP 409 Conflict no handler.
- **RF-15**: IDs DEVEM ser gerados como UUID v4 via `github.com/google/uuid`, mantendo a convenção já estabelecida em `internal/identity`.
- **RF-16**: O módulo NÃO DEVE persistir, transitar ou expor PAN, CVV/CVC, trilha magnética, PIN, ou qualquer outro dado sensível de cartão de pagamento. Validador estático/lint DEVE ser configurado para rejeitar PRs que adicionem colunas ou campos com nomes que casem com `pan|cvv|cvc|track|pin` no escopo deste módulo.

### Migrations

- **RF-17**: A migration `0010_create_platform_idempotency_keys.up.sql` DEVE criar a tabela `idempotency_keys(scope TEXT, key TEXT, user_id UUID, request_hash TEXT, response_status INT, response_body BYTEA, expires_at TIMESTAMPTZ, created_at TIMESTAMPTZ)` com `PRIMARY KEY (scope, key, user_id)`. Migration `down` DEVE renomear a tabela para `idempotency_keys_archived_<timestamp>` em vez de `DROP TABLE`.
- **RF-18**: A migration `0011_create_card_cards.up.sql` DEVE criar a tabela `cards` com colunas e constraints de RF-09/RF-10/RF-11/RF-12. Migration `down` DEVE renomear `cards` → `cards_archived_<timestamp>` e dropar apenas os índices únicos.
- **RF-19**: Ambas as migrations DEVEM ser idempotentes para reaplicação (usar `IF NOT EXISTS`/`IF EXISTS` onde apropriado).
- **RF-20**: `docs/runbooks/card-rollback.md` DEVE documentar o procedimento de rollback completo: revert do registro em `cmd/server/server.go`, aplicação das migrations `down`, e instruções para restauração caso a tabela renomeada precise voltar ao nome original.

### HTTP — endpoints e contrato

- **RF-21**: O módulo DEVE expor `POST /api/v1/cards` que aceita `{name, nickname, closing_day, due_day}` em JSON e retorna 201 Created com `{id, user_id, name, nickname, closing_day, due_day, created_at, updated_at}` + header `Location: /api/v1/cards/{id}`.
- **RF-22**: O módulo DEVE expor `GET /api/v1/cards` que aceita query params `cursor` (string base64) e `limit` (int 1–100, default 20). Resposta 200 OK retorna `{items: [...], next_cursor: string|null}` ordenado por `created_at DESC, id DESC`, excluindo soft-deleted.
- **RF-23**: O módulo DEVE expor `GET /api/v1/cards/{id}` retornando o cartão se pertence ao usuário autenticado e não está soft-deleted; 404 caso contrário.
- **RF-24**: O módulo DEVE expor `PUT /api/v1/cards/{id}` aceitando `{name, nickname, closing_day, due_day}` (todos opcionais; envio parcial atualiza somente os campos presentes via JSON sparse). Retorna 200 OK com o cartão atualizado.
- **RF-25**: O módulo DEVE expor `DELETE /api/v1/cards/{id}` que aplica soft-delete e retorna 204 No Content.
- **RF-26**: O módulo DEVE expor `GET /api/v1/cards/{id}/invoices?for=<YYYY-MM-DD>` que retorna `{closing_date: "YYYY-MM-DD", due_date: "YYYY-MM-DD"}` usando o ciclo atual do cartão e o algoritmo `InvoiceFor`. Datas no fuso `America/Sao_Paulo`. 400 Bad Request se `for` ausente ou em formato inválido; 404 se cartão não existe/foi soft-deleted.
- **RF-27**: Todos os endpoints DEVEM exigir header `X-User-ID` (UUID v4) via middleware `RequireUser`. Ausência/invalidade retorna 401 Unauthorized com envelope de erro padronizado.
- **RF-28**: `POST`, `PUT` e `DELETE` DEVEM exigir header `Idempotency-Key` (string ASCII 1–128 chars). Ausência retorna 400 Bad Request. Mesma chave + mesmo `request_hash` retorna a resposta original armazenada; mesma chave + `request_hash` divergente retorna 409 Conflict.
- **RF-29**: O contrato HTTP completo DEVE ser publicado em `internal/card/infrastructure/http/server/openapi.yaml` (OpenAPI 3.1) e validado por contract tests via golden files em `internal/card/infrastructure/http/server/testdata/`.

### Idempotência

- **RF-30**: O pacote `internal/platform/idempotency/` DEVE expor: `type Storage interface { Get(ctx, scope, key, userID) (*Record, error); Put(ctx, scope, key, userID, requestHash, status, body, expiresAt) error }`, implementação `PostgresStorage` baseada em pgx puro e `database.DBTX`, e middleware chi `Middleware(scope string, storage Storage, ttl time.Duration) func(http.Handler) http.Handler`.
- **RF-31**: O middleware DEVE: (a) ler `Idempotency-Key` do header, (b) calcular `request_hash` a partir do corpo (SHA-256), (c) consultar `Storage.Get`; se hit com `request_hash` idêntico, retornar a resposta armazenada com status original; se hit com `request_hash` divergente, retornar 409 Conflict; se miss, executar o handler e gravar via `Storage.Put` com `expires_at = now + ttl`.
- **RF-32**: TTL padrão DEVE ser 24 horas. Limpeza de registros expirados NÃO é parte do MVP (registrado em "Suposições e Questões em Aberto").

### Observabilidade

- **RF-33**: Spans OTel DEVEM ser criados em todas as camadas com nomes no padrão `card.<layer>.<operation>`: `card.handler.create`, `card.handler.list`, `card.handler.get`, `card.handler.update`, `card.handler.delete`, `card.handler.invoice_for`, `card.middleware.require_user`, `card.middleware.idempotency`, `card.usecase.create`, `card.usecase.list`, `card.usecase.get`, `card.usecase.update`, `card.usecase.delete`, `card.usecase.invoice_for`, `card.repository.pg.insert`, `card.repository.pg.list_by_user`, `card.repository.pg.get_by_id`, `card.repository.pg.update`, `card.repository.pg.soft_delete`, `card.domain.invoice_for`.
- **RF-34**: Logs estruturados JSON DEVEM incluir `trace_id`, `span_id`, `user_id`, `card_id`, `operation`, `outcome`, `duration_ms`, `error_kind`. Logs DEVEM ser emitidos para os eventos: `card.create.started`, `card.create.completed`, `card.create.failed`, `card.list.served`, `card.update.completed`, `card.delete.completed`, `card.invoice_for.computed`, `card.idempotency.replay`, `card.auth.rejected`.
- **RF-35**: Logs e spans NÃO DEVEM conter `name`, `nickname`, payload bruto do request ou do response. Helper `redactCardLogFields(card)` DEVE ser obrigatório em todas as chamadas de logger no módulo, e teste de regressão DEVE inspecionar saída do logger configurado para garantir ausência desses campos.
- **RF-36**: Erros propagados DEVEM ser registrados via `span.RecordError(err)` e logados com `error_kind` mapeado a partir do sentinel (`ErrCardNotFound`, `ErrNicknameConflict`, `ErrInvalidClosingDay`, etc.).

### Aderência a R0–R7 e governança

- **RF-37**: O módulo NÃO DEVE conter `init()` em nenhum pacote (R0).
- **RF-38**: O módulo NÃO DEVE invocar `panic` em código de produção (R5.12). Erros de startup DEVEM encerrar via `os.Exit(1)` no `main`.
- **RF-39**: Toda função que faça IO ou propague IO DEVE aceitar `context.Context` como primeiro parâmetro (R6).
- **RF-40**: Erros DEVEM ser construídos com `errors.New` para sentinels, `fmt.Errorf("ctx: %w", err)` para wrapping e `errors.Join` para agregação (R7.6). Cada erro DEVE ser tratado uma única vez na cadeia (R5.10).
- **RF-41**: O módulo NÃO DEVE usar abstrações de relógio (`clock.Clock` ou similares). Quando o instante for necessário, DEVE ser passado por argumento (preferencial) ou obtido inline via `time.Now().UTC()` (R6.7).
- **RF-42**: O módulo NÃO DEVE usar o padrão `var _ Interface = (*Type)(nil)` para asserção de interface em tempo de compilação (R6.4).
- **RF-43**: O módulo NÃO DEVE expor IO em pacotes `domain` (regra de fronteira do AGENTS.md). `domain` DEVE permanecer puro, sem importar `application`, `infrastructure`, `platform`, banco, HTTP, filas, serialização, configuração ou drivers.

### Testes

- **RF-44**: Testes do `domain/services/billing_cycle.go` DEVEM incluir **mínimo 50 fixtures table-driven** cobrindo: fev/28, fev/29 bissexto, abr/jun/set/nov (30 dias), virada de ano (dez/jan), `due_day == closing_day`, `due_day > closing_day` (mesmo mês), `due_day < closing_day` (mês anterior), `closing_day=31`, `due_day=31`, datas em DST histórico BR (2018), datas em horário-padrão (2026).
- **RF-45**: Testes do `domain/services/billing_cycle.go` DEVEM incluir property-based test via `testing/quick` com `quick.Config{MaxCount: 10000}` validando invariantes: (a) `due_date >= closing_date`, (b) `due_date >= purchase_date`, (c) idempotência (mesma entrada → mesma saída), (d) `closing_date.day == min(closing_day, daysInMonth(closing_date.year, closing_date.month))`.
- **RF-46**: Testes de repositório DEVEM rodar como integração via testcontainers (Postgres 16) com a migration aplicada, cobrindo: insert + read, soft-delete + read (espera 404), conflito de unicidade parcial (espera `ErrNicknameConflict`), listagem paginada estável.
- **RF-47**: Testes de contrato HTTP DEVEM usar golden files em `internal/card/infrastructure/http/server/testdata/` validando que o OpenAPI 3.1 declarado bate com responses reais para casos canônicos.
- **RF-48**: Mocks de interfaces da `application/interfaces/` DEVEM ser gerados via `mockery v2` configurado em `mockery.yml`.

### Wiring

- **RF-49**: `module.go` DEVE expor `NewCardModule(cfg *configs.Config, o11y observability.Observability, mgr manager.Manager) CardModule` retornando struct com `CardRouter *server.CardRouter`, `RepositoryFactory interfaces.RepositoryFactory`, `CardLookup *usecases.InvoiceFor`. Retorno **sem error** (sem inicialização complexa que possa falhar fora de Postgres/observability já validados upstream).
- **RF-50**: `cmd/server/server.go` DEVE invocar `NewCardModule(cfg, o11y, dbManager)` e, se `cardModule.CardRouter != nil`, chamar `srv.RegisterRouters(cardModule.CardRouter)` seguido de log estruturado `"card module wired"` com atributo `router_registered=true`.

## Experiência do Usuário

Módulo backend-only; sem UI própria. Front (web/mobile) e WhatsApp consomem os endpoints HTTP. O envelope de erro segue o padrão já em uso por `internal/billing` (consistência cross-módulo). Mensagens de erro DEVEM ser concisas em pt-BR (ex.: `"apelido já em uso"`, `"data inválida; use YYYY-MM-DD"`, `"cartão não encontrado"`).

## Restrições Técnicas de Alto Nível

- **Stack obrigatória**: Go com versão fixada em `go.mod`; chi v5 para HTTP; pgx v5 para Postgres; `google/uuid` v4; `testify/mock` + `mockery v2`; `testcontainers-go` para integração.
- **Arquitetura**: monolito modular; `domain` puro; `application` sem IO; `infrastructure` com IO concreto; bounded context `internal/card` sem importar `internal/billing` ou `internal/identity` diretamente (apenas portas declaradas).
- **Banco**: Postgres compartilhado (mesmo `dbManager` de identity/billing); pgx puro; migrations em `/migrations/NNNNN_*.{up,down}.sql` via `devkit-go/pkg/database/migration`.
- **Observabilidade**: stack OTel já estabelecida via `internal/platform/observability`; sem dependências novas.
- **Segurança**: aplicação **não-PCI**; LGPD com bases legais "execução de contrato + legítimo interesse"; PII (`name`, `nickname`) jamais em logs/spans.
- **Autenticação**: módulo de auth ainda não existe; uso transitório de header `X-User-ID` (UUID v4) documentado em ADR-003; substituição por JWT/OIDC em fase 2 preserva o contrato HTTP.
- **Performance**: p99 CRUD < 300 ms; p99 `InvoiceFor` < 10 ms; SLO 99,5% de disponibilidade mensal.
- **Padrão Obrigatório de Módulo**: layout, construtor, wiring e padrão de handlers/middleware seguem AGENTS.md sem desvios.
- **R0–R7**: aderência total; sem `init()`, sem `panic` em produção, sem `clock.Clock`, sem `var _ Interface = (*Type)(nil)`.

## Fora de Escopo

- **Limite de crédito** e saldo disponível por cartão. Entram em fase 2 quando o módulo de transações exigir.
- **Bandeira** (`brand`) e **últimos 4 dígitos** (`last4`). Não exigidos pela regra de fatura; entram em fase 2 por UX.
- **Multi-titular** / cartão adicional. Cada cartão tem exatamente um `user_id`.
- **Integração com Open Finance / Belvo / Pluggy** para sync automático.
- **Emissor (banco)** como campo estruturado. Sem normalização BACEN/ISPB no MVP.
- **Audit log de domínio** (`card_audit_log`) com PII completa. Logs OTel + spans cobrem o MVP; audit dedicado entra em fase 2 se LGPD/compliance exigir.
- **Versionamento histórico de ciclos** por cartão (`card_billing_cycles`). Mudança de ciclo simplesmente sobrescreve.
- **Métricas Prometheus dedicadas** (`card_operations_total`, `card_invoicefor_duration_seconds`). Spans OTel cobrem latência/erro no MVP; métricas entram em fase 2 se necessário.
- **Recálculo retroativo automático** de faturas ao alterar ciclo. Histórico financeiro é imutável por desenho.
- **JWT/OIDC** já no MVP. Header `X-User-ID` é solução transitória.
- **Migração de `internal/billing` / `internal/identity`** para o novo pacote `internal/platform/idempotency/`. Fica para refactor pós-MVP.
- **Job de limpeza** de `idempotency_keys` expirados. Tabela cresce limitada (~10k linhas em estado estacionário); job entra em fase 2 se volume justificar.
- **Endpoint de recálculo manual** (`POST /cards/{id}/recompute-invoices?from=<date>`). Fora do MVP por imutabilidade do histórico financeiro.
- **Validação de Luhn** ou checksum de PAN. `mecontrola` não toca em PAN.

## Suposições e Questões em Aberto

- **S-01 — FK lógica vs física**: `cards.user_id` referencia `users.id` como FK lógica no MVP. Decidir em techspec se a FK física será adicionada (depende de a tabela `users` estar no mesmo schema/banco).
- **S-02 — Política de retenção**: ainda não há política formal de expurgo para cartões soft-deleted. Sugestão preliminar: 5 anos para alinhar com prática contábil BR. Decisão fica para fase 2.
- **S-03 — Ferramenta de load test**: `k6` vs `vegeta` para validação dos cenários M-02/M-03/M-04 em homologação. Definir em ADR-005 durante a entrega.
- **S-04 — Validação anti-PCI no CI**: configuração específica do linter customizado para RF-16 (rejeitar `pan|cvv|cvc|track|pin`) fica para a techspec; possíveis ferramentas: regra `golangci-lint` custom ou pre-commit hook em pre-merge.
- **S-05 — Job de cleanup de `idempotency_keys`**: não há job no MVP; volume permanece estável por TTL 24h, mas escolha entre Postgres `pg_cron` e job Go via `internal/platform/worker/job` fica para fase 2.
- **S-06 — Convenção quando `closing_day == due_day`**: PRD define que fechamento ocorre no dia anterior ao vencimento (RF-04). Validar em revisão de produto se essa convenção é semanticamente aceitável; alternativa seria recusar `closing_day == due_day` na validação do VO `BillingCycle` e forçar o usuário a escolher.
- **S-07 — Cabeçalho `X-User-ID` em produção**: confirmar com infraestrutura que o gateway/balanceador na frente do `mecontrola` autentica e injeta esse header de forma confiável antes da exposição pública geral; antes disso, exposição interna apenas.
