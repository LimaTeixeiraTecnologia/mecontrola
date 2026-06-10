<!-- spec-hash-prd: f50a99a3576147e2f2366a8bbff0a347f0fc42d24307dfbf90d7fecbbcf9ae69 -->

# Especificação Técnica — Módulo de Orçamentos Mensais (`internal/budgets`)

## Resumo Executivo

O módulo `internal/budgets` é um bounded context de escrita/leitura financeira que entrega o MVP de orçamentos mensais por categoria descrito em `prd.md` v24. Segue o **Padrão Obrigatório de Módulo** de `AGENTS.md` (DI manual explícita em `module.go`, igual a `internal/billing`), respeita as **Regras de Adaptador** R-ADAPTER-001 (handlers/consumers/jobs/producers finos, fluxo `adapter → usecase → repo/service/client`) e cumpre a política inegociável de **zero comentários em `.go` de produção**.

Decisões arquiteturais centrais (cada uma vinculada a uma ADR):

1. **Integração com `internal/categories`** via interface consumer-defined em `internal/budgets/application/interfaces/CategoriesReader`, implementada por categories — respeita AGENTS.md "interface no consumidor" (ADR-001).
2. **Outbox interno unificado** com `event_type = budgets.expense.committed.v1` carregando `committed_at` e `cutoff_competence_br` para a avaliação assíncrona de alertas (RT-24); avaliação roda em job dedicado, sem bloquear o caminho síncrono (ADR-002).
3. **Estado de cruzamento de limiar persistido** em `budgets_threshold_states` com versão monotônica, lido/atualizado exclusivamente pelo avaliador assíncrono em UPSERT idempotente — garante RF-59/RF-60/RF-60a/RF-60b (ADR-003).
4. **Resumo on-demand por agregação SQL** sobre `budgets_expenses` com índice composto parcial `(user_id, competence, subcategory_id) WHERE deleted_at IS NULL`; zero acumulado persistido (RF-54), p95 ≤ 300 ms para até 100 despesas/mês (ADR-004).
5. **Eventos pendentes retomados por job periódico** (`budgets-pending-events-reaper`, 30 s) com expiry de 24h embutido; allowlist de produtores fixada como constante Go (RT-28). (ADR-005)

O contrato de leitura de categorias é pré-requisito de produção (RT-23); budgets falha startup se as cinco raízes oficiais não forem resolvidas no boot (RT-31).

## Arquitetura do Sistema

### Visão Geral dos Componentes

```text
internal/budgets/
  module.go                                            -- DI manual; expõe BudgetsRouter, EventHandlers, Jobs
  application/
    dtos/input/
      create_budget_input.go
      activate_budget_input.go
      delete_draft_input.go
      create_recurrence_input.go
      upsert_expense_input.go                          -- usado por API e por consumer de eventos
      delete_expense_input.go
      list_alerts_input.go
    dtos/output/
      budget_output.go
      monthly_summary_output.go                        -- por raiz oficial (RF-54a)
      expense_output.go
      alert_output.go
      recurrence_result_output.go                      -- resultado por competência (RF-21a)
    usecases/
      create_budget.go                                  -- RF-01..RF-08
      activate_budget.go                                -- RF-07/RF-07a/RF-07b
      delete_draft_budget.go                            -- RF-09b/RF-09c
      create_recurrence.go                              -- RF-19..RF-24
      create_or_auto_draft_for_expense.go               -- RF-12..RF-17 (auxiliar interno)
      upsert_expense.go                                 -- RF-25..RF-31 (criação + edição; idempotente)
      delete_expense.go                                 -- RF-27/RF-44..RF-47
      evaluate_alert.go                                 -- RF-55..RF-64 (consumidor do outbox)
      apply_pending_event.go                            -- usado pelo reaper
      get_monthly_summary.go                            -- RF-48..RF-54
      list_alerts.go                                    -- GET /api/v1/budgets/alerts
    interfaces/
      budget_repository.go
      expense_repository.go
      alert_repository.go
      threshold_state_repository.go
      pending_event_repository.go
      categories_reader.go                              -- consumer-defined (ADR-001)
      outbox_publisher.go                               -- adapter local sobre outbox.Publisher
  domain/
    entities/
      budget.go                                         -- estado: draft|active; invariantes RF-06/RF-07
      allocation.go                                     -- basis points por raiz
      expense.go                                        -- version monotônica; tombstoneVersion
      expense_tombstone.go                              -- representação read-only para idempotência
      alert.go                                          -- state machine
      threshold_state.go                                -- currently_crossed + version
      pending_event.go                                  -- state machine
    valueobjects/
      competence.go                                     -- YYYY-MM em America/Sao_Paulo (RT-27)
      cents.go                                          -- BRL inteiro (RT-04)
      basis_points.go                                   -- 0..10000
      root_slug.go                                      -- enum dos 5 slugs oficiais
      threshold.go                                      -- enum 80|100
      mutation_kind.go                                  -- create|update|delete
      external_transaction_id.go                        -- UUID v4 ou ULID canônicos (RT-26)
      producer_source.go                                -- enum da allowlist (RT-28)
    services/
      allocation_distributor.go                         -- half-even + ordem determinística (RF-11/RF-11a)
      threshold_evaluator.go                            -- pure: dado gasto/planejado, retorna transições
  infrastructure/
    config/
      producers.go                                     -- allowlist constante (RT-28)
      categories_cache.go                              -- cache TTL 60s + bust por editorial_version (RT-31)
    http/server/
      router.go                                        -- chi.Router com RequireUser em todas as rotas
      handlers/
        create_budget_handler.go
        activate_budget_handler.go
        delete_draft_handler.go
        create_recurrence_handler.go
        upsert_expense_handler.go
        delete_expense_handler.go
        get_monthly_summary_handler.go
        list_alerts_handler.go
    messaging/database/
      producers/
        expense_committed_publisher.go                 -- adapter sobre outbox.Publisher (event único)
      consumers/
        expense_committed_consumer.go                  -- consome budgets.expense.committed.v1 → EvaluateAlert
        external_expense_consumer.go                   -- consome eventos cross-module (allowlist) → UpsertExpense
    jobs/handlers/
      abandoned_draft_reaper.go                        -- RF-18b cron 03:00 BR
      pending_events_reaper.go                         -- RF-38/RF-39a poll 30s + expiry 24h
      retention_purge.go                               -- RF-66 cron mensal
    repositories/postgres/
      budget_repository.go
      expense_repository.go
      alert_repository.go
      threshold_state_repository.go
      pending_event_repository.go
      categories_reader_adapter.go                     -- implementação consumindo CategoriesModule
```

