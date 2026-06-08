# Transcript do Discovery Técnico

## Contexto Inicial

Continuação direta do bundle de brainstorming decisório `discoveries/brainstorm-modulo-crud-de-cartoes-de-credito-com-calculo-de-fatura/` (status `done`, validado SUCCESS). A direção arquitetural foi fixada e é inegociável neste discovery:

- Bounded context `internal/card` seguindo o Padrão Obrigatório de Módulo do `AGENTS.md` (`application/usecases`, `application/dtos/{input,output}`, `application/interfaces`, `domain/entities`, `domain/valueobjects`, `domain/services`, `infrastructure/http/server/{handlers,middleware,router.go}`, `infrastructure/repositories/postgres`, `module.go`).
- `InvoiceFor` como função pura de domínio (`domain/services/billing_cycle.go`), stateless, sem IO, sem mock de tempo (R6.7), reutilizável por módulos consumidores.
- Schema MVP: `id`, `user_id`, `name`, `nickname`, `closing_day`, `due_day`, `created_at`, `updated_at`, `deleted_at`.
- Algoritmo de ciclo: clamp `min(day, daysInMonth(year, month))` + auto-detecção `closing_day > due_day` (fechamento mês anterior) vs `closing_day < due_day` (mesmo mês).
- Timezone canônico `America/Sao_Paulo` no cálculo; persistência em UTC.
- Soft-delete (`deleted_at`) + unicidade parcial Postgres `(user_id, nickname) WHERE deleted_at IS NULL`.
- Header `Idempotency-Key` em POST/PUT/DELETE (TTL 24h).
- Robustez extrema: 50+ fixtures table-driven + property-based tests.
- Histórico financeiro imutável; mudança de ciclo NÃO recalcula transações antigas.
- Endpoint público `GET /cards/:id/invoices?for=<date>` + porta interna `CardLookup.InvoiceFor`.
- Observabilidade MVP: logs estruturados com `trace_id` propagado + OTel spans em domain/adapter (`pkg.layer.op`).
- Aderência total a R0–R7 do `AGENTS.md`.
- Não-PCI: jamais persistir PAN, CVV, trilha magnética ou PIN.

### Materiais de Apoio
- `discoveries/brainstorm-modulo-crud-de-cartoes-de-credito-com-calculo-de-fatura/decision-brief.md`
- `discoveries/brainstorm-modulo-crud-de-cartoes-de-credito-com-calculo-de-fatura/option-scorecard.md`
- `discoveries/brainstorm-modulo-crud-de-cartoes-de-credito-com-calculo-de-fatura/assumptions.md`
- `AGENTS.md` (Padrão Obrigatório de Módulo + R0–R7)
- `CLAUDE.md` (referência cruzada)
- `internal/identity/` (padrão de referência de módulo)
- `internal/billing/` (padrão de referência com middleware customizado)
- `internal/platform/observability/` (Tracer, Logger)
- `internal/platform/outbox/` (padrão de instance_id + IDs)
- Pesquisa oficial 2026 sobre bandeiras BR e regulação BACEN/BCB (consolidada no brainstorm)

### Inventário concreto de convenções (subagente Explore)
- Módulo: `NewXxxModule(cfg *configs.Config, o11y observability.Observability, mgr manager.Manager) XxxModule` ou `(XxxModule, error)`; `cmd/server/server.go:117+` faz `srv.RegisterRouters(module.Router)`.
- Router: `Register(r chi.Router)` em struct `*XxxRouter` que monta `r.Route("/api/v1/<prefix>", ...)`; handlers são métodos `Handle(w, r)`.
- Repositório: pgx puro, `database.DBTX` concreto injetado; spans `o11y.Tracer().Start(ctx, "pkg.layer.op")`; `span.RecordError(err)`.
- Migrations: raiz `/migrations/NNNNN_*.{up,down}.sql`, próximo número livre **0010**; migrator via `devkit-go/pkg/database/migration`.
- Errors: sentinels em `application/errors.go`; mapeamento HTTP em handler via `errors.Is`.
- DTOs: structs simples em `application/dtos/{input,output}`; request/response inline no handler com tags `json:`; validação via value objects, **não há lib genérica**.
- ID: `github.com/google/uuid` v4 (teste explícito de `parsed.Version() == 4`).
- Testes: `_test.go` ao lado; integration com `//go:build integration` + testcontainers + migrator embed; mocks com `testify/mock` gerados via `mockery v2` (configurado em `mockery.yml`).
- Auth: **não há middleware de autenticação atual** — endpoints existentes recebem `user_id` indiretamente (webhook + payload). Será introduzido pelo módulo `card`.
- Property-based testing: **nenhuma lib presente**; precisará escolher entre `testing/quick` ou `go test -fuzz`.

## Rodada 1

### Q1.1 — Tier de criticidade do domínio
- Resposta: **Importante** — erro causa retrabalho, não prejuízo direto.
- Implicação: SLO 99.5%, p99 CRUD < 300ms, p99 InvoiceFor < 10ms. Tier suficiente para spans+logs sem métricas Prometheus dedicadas no MVP.