Relacionamentos chave:

- `module.go` instancia repos, use cases, handlers, router, jobs, consumers e o producer de outbox; exporta os 3 jobs e o registry de event handlers para o `cmd/worker/worker.go`.
- `cmd/server/server.go` registra `BudgetsRouter` quando não-nulo (padrão dos demais módulos).
- `cmd/worker/worker.go` registra os 3 jobs e os 2 consumers (`expense.committed.v1` interno e o consumer de eventos externos da allowlist) no `events.Dispatcher` e no `worker.Manager`.
- Fluxo síncrono de escrita: handler → `UpsertExpense` → tx único `{ INSERT/UPDATE expense; INSERT outbox row }` → 201/200.
- Fluxo assíncrono de alerta: dispatcher → consumer → `EvaluateAlert` → recalcula gasto da raiz → `threshold_state_repository.UpsertIfTransition` → `alert_repository.Insert` (se transição `false→true`).

### Fluxo de Dados

```text
[Client HTTP]
   │  POST /api/v1/budgets/expenses { external_transaction_id, subcategory_id, amount_cents, competence }
   ▼
[upsert_expense_handler]  (RequireUser)
   │  decodifica + Principal.UserID + source="api"
   ▼
[UpsertExpense usecase]
   │  1. CategoriesReader.ValidateExpenseSubcategory(subID) → rootSlug
   │  2. tx.Run:
   │     2.1 ExpenseRepo.GetByIdentity(user, source, extID)
   │         ├─ tombstone? → return idempotent OK | conflict (conforme RF-47b/RF-29e)
   │         └─ existe? → conflict por versão / update / no-op
   │     2.2 BudgetRepo.GetOrAutoDraft(user, competence)        (RF-12)
   │     2.3 ExpenseRepo.Upsert(expense)                         (version=1 ou +1)
   │     2.4 OutboxPublisher.PublishExpenseCommitted(envelope)   (mesma tx)
   ▼
[Outbox dispatcher (já existente em internal/platform/outbox)]
   │  entrega at-least-once para events.Dispatcher
   ▼
[expense_committed_consumer]
   │  idempotente por event.ID
   ▼
[EvaluateAlert usecase]
   │  1. recalcula soma das despesas (user, competence, root_slug)
   │  2. lê BudgetRepo (precisa estar ACTIVE; senão suprime — RF-57)
   │  3. ThresholdEvaluator(spent, planned) → []Transition
   │  4. ThresholdStateRepo.UpsertIfTransition(...) → bool transitioned
   │  5. se transitioned + competência commit == cutoff_atual → AlertRepo.Insert(pending_delivery)
   │     se transitioned + competência commit < cutoff → AlertRepo.Insert(suppressed_retroactive) + métrica
   │     se transitioned + delivered cap atingido → AlertRepo.Insert(rate_limited)
   │     se !transitioned → métrica suppressed_stale (RF-56b)
```

## Design de Implementação

### Interfaces Chave

```go
type CategoriesReader interface {
    ResolveRootsBySlug(ctx context.Context, slugs []string) (map[string]uuid.UUID, error)
    ValidateExpenseSubcategory(ctx context.Context, id uuid.UUID) (rootSlug string, deprecated bool, err error)
    EditorialVersion(ctx context.Context) (int64, error)
}

type BudgetRepository interface {
    GetByUserCompetence(ctx context.Context, db database.DBTX, userID uuid.UUID, c valueobjects.Competence) (entities.Budget, error)
    CreateDraft(ctx context.Context, db database.DBTX, b entities.Budget) error
    Activate(ctx context.Context, db database.DBTX, b entities.Budget) error
    DeleteDraft(ctx context.Context, db database.DBTX, userID uuid.UUID, c valueobjects.Competence) error
    ListFutureNotActivated(ctx context.Context, db database.DBTX, userID uuid.UUID, from valueobjects.Competence, max int) ([]entities.Budget, error)
}

type ExpenseRepository interface {
    GetByIdentity(ctx context.Context, db database.DBTX, k entities.ExpenseIdentity) (entities.Expense, entities.ExpenseTombstone, error)
    Insert(ctx context.Context, db database.DBTX, e entities.Expense) error
    Update(ctx context.Context, db database.DBTX, e entities.Expense, expectedVersion int64) error
    SoftDelete(ctx context.Context, db database.DBTX, e entities.Expense, expectedVersion int64) (tombstoneVersion int64, err error)
    SumByRoot(ctx context.Context, db database.DBTX, userID uuid.UUID, c valueobjects.Competence) (map[valueobjects.RootSlug]int64, error)
}

type ThresholdStateRepository interface {
    UpsertIfTransition(ctx context.Context, db database.DBTX, k entities.ThresholdKey, nowCrossed bool, committedAt time.Time) (transitioned bool, err error)
}

type AlertRepository interface {
    Insert(ctx context.Context, db database.DBTX, a entities.Alert) error
    CountDelivered(ctx context.Context, db database.DBTX, k entities.ThresholdKey) (int64, error)
    ListForUser(ctx context.Context, db database.DBTX, userID uuid.UUID, q dto.AlertQuery) ([]entities.Alert, string, error)
}

type PendingEventRepository interface {
    Insert(ctx context.Context, db database.DBTX, p entities.PendingEvent) error
    ListReady(ctx context.Context, db database.DBTX, limit int) ([]entities.PendingEvent, error)
    Transition(ctx context.Context, db database.DBTX, id uuid.UUID, to entities.PendingState, reason string) error
}

type ExpenseCommittedPublisher interface {
    Publish(ctx context.Context, db database.DBTX, env ExpenseCommittedEnvelope) error
}
```

Notas sobre as interfaces (todas consumer-defined em `application/interfaces/`):

- `database.DBTX` é o handle concreto do devkit (R-ADAPTER-001 e regra AGENTS.md "use cases recebem `database.DBTX` concreto").
- **Unit of Work**: cada use case com mutação consome `uow.UnitOfWork[T]` de `github.com/JailtonJunior94/devkit-go/pkg/database/uow` diretamente (padrão já adotado em `internal/billing/application/usecases/` e `internal/identity/module.go`). Composição em `module.go` via `uow.New[T](mgr, uow.WithObservability(o11y))` ou `uow.NewVoid(mgr, ...)`. Use cases chamam `uc.uow.Do(ctx, func(ctx, tx database.DBTX) (T, error) { ... })`. **Não** criar interface `TxRunner` local — viola "preferir a opção mais segura, sem abstrações sem demanda" de `AGENTS.md`.
- `CategoriesReader` é a única dependência cross-module; implementada por `infrastructure/repositories/postgres/categories_reader_adapter.go` que delega ao `CategoriesModule` exposto (ver ADR-001).
- Não há `Clock` injetado (R6.7); cada use case chama `time.Now().UTC()` inline e calcula a competência BR com `time.LoadLocation("America/Sao_Paulo")` resolvido no boot e mantido em memória.