### Q1.2 — Prefixo HTTP
- Resposta: **`/api/v1/cards`** — segue convenção `internal/identity` (`/api/v1/identity/users`) e `internal/billing` (`/api/v1/billing/webhooks`).
- Implicação: nomenclatura `card.handler.<op>`, `card.usecase.<op>`, `card.repository.<op>` em spans.

### Q1.3 — SLO mínimo
- Resposta: **Disponibilidade 99.5% / p99 CRUD < 300ms / p99 InvoiceFor < 10ms**.
- Implicação: alertas via tracing exporter em error budget; baseline confortável para Postgres único.

### Q1.4 — Escopo de endpoints (multi-select)
- Resposta: **POST, GET list (paginação cursor), GET :id, PUT :id, DELETE :id, GET :id/invoices?for=<date>** — todos.
- Implicação: 6 endpoints. POST/PUT/DELETE consumirão middleware de idempotência. `GET /invoices?for=<date>` valida formato ISO-8601 e usa `InvoiceFor` pura.

## Rodada 2

### Q2.1 — Geração de ID
- Resposta: **UUID v4** via `github.com/google/uuid`.
- Implicação: aderente à convenção atual (`internal/identity/domain/entities/id.go`). Sem ordenação natural, mas a paginação cursor inclui `created_at` para ordenação estável.

### Q2.2 — Idempotência
- Resposta: **Pacote genérico em `internal/platform/idempotency/`** com middleware Chi + Storage interface + impl Postgres. Tabela compartilhada `idempotency_keys`. Card consome agora; billing/identity migrarão depois.
- Implicação: +1 dia ao escopo. ADR-002 registra introdução do pacote.

### Q2.3 — Volumetria
- Resposta: **100k usuários × 3 cartões = 300k cards / 10k criadas/dia**.
- Implicação: 300k linhas em `cards` — Postgres único é mais que suficiente. Índice composto `(user_id, created_at DESC) WHERE deleted_at IS NULL` cobre listagem.

### Q2.4 — Paginação
- Resposta: **Cursor opaco base64 (`id` + `created_at`) + `ORDER BY created_at DESC, id DESC`**.
- Implicação: contrato HTTP `?cursor=<base64>&limit=20`. p99 < 30ms com índice composto. Estável frente a inserções concorrentes.

## Rodada 3

### Q3.1 — PII em logs
- Resposta: **Nunca logar `name`/`nickname`**; spans com `card_id` + `user_id` apenas.
- Implicação: helper `redactCardLogFields(card)` em handler converte para `{card_id, user_id, closing_day, due_day}`. Aderente a `internal/identity/domain/pii/`.

### Q3.2 — Rollout
- Resposta: **Deploy direto** — rotas só registradas via `srv.RegisterRouters(cardModule.CardRouter)`.
- Implicação: rollback = revert do commit + redeploy. Sem feature flag por usuário.

### Q3.3 — Rollback de migration
- Resposta: **`down` preserva dados** — renomeia `cards` → `cards_archived_<timestamp>`; drop apenas índices únicos.
- Implicação: scripts down explicitamente comentam que dados são preservados; runbook documenta como restaurar.

### Q3.4 — Métricas adicionais (multi-select)
- Resposta: **Apenas logs + OTel (decisão brainstorm mantida)**.
- Implicação: spans cobrem latência e erros; alertas vêm do tracing exporter. Métricas Prometheus dedicadas entram em fase 2 se necessário.

## Rodada 4

### Q4.1 — Autenticação
- Resposta: **Middleware `RequireUser` extraindo header `X-User-ID` (UUID v4)** + injeção em `ctx`. JWT/OIDC ficam para fase 2.
- Implicação: `internal/card/infrastructure/http/server/middleware/require_user.go`. Retorna 401 se header ausente/inválido. ADR-003 registra premissa transitória.

### Q4.2 — Motor de property-based tests
- Resposta: **`testing/quick` stdlib**.
- Implicação: zero dependência nova. `InvoiceFor` recebe `(purchaseDate, closingDay, dueDay)` — espaço pequeno e enumerável por `quick.Config{MaxCount: 10000}`.

### Q4.3 — Escopo da idempotência genérica
- Resposta: **Criar pacote genérico AGORA, usar só em `card`; billing/identity migram depois**.
- Implicação: tabela `idempotency_keys(scope, key, user_id, request_hash, response_status, response_body, expires_at)` compartilhada; coluna `scope` evita colisão entre módulos. Migration `0010_create_platform_idempotency_keys`.

### Q4.4 — Pool de conexão Postgres
- Resposta: **Reusa `database.DBTX` compartilhado**.
- Implicação: zero alteração de wiring de Postgres. Mantém convenção.

## Decisão Final
Materializar dossiê com tudo acima. Próximo passo: `create-prd` consumindo este bundle.