Exemplo de assinatura de use case com UoW (espelha `internal/billing/application/usecases/process_sale_approved.go`):

```go
type UpsertExpense struct {
    expenses   interfaces.ExpenseRepository
    budgets    interfaces.BudgetRepository
    categories interfaces.CategoriesReader
    publisher  interfaces.ExpenseCommittedPublisher
    uow        uow.UnitOfWork[entities.Expense]
    o11y       observability.Observability
}

func NewUpsertExpense(
    expenses interfaces.ExpenseRepository,
    budgets interfaces.BudgetRepository,
    categories interfaces.CategoriesReader,
    publisher interfaces.ExpenseCommittedPublisher,
    u uow.UnitOfWork[entities.Expense],
    o11y observability.Observability,
) *UpsertExpense { /* ... */ }

func (uc *UpsertExpense) Execute(ctx context.Context, in input.UpsertExpense) (entities.Expense, error) {
    return uc.uow.Do(ctx, func(ctx context.Context, tx database.DBTX) (entities.Expense, error) {
        // 1. validar subcategoria via uc.categories
        // 2. ler estado canônico via uc.expenses.GetByIdentity
        // 3. uc.budgets.GetOrAutoDraft
        // 4. uc.expenses.Insert/Update
        // 5. uc.publisher.Publish(ctx, tx, envelope)
        return e, nil
    }).(...)
}
```

### Modelos de Dados

#### `budgets`

```sql
CREATE TABLE budgets (
    id              UUID PRIMARY KEY,
    user_id         UUID NOT NULL,
    competence      CHAR(7) NOT NULL,                  -- 'YYYY-MM' (RT-27)
    total_cents     BIGINT NOT NULL DEFAULT 0,         -- >= 0; activation exige > 0
    state           SMALLINT NOT NULL,                 -- 1=draft, 2=active (iota+1)
    activated_at    TIMESTAMPTZ NULL,
    auto_draft      BOOLEAN NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ NOT NULL,
    updated_at      TIMESTAMPTZ NOT NULL,
    CONSTRAINT budgets_user_comp_uk UNIQUE (user_id, competence)
);
CREATE INDEX budgets_competence_idx ON budgets (competence);
```

#### `budgets_allocations`

```sql
CREATE TABLE budgets_allocations (
    budget_id       UUID NOT NULL REFERENCES budgets(id) ON DELETE CASCADE,
    root_slug       TEXT NOT NULL,                     -- enum dos 5 slugs (CHECK)
    basis_points    INTEGER NOT NULL CHECK (basis_points >= 0 AND basis_points <= 10000),
    planned_cents   BIGINT NOT NULL,                   -- calculado em ativação (RF-10/RF-11)
    PRIMARY KEY (budget_id, root_slug),
    CONSTRAINT budgets_allocations_root_chk CHECK (root_slug IN
        ('expense.custo_fixo','expense.conhecimento','expense.prazeres','expense.metas','expense.liberdade_financeira'))
);
```

#### `budgets_expenses`

```sql
CREATE TABLE budgets_expenses (
    id                       UUID PRIMARY KEY,
    user_id                  UUID NOT NULL,
    source                   TEXT NOT NULL,            -- allowlist + 'api'
    external_transaction_id  TEXT NOT NULL,            -- case-sensitive (RT-26)
    subcategory_id           UUID NOT NULL,            -- FK lógica para categories
    root_slug                TEXT NOT NULL,            -- desnormalizado p/ agregação rápida (RT-29)
    competence               CHAR(7) NOT NULL,
    amount_cents             BIGINT NOT NULL CHECK (amount_cents > 0),
    occurred_at              TIMESTAMPTZ NOT NULL,
    version                  BIGINT NOT NULL,          -- monotônica (RF-29c)
    tombstone_version        BIGINT NULL,              -- preenchida na exclusão (RF-29e)
    deleted_at               TIMESTAMPTZ NULL,         -- soft-delete físico (ADR-004)
    created_at               TIMESTAMPTZ NOT NULL,
    updated_at               TIMESTAMPTZ NOT NULL,
    CONSTRAINT budgets_expenses_identity_uk UNIQUE (user_id, source, external_transaction_id)
);
CREATE INDEX budgets_expenses_summary_idx
    ON budgets_expenses (user_id, competence, subcategory_id)
    WHERE deleted_at IS NULL;
CREATE INDEX budgets_expenses_summary_root_idx
    ON budgets_expenses (user_id, competence, root_slug)
    WHERE deleted_at IS NULL;
CREATE INDEX budgets_expenses_deleted_at_idx
    ON budgets_expenses (deleted_at)
    WHERE deleted_at IS NOT NULL;
```

A unicidade `(user_id, source, external_transaction_id)` cobre identidade canônica (RF-42), incluindo linhas com `deleted_at IS NOT NULL` (tombstone) — RF-45/RF-47b. O reuso pós-expurgo de 24m é garantido pelo job de retenção que **DELETE físico** das linhas com `deleted_at < now() - interval '24 months'`. `root_slug` é desnormalizado no commit a partir de `CategoriesReader.ValidateExpenseSubcategory`; sem JOIN no caminho de leitura quente.

#### `budgets_threshold_states`

```sql
CREATE TABLE budgets_threshold_states (
    user_id                       UUID NOT NULL,
    competence                    CHAR(7) NOT NULL,
    root_slug                     TEXT NOT NULL,
    threshold                     SMALLINT NOT NULL CHECK (threshold IN (80, 100)),
    currently_crossed             BOOLEAN NOT NULL DEFAULT FALSE,
    version                       BIGINT NOT NULL DEFAULT 0,
    last_crossed_at               TIMESTAMPTZ NULL,
    last_uncrossed_at             TIMESTAMPTZ NULL,
    last_evaluated_committed_at   TIMESTAMPTZ NULL,
    PRIMARY KEY (user_id, competence, root_slug, threshold)
);
```

UPSERT é idempotente: linha por chave existente; `version` incrementa **apenas** em transição real. Avaliador atrasado com `committed_at` < `last_evaluated_committed_at` ignora a atualização (out-of-order safe).

#### `budgets_alerts`

```sql
CREATE TABLE budgets_alerts (
    id              UUID PRIMARY KEY,
    user_id         UUID NOT NULL,
    competence      CHAR(7) NOT NULL,
    root_slug       TEXT NOT NULL,
    threshold       SMALLINT NOT NULL CHECK (threshold IN (80, 100)),
    state           SMALLINT NOT NULL,                 -- 1=pending_delivery, 2=delivered, 3=suppressed_stale, 4=suppressed_retroactive, 5=rate_limited
    triggered_by_committed_at TIMESTAMPTZ NOT NULL,
    spent_cents     BIGINT NOT NULL,
    planned_cents   BIGINT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL
);
CREATE INDEX budgets_alerts_user_comp_idx ON budgets_alerts (user_id, competence, root_slug, threshold);
CREATE INDEX budgets_alerts_listing_idx
    ON budgets_alerts (user_id, created_at DESC)
    WHERE state IN (1, 2);
```

No MVP, RF-64d define `delivered` no instante do INSERT (sem provider externo, OUT-01). O contador RF-61 (10 por tupla) lê com `WHERE state IN (1,2)` antes de inserir; `rate_limited` é registrado mesmo ao atingir o teto.

#### `budgets_expense_events_pending`

```sql
CREATE TABLE budgets_expense_events_pending (
    id                       UUID PRIMARY KEY,
    event_id                 UUID NOT NULL UNIQUE,     -- idempotência cross-source
    source                   TEXT NOT NULL,            -- allowlist
    user_id                  UUID NOT NULL,
    external_transaction_id  TEXT NOT NULL,
    expected_version         BIGINT NOT NULL,
    mutation_kind            SMALLINT NOT NULL,        -- 1=create, 2=update, 3=delete
    payload                  JSONB NOT NULL,
    state                    SMALLINT NOT NULL,        -- 1=pending, 2=applied, 3=failed, 4=expired
    received_at              TIMESTAMPTZ NOT NULL,
    transitioned_at          TIMESTAMPTZ NULL,
    reason                   TEXT NULL
);
CREATE INDEX budgets_pending_state_received_idx
    ON budgets_expense_events_pending (state, received_at)
    WHERE state = 1;
CREATE INDEX budgets_pending_identity_idx
    ON budgets_expense_events_pending (user_id, source, external_transaction_id)
    WHERE state = 1;
```

### Endpoints de API

Todos sob `/api/v1/budgets`, todos com `RequireUser` do `internal/identity` (RF-71/RT-20). `Principal.UserID` extraído do contexto; **qualquer** tentativa de enviar `user_id` no payload é payload inválido (RF-71a).

| Método | Path | Use case | Erros principais |
|---|---|---|---|
| POST | `/api/v1/budgets` | `CreateBudget` | 400 (total inválido), 409 (já existe na competência) |
| POST | `/api/v1/budgets/{competence}/activate` | `ActivateBudget` | 422 (soma ≠ 100% ou total = 0), 409 (já ativo), 404 |
| DELETE | `/api/v1/budgets/{competence}` | `DeleteDraftBudget` | 409 (ativo), 404 |
| POST | `/api/v1/budgets/recurrence` | `CreateRecurrence` | 422 (source_competence inválida), 207 multi-status (RF-21a/b) |
| POST | `/api/v1/budgets/expenses` | `UpsertExpense (create)` | 400 (ext_id formato, subcategory inválida, payload com `user_id`/`source`/`version`), 409 (versão), 200 idempotente |
| PATCH | `/api/v1/budgets/expenses/{id}` | `UpsertExpense (update)` | 409 (versão), 422 (subcategoria), 404 |
| DELETE | `/api/v1/budgets/expenses/{id}` | `DeleteExpense` | 409 (versão), 404 |
| GET | `/api/v1/budgets/{competence}/summary` | `GetMonthlySummary` | 404 (sem orçamento, exceto auto_draft) |
| GET | `/api/v1/budgets/alerts` | `ListAlerts` | cursor-paginated, filtros opcionais |

OpenAPI: `internal/budgets/openapi.yaml`, mesma estratégia de `internal/categories`.

## Pontos de Integração

- **`internal/categories`** (RT-23): única dependência cross-module síncrona. Adapter `categories_reader_adapter.go` chama o use case `ResolveBySlug` exposto pelo `CategoriesModule` (ADR-001). Cache local com TTL 60s; raízes resolvidas no boot — falha de resolução de raízes **impede startup** (RT-31).
- **`internal/identity`** (RF-71): middleware `RequireUser` aplicado ao router; `auth.FromContext(ctx)` para extrair `user_id`.
- **`internal/platform/outbox`** (RT-24): único publisher de eventos internos. Não usar broker externo.
- **`internal/platform/events`** (Dispatcher) + **`internal/platform/worker`** (Manager): registro de consumers e jobs no `cmd/worker/worker.go`.
- **Produtores cross-module externos** (RF-32): allowlist constante em `internal/budgets/infrastructure/config/producers.go`. Consumer único `external_expense_consumer.go` valida `source ∈ allowlist`, delega para `UpsertExpense`. Sem broker; eventos chegam via `events.Dispatcher` interno (publicados por outros módulos via outbox compartilhado).

## Abordagem de Testes

### Testes Unitários

- **Domínio**: `entities/*`, `valueobjects/*`, `services/allocation_distributor` (table-driven cobrindo half-even + ordem determinística de centavos residuais — RF-11a) e `services/threshold_evaluator` (transições `below/at_threshold/above` para 80% e 100%).
- **Use cases**: mockery (já em uso) gera mocks para os repositórios e o `CategoriesReader`. Casos críticos:
  - `UpsertExpense`: idempotência por identidade canônica, conflito de versão (RF-29b), tombstone bloqueia recriação (RF-45), payload com `version` explícito rejeitado (RF-29d), criação dispara `auto_draft` (RF-12) **só** quando o commit financeiro persiste.
  - `EvaluateAlert`: suprime quando budget != ACTIVE (RF-57); suprime quando estado atual da categoria já não cruza (RF-56b); rate_limit por tupla (RF-61); retroativo registra `suppressed_retroactive` (RF-60c).
  - `CreateRecurrence`: source válida com 100% (RF-23a), até 12 meses (RF-19), respeita já-ativados (RF-24), retorna por competência (RF-21a/b).
  - `DeleteDraftBudget`: rejeita ativo (RF-09c).
- **Adapters**: handlers HTTP testados com `httptest`, focando em decodificação, mapeamento de erro → status e ausência de regra de negócio (R-ADAPTER-001.2 verificado por revisão).

### Testes de Integração

Adotados — atendemos os três critérios do template: (a) o caminho de escrita financeira é tx-crítica com outbox no mesmo commit; (b) histórico de incidentes em billing já demonstrou divergência mock vs Postgres real; (c) o repositório já tem `internal/platform/testcontainer`. Build tag `//go:build integration`.

Suítes obrigatórias:

- `repositories/postgres/expense_repository_integration_test.go`: unicidade `(user, source, ext_id)` cobre tombstone; índice composto parcial usado pelo `SumByRoot` (com `EXPLAIN`); soft-delete e expurgo físico.
- `repositories/postgres/threshold_state_repository_integration_test.go`: UPSERT idempotente; ordenação por `committed_at` para descarte de out-of-order; incremento de `version` somente em transição.
- `repositories/postgres/pending_event_repository_integration_test.go`: máquina de estados, índice parcial `WHERE state = 1`, idempotência por `event_id`.
- `messaging/database/producers/expense_committed_publisher_integration_test.go`: linha de outbox inserida na mesma tx da despesa; rollback simultâneo.
- `messaging/database/consumers/expense_committed_consumer_integration_test.go`: caminho completo commit → dispatcher → consumer → alerta.
- `jobs/handlers/pending_events_reaper_integration_test.go`: aplica quando versão fica satisfeita; expira após 24h; reentrante.
- `jobs/handlers/retention_purge_integration_test.go`: expurga apenas elegíveis; bloqueia se houver pendente não terminal (RF-67a).

### Testes E2E

Não necessário no MVP. O fluxo cross-módulo (categories ↔ budgets, identity → budgets) é coberto pelos integration tests com Postgres real + DI manual. Sem frontend (OUT-14).

## Sequenciamento de Desenvolvimento

### Ordem de Build

1. **Migração de schema** (`migrations/000009_create_budgets_baseline.up.sql`) — todas as 6 tabelas + índices acima; é fundação para qualquer teste de integração.
2. **Domínio puro** (`domain/entities`, `domain/valueobjects`, `domain/services`) — testável sem IO; valida regras de invariantes e distribuição determinística.
3. **`CategoriesReader` + adapter** — bloqueia ativação e UpsertExpense; precisa ser estabilizado antes dos use cases de escrita.
4. **Repositórios Postgres** + integration tests por repo.
5. **Use cases de orçamento** (`CreateBudget`, `ActivateBudget`, `DeleteDraftBudget`, `CreateRecurrence`) — sem dependência de outbox.
6. **`OutboxPublisher` adapter + `UpsertExpense` + `DeleteExpense`** — desbloqueia ingestão financeira.
7. **`GetMonthlySummary`** — desbloqueia validação manual; depende só de despesas + budget.
8. **Avaliador assíncrono**: consumer + `EvaluateAlert` + `ThresholdStateRepository` + `AlertRepository` + `ListAlerts`.
9. **Eventos externos**: `external_expense_consumer` + `pending_event_repository` + `apply_pending_event`.
10. **Jobs**: `pending_events_reaper`, `abandoned_draft_reaper`, `retention_purge`.
11. **`module.go` + wiring em `cmd/server/server.go` e `cmd/worker/worker.go`** — composição final.
12. **OpenAPI + smoke load test + dashboards Grafana** — validação operacional pré-liberação.

### Dependências Técnicas Bloqueantes

- `internal/categories` em produção cumprindo RT-23 (raízes oficiais por slug + validação de subcategoria com `deprecated_at`). Sem isso o boot de budgets falha por design.
- `internal/identity` E1 task 9.0 já merged (HEAD atual confirma) — `auth.FromContext`/`RequireUser` disponíveis.
- Comprovação de QA-04 (RPO 15min / RTO 4h) antes da liberação produtiva (já listada no PRD).

## Monitoramento e Observabilidade

Métricas Prometheus (cardinalidade limitada — RT-25; labels permitidos: `module`, `root_slug`, `competence`, `state`, `source`, `threshold`):

- `budgets_expense_mutations_total{mutation_kind,source,outcome}` — contador de criações/edições/exclusões; outcome ∈ `committed|conflict|rejected|tombstoned`.
- `budgets_expense_mutation_latency_seconds{mutation_kind}` — histograma p50/p95/p99 do caminho síncrono (alvo p95 ≤ 300ms — M-05).
- `budgets_pending_events_total{state,source}` + `budgets_pending_oldest_seconds{source}` — RF-39c.
- `budgets_threshold_transitions_total{root_slug,threshold,direction}` — direction ∈ `crossed|uncrossed`.
- `budgets_alerts_emitted_total{root_slug,threshold,state}` — state da máquina RF-64a.
- `budgets_alert_evaluation_lag_seconds` — gap entre `committed_at` e instante da avaliação.
- `budgets_abandoned_drafts_total` — emitida pelo cron RF-18b.
- `budgets_retention_purged_total{table}` + `budgets_retention_purge_deferred_total{reason}` — RF-67a/b.

Logs estruturados (`log/slog`) em níveis:

- `INFO`: mutação aceita (sem payload financeiro completo — RF-69), transição de threshold, job start/end.
- `WARN`: conflito de versão, evento duplicado, suprimido (todos com `reason`), `rate_limited`, source fora da allowlist.
- `ERROR`: falha de IO, indisponibilidade de `CategoriesReader` em validação (RT-18 degrada gracefully com erro 503 nas validações novas, consultas seguem).

Traces (OTel): spans em handlers, use cases, repositórios e consumer. Atributos de alta cardinalidade (`user_id`, `external_transaction_id`, `subcategory_id`) permanecem em traces (RT-25) sob política PII vigente.

Dashboards Grafana: replicar layout dos dashboards de billing (já em uso) substituindo séries.

## Considerações Técnicas

### Decisões Chave

| ADR | Decisão |
|---|---|
| [ADR-001](./adr-001-categories-reader-consumer-defined.md) | Integração com `internal/categories` via interface consumer-defined em budgets |
| [ADR-002](./adr-002-outbox-event-unico-expense-committed.md) | Outbox interno unificado `budgets.expense.committed.v1` |
| [ADR-003](./adr-003-threshold-state-tabela-dedicada.md) | Estado de cruzamento em tabela dedicada com versão monotônica |
| [ADR-004](./adr-004-summary-on-demand-indice-parcial.md) | Resumo on-demand com índice composto parcial `WHERE deleted_at IS NULL` |
| [ADR-005](./adr-005-pending-events-reaper-job.md) | Job periódico para retomada e expiry de eventos pendentes |

### Riscos Conhecidos

1. **Atraso na avaliação assíncrona quando o dispatcher de outbox enfileira muito**: alertas podem demorar; impacto financeiro nulo (M-03 garantido pelo recálculo do resumo). Mitigação: dashboard `budgets_alert_evaluation_lag_seconds` + alerta em > 5 min p95.
2. **Re-trabalho de avaliação por replay at-least-once**: protegido por `threshold_state.version` + `last_evaluated_committed_at`; replays não geram alerta duplicado.
3. **Crescimento de `budgets_threshold_states`**: 10k usuários × 12 meses × 5 raízes × 2 limiares = 1,2M linhas no horizonte de retenção — confortável; expurgo segue o `retention_purge`.
4. **Indisponibilidade de `internal/categories`** (RT-18): degrada apenas validações novas (RF-04d retorna 503) e novas ativações; consultas/resumos/alertas baseados em dados já persistidos permanecem operacionais. Boot falha se as 5 raízes não resolverem (intencional).
5. **Recorrência interage com `source_competence` em rascunho**: rejeitar rascunho automático sem alocações (RF-23a) cobre o caso; teste dedicado.
6. **Drift de hash do PRD**: este techspec carrega o hash de v24 no cabeçalho. Editar `prd.md` requer rebump (`create-technical-specification` re-executado ou hash atualizado manualmente).

### Conformidade com Padrões

- `.claude/rules/governance.md` — R-GOV-001 honrada (decisões com evidência, sem ações destrutivas).
- `.claude/rules/go-adapters.md` — R-ADAPTER-001.1 (zero comentários), R-ADAPTER-001.2 (adapters finos: handlers/consumers/jobs/producers), R-ADAPTER-001.3 (matriz de referências go-implementation por adapter).
- `AGENTS.md` — Padrão Obrigatório de Módulo (DI manual em `module.go` estilo `BillingModule`), Layout Obrigatório por Módulo, regras R0–R7 do `go-implementation`.
- `internal/platform/outbox` — handlers idempotentes por `event.ID` (consumer `expense_committed_consumer` cumpre).
- Política de comentários **inegociável**: nenhum arquivo `.go` de produção carrega comentários (exceções: `// Code generated`, `//go:build`, `//nolint:` com justificativa inline).

### Arquivos Relevantes e Dependentes

Criados:

- `migrations/000009_create_budgets_baseline.up.sql` + `.down.sql`
- Todo o subtree `internal/budgets/`
- `internal/budgets/openapi.yaml`
- 5 ADRs em `.specs/prd-budgets-monthly/adr-00X-*.md`

Alterados:

- `cmd/server/server.go` — registrar `BudgetsRouter`.
- `cmd/worker/worker.go` — registrar consumers, jobs e event handlers do `BudgetsModule`.
- `configs/` — adicionar `BudgetsConfig` (pelo menos: `PendingEventsScanInterval`, `PendingEventTTL`, `AbandonedDraftCron`, `RetentionPurgeCron`).
- `internal/categories/module.go` — expor o use case consumido por `CategoriesReaderAdapter` (`ResolveBySlug` + `ValidateExpenseSubcategory` se não existir).
- `mockery.yml` — incluir interfaces de budgets.

Não alterados (consumidos apenas):

- `internal/platform/outbox/*`, `internal/platform/events/*`, `internal/platform/worker/*`, `internal/platform/testcontainer/*`
- `internal/identity/application/auth/*`, `internal/identity/infrastructure/http/server/middleware/require_user.go`
