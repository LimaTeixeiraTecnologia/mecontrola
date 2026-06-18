# Plano E2E — Módulo `internal/budgets`

Data: 2026-06-18
Skill obrigatória: go-implementation

---

## 1. Inventário Confirmado

### 1.1 Endpoints HTTP

| Método | Rota | Handler | Status Possíveis |
|--------|------|---------|-----------------|
| POST | `/api/v1/budgets` | `CreateBudgetHandler` | 201, 400, 401, 409, 422, 500 |
| POST | `/api/v1/budgets/recurrence` | `CreateRecurrenceHandler` | 207, 400, 401, 422, 500 |
| GET | `/api/v1/budgets/alerts` | `ListAlertsHandler` | 200, 400, 401, 500 |
| POST | `/api/v1/budgets/expenses` | `UpsertExpenseHandler.HandleCreate` | 201, 400, 401, 409, 500 |
| PATCH | `/api/v1/budgets/expenses/{id}` | `UpsertExpenseHandler.HandleUpdate` | 200, 400, 401, 409, 500 |
| DELETE | `/api/v1/budgets/expenses/{id}` | `DeleteExpenseHandler` | 204, 400, 401, 404, 409, 500 |
| POST | `/api/v1/budgets/{competence}/activate` | `ActivateBudgetHandler` | 200, 400, 401, 404, 409, 422, 500 |
| DELETE | `/api/v1/budgets/{competence}` | `DeleteBudgetHandler` | 204, 400, 401, 404, 409, 500 |
| GET | `/api/v1/budgets/{competence}/summary` | `GetMonthlySummaryHandler` | 200, 400, 401, 404, 500 |

### 1.2 Use Cases (18)

`CreateBudget`, `ActivateBudget`, `DeleteDraftBudget`, `CreateRecurrence`, `UpsertExpense`,
`DeleteExpense`, `GetMonthlySummary`, `ListAlerts`, `EvaluateAlert`, `IngestExternalExpense`,
`CreateOrAutoDraftForExpense`, `ApplyPendingEvent`, `EvaluateThresholdAlerts`,
`NotifyThresholdAlert`, `SignalAbandonedDrafts`, `RunPendingEventsReaper`, `PurgeRetention`.

### 1.3 Eventos de Domínio + Producers

| Event Type | Aggregate | Publisher | Use Case Publicante |
|-----------|-----------|-----------|---------------------|
| `budgets.expense.committed.v1` | `budgets.expense` | `ExpenseCommittedPublisher` | `UpsertExpense`, `DeleteExpense` |
| `budgets.budget_activated.v1` | `budgets.budget` | `BudgetActivatedPublisher` | `ActivateBudget` |
| `budgets.threshold_alert_triggered.v1` | `budgets.threshold_alert` | `ThresholdAlertPublisher` | `EvaluateThresholdAlerts` |

### 1.4 Consumers (6)

| Consumer | Evento Consumido | Idempotência |
|----------|-----------------|--------------|
| `ExpenseCommittedConsumer` | `budgets.expense.committed.v1` | via `EvaluateAlert` |
| `TransactionCreatedConsumer` | `transactions.transaction.created.v1` | aggregate_id (transaction_id) |
| `TransactionDeletedConsumer` | `transactions.transaction.deleted.v1` | aggregate_id (transaction_id) |
| `ExternalExpenseConsumer` | `external.expense.v1` | `event_id` no payload |
| `OnboardingBudgetConsumer` | `onboarding.splits_calculated` | user_id |
| `ThresholdAlertNotifier` | `budgets.threshold_alert_triggered.v1` | `(user_id, budget_id, kind, ref_day)` em `budget_alerts_sent` |

### 1.5 Jobs (4)

| Job | Schedule | Use Case | Idempotência |
|-----|----------|----------|-------------|
| `AbandonedDraftReaper` | `0 3 * * *` | `SignalAbandonedDrafts` | `InsertIfAbsent` em `budgets_abandoned_draft_signals` |
| `PendingEventsReaper` | configurável | `RunPendingEventsReaper` | estado `Applied`/`Failed`/`Expired` em `budgets_expense_events_pending` |
| `RetentionPurge` | `0 4 1 * *` | `PurgeRetention` | idempotente por `olderThan` + `LIMIT` |
| `ThresholdAlertsJob` | `@hourly` | `EvaluateThresholdAlerts` | `InsertIfNotExist` em `budget_alerts_sent` |

### 1.6 Tabelas

`mecontrola.budgets`, `mecontrola.budgets_allocations`, `mecontrola.budgets_expenses`,
`mecontrola.budgets_expenses_tombstone`, `mecontrola.budgets_threshold_states`,
`mecontrola.budgets_alerts`, `mecontrola.budgets_expense_events_pending`,
`mecontrola.budgets_abandoned_draft_signals`, `mecontrola.budget_alerts_sent`

---

## 2. Gaps de Cobertura

| Camada | Arquivo Ausente | Prioridade |
|--------|----------------|-----------|
| Handler Unit | `activate_budget_handler_test.go` | Alta |
| Handler Unit | `delete_budget_handler_test.go` | Alta |
| Handler Unit | `create_recurrence_handler_test.go` | Alta |
| Handler Unit | `upsert_expense_handler_test.go` | Alta |
| Handler Unit | `delete_expense_handler_test.go` | Alta |
| Consumer Integration | `transaction_deleted_consumer_integration_test.go` | Alta |
| Consumer Integration | `expense_committed_consumer_integration_test.go` | Alta |
| Consumer Integration | Todos os 6 consumers precisam de teste `integration` c/ DB real | Alta |
| Producer Integration | `threshold_alert_publisher_integration_test.go` | Alta |
| Job Integration | `abandoned_draft_reaper_integration_test.go` | Alta |
| Job Integration | `pending_events_reaper_integration_test.go` | Alta |
| Job Integration | `retention_purge_integration_test.go` | Alta |
| Job Integration | `threshold_alerts_job_integration_test.go` | Alta |
| E2E godog | `internal/budgets/e2e/` (diretório completo) | Alta |

**Cobertura existente ✅:**
- Testes unitários de domínio (commands, entities, services, value objects) — bem cobertos
- Testes unitários de use cases — bem cobertos
- Testes de integração de repositórios — cobertos (`budget`, `expense`, `alert`, `threshold_state`, `threshold_alert_sent`, `pending_event`)
- Testes unitários de consumers (sem DB real) — cobertos
- Testes unitários de producers (sem DB real) — cobertos
- Teste de integração: `transaction_to_budget_chain_integration_test.go` — 1 fluxo

---

## 3. Estrutura de Pastas Proposta

```
internal/budgets/
├── e2e/
│   ├── features/
│   │   ├── f01_create_budget.feature
│   │   ├── f02_activate_budget.feature
│   │   ├── f03_delete_draft_budget.feature
│   │   ├── f04_upsert_expense.feature
│   │   ├── f05_delete_expense.feature
│   │   ├── f06_create_recurrence.feature
│   │   ├── f07_get_monthly_summary.feature
│   │   ├── f08_list_alerts.feature
│   │   ├── f09_transaction_consumer.feature
│   │   ├── f10_external_expense_consumer.feature
│   │   ├── f11_onboarding_consumer.feature
│   │   └── f12_threshold_alerts_job.feature
│   ├── ctx_test.go           -- contexto compartilhado, helpers HTTP, helpers DB
│   ├── suite_test.go         -- TestE2E, wiring do módulo, godog runner
│   ├── steps_budget_test.go  -- steps de budget (create, activate, delete)
│   ├── steps_expense_test.go -- steps de expense (upsert, delete)
│   ├── steps_recurrence_test.go
│   ├── steps_summary_test.go
│   ├── steps_alerts_test.go
│   ├── steps_consumer_test.go -- steps de consumers (transaction, external, onboarding)
│   ├── steps_jobs_test.go     -- steps de jobs (threshold, abandoned draft)
│   └── steps_shared_test.go   -- steps reutilizáveis (status HTTP, outbox, banco)
│
└── infrastructure/
    ├── http/server/handlers/
    │   ├── activate_budget_handler_test.go    -- NOVO
    │   ├── delete_budget_handler_test.go      -- NOVO
    │   ├── create_recurrence_handler_test.go  -- NOVO
    │   ├── upsert_expense_handler_test.go     -- NOVO
    │   └── delete_expense_handler_test.go     -- NOVO
    ├── messaging/database/
    │   ├── consumers/
    │   │   ├── transaction_deleted_consumer_test.go           -- NOVO (unit)
    │   │   ├── expense_committed_consumer_test.go             -- NOVO (unit)
    │   │   ├── transaction_created_consumer_integration_test.go -- NOVO (integration)
    │   │   ├── transaction_deleted_consumer_integration_test.go -- NOVO (integration)
    │   │   ├── external_expense_consumer_integration_test.go  -- NOVO (integration)
    │   │   ├── onboarding_budget_consumer_integration_test.go -- NOVO (integration)
    │   │   ├── expense_committed_consumer_integration_test.go -- NOVO (integration)
    │   │   └── threshold_alert_notifier_integration_test.go   -- NOVO (integration)
    │   └── producers/
    │       └── threshold_alert_publisher_integration_test.go  -- NOVO (integration)
    └── jobs/handlers/
        ├── abandoned_draft_reaper_integration_test.go    -- NOVO
        ├── pending_events_reaper_integration_test.go     -- NOVO
        ├── retention_purge_integration_test.go           -- NOVO
        └── threshold_alerts_job_integration_test.go      -- NOVO
```

---

## 4. Arquivos Gherkin (.feature) — PT-BR

### f01_create_budget.feature

```gherkin
# language: pt
Funcionalidade: Criação de orçamento

  Cenário: criar orçamento com sucesso retorna 201 e persiste no banco
    Dado que o ambiente de teste para budgets está pronto
    Quando o usuário autenticado cria um orçamento válido para a competência "2025-01" com total de "100000" centavos
    Então a resposta HTTP deve ter status 201
    E o banco deve conter exatamente 1 orçamento para o usuário na competência "2025-01"
    E o orçamento no banco deve ter estado "draft"

  Cenário: criar orçamento com payload inválido retorna 400
    Dado que o ambiente de teste para budgets está pronto
    Quando o usuário autenticado envia um payload inválido para criação de orçamento
    Então a resposta HTTP deve ter status 400

  Cenário: criar orçamento sem autenticação retorna 401
    Dado que o ambiente de teste para budgets está pronto
    Quando o usuário não autenticado tenta criar um orçamento
    Então a resposta HTTP deve ter status 401

  Cenário: criar orçamento duplicado retorna 409
    Dado que o ambiente de teste para budgets está pronto
    E que já existe um orçamento para o usuário na competência "2025-02"
    Quando o usuário autenticado cria um orçamento válido para a competência "2025-02" com total de "50000" centavos
    Então a resposta HTTP deve ter status 409
    E o banco deve conter exatamente 1 orçamento para o usuário na competência "2025-02"

  Cenário: criar orçamento com alocações com soma inválida retorna 422
    Dado que o ambiente de teste para budgets está pronto
    Quando o usuário autenticado cria um orçamento com alocações cuja soma não é 10000 basis points
    Então a resposta HTTP deve ter status 422
```

### f02_activate_budget.feature

```gherkin
# language: pt
Funcionalidade: Ativação de orçamento

  Cenário: ativar orçamento rascunho persiste estado active e publica evento no outbox
    Dado que o ambiente de teste para budgets está pronto
    E que existe um orçamento rascunho para o usuário na competência "2025-03"
    Quando o usuário autenticado ativa o orçamento da competência "2025-03"
    Então a resposta HTTP deve ter status 200
    E o banco deve conter o orçamento da competência "2025-03" com estado "active"
    E a tabela outbox_events deve conter 1 evento do tipo "budgets.budget_activated.v1" para o orçamento
    E o evento no outbox deve conter o campo "user_id" correto

  Cenário: ativar orçamento inexistente retorna 404
    Dado que o ambiente de teste para budgets está pronto
    Quando o usuário autenticado tenta ativar o orçamento da competência "2099-12"
    Então a resposta HTTP deve ter status 404

  Cenário: ativar orçamento já ativo retorna 409
    Dado que o ambiente de teste para budgets está pronto
    E que existe um orçamento ativo para o usuário na competência "2025-04"
    Quando o usuário autenticado tenta ativar o orçamento da competência "2025-04"
    Então a resposta HTTP deve ter status 409

  Cenário: ativar sem autenticação retorna 401
    Dado que o ambiente de teste para budgets está pronto
    Quando o usuário não autenticado tenta ativar o orçamento da competência "2025-03"
    Então a resposta HTTP deve ter status 401
```

### f03_delete_draft_budget.feature

```gherkin
# language: pt
Funcionalidade: Exclusão de orçamento rascunho

  Cenário: excluir orçamento rascunho retorna 204 e remove do banco
    Dado que o ambiente de teste para budgets está pronto
    E que existe um orçamento rascunho para o usuário na competência "2025-05"
    Quando o usuário autenticado exclui o orçamento rascunho da competência "2025-05"
    Então a resposta HTTP deve ter status 204
    E o banco não deve conter orçamento para o usuário na competência "2025-05"

  Cenário: excluir orçamento inexistente retorna 404
    Dado que o ambiente de teste para budgets está pronto
    Quando o usuário autenticado tenta excluir o orçamento da competência "2099-11"
    Então a resposta HTTP deve ter status 404

  Cenário: excluir orçamento ativo retorna 409
    Dado que o ambiente de teste para budgets está pronto
    E que existe um orçamento ativo para o usuário na competência "2025-06"
    Quando o usuário autenticado tenta excluir o orçamento da competência "2025-06"
    Então a resposta HTTP deve ter status 409
```

### f04_upsert_expense.feature

```gherkin
# language: pt
Funcionalidade: Criação e atualização de despesa

  Cenário: criar despesa nova persiste no banco e publica evento no outbox
    Dado que o ambiente de teste para budgets está pronto
    E que existe um orçamento ativo para o usuário na competência "2025-07"
    Quando o usuário autenticado cria uma despesa válida com valor de "5000" centavos na competência "2025-07"
    Então a resposta HTTP deve ter status 201
    E o banco deve conter a despesa com amount_cents "5000" para o usuário
    E a tabela outbox_events deve conter 1 evento do tipo "budgets.expense.committed.v1"
    E o evento no outbox deve ter mutation_kind "debit"

  Cenário: atualizar despesa existente incrementa version e publica evento
    Dado que o ambiente de teste para budgets está pronto
    E que existe uma despesa para o usuário com versão 1 e valor "3000" centavos
    Quando o usuário autenticado atualiza a despesa para "7000" centavos
    Então a resposta HTTP deve ter status 200
    E o banco deve conter a despesa com amount_cents "7000" e version "2"
    E a tabela outbox_events deve conter 1 evento do tipo "budgets.expense.committed.v1"

  Cenário: criar despesa sem autenticação retorna 401
    Dado que o ambiente de teste para budgets está pronto
    Quando o usuário não autenticado tenta criar uma despesa
    Então a resposta HTTP deve ter status 401

  Cenário: criar despesa com payload inválido retorna 400
    Dado que o ambiente de teste para budgets está pronto
    Quando o usuário autenticado envia payload inválido para criação de despesa
    Então a resposta HTTP deve ter status 400

  Cenário: criar despesa duplicada retorna 409
    Dado que o ambiente de teste para budgets está pronto
    E que existe uma despesa para o usuário com source "transactions" e external_id "ext-001"
    Quando o usuário autenticado tenta criar despesa com mesmo source "transactions" e external_id "ext-001"
    Então a resposta HTTP deve ter status 409
    E o banco deve conter exatamente 1 despesa com external_transaction_id "ext-001"
```

### f05_delete_expense.feature

```gherkin
# language: pt
Funcionalidade: Exclusão de despesa

  Cenário: excluir despesa existente realiza soft-delete e publica evento credit
    Dado que o ambiente de teste para budgets está pronto
    E que existe uma despesa para o usuário com versão 1
    Quando o usuário autenticado exclui a despesa com version esperada "1"
    Então a resposta HTTP deve ter status 204
    E o banco deve conter a despesa com deleted_at preenchido
    E o banco deve conter uma linha de tombstone para a despesa
    E a tabela outbox_events deve conter 1 evento do tipo "budgets.expense.committed.v1" com mutation_kind "credit"

  Cenário: excluir despesa inexistente retorna 404
    Dado que o ambiente de teste para budgets está pronto
    Quando o usuário autenticado tenta excluir uma despesa inexistente
    Então a resposta HTTP deve ter status 404

  Cenário: excluir despesa com version incorreta retorna 409
    Dado que o ambiente de teste para budgets está pronto
    E que existe uma despesa para o usuário com versão 2
    Quando o usuário autenticado tenta excluir a despesa com version esperada "1"
    Então a resposta HTTP deve ter status 409

  Cenário: despesa excluída some da listagem mas tombstone bloqueia recriação
    Dado que o ambiente de teste para budgets está pronto
    E que existe uma despesa soft-deleted para o usuário com source "transactions" e external_id "ext-002"
    Quando o usuário autenticado tenta criar despesa com source "transactions" e external_id "ext-002"
    Então a resposta HTTP deve ter status 409
```

### f06_create_recurrence.feature

```gherkin
# language: pt
Funcionalidade: Criação de recorrência de orçamento

  Cenário: criar recorrência a partir de orçamento ativo cria rascunhos futuros
    Dado que o ambiente de teste para budgets está pronto
    E que existe um orçamento ativo para o usuário na competência "2025-01"
    Quando o usuário autenticado cria recorrência de "3" meses a partir de "2025-01"
    Então a resposta HTTP deve ter status 207
    E o banco deve conter 3 novos orçamentos rascunho para o usuário

  Cenário: criar recorrência sem orçamento fonte retorna 422
    Dado que o ambiente de teste para budgets está pronto
    Quando o usuário autenticado tenta criar recorrência a partir de competência inexistente "2099-01"
    Então a resposta HTTP deve ter status 422
```

### f07_get_monthly_summary.feature

```gherkin
# language: pt
Funcionalidade: Resumo mensal de orçamento

  Cenário: obter resumo mensal de competência com orçamento ativo
    Dado que o ambiente de teste para budgets está pronto
    E que existe um orçamento ativo com despesas para o usuário na competência "2025-08"
    Quando o usuário autenticado solicita o resumo da competência "2025-08"
    Então a resposta HTTP deve ter status 200
    E a resposta deve conter o campo "competence" com valor "2025-08"
    E a resposta deve conter o campo "total_cents" maior que zero

  Cenário: obter resumo de competência inexistente retorna 404
    Dado que o ambiente de teste para budgets está pronto
    Quando o usuário autenticado solicita o resumo da competência "2099-01"
    Então a resposta HTTP deve ter status 404
```

### f08_list_alerts.feature

```gherkin
# language: pt
Funcionalidade: Listagem de alertas

  Cenário: listar alertas retorna lista paginada
    Dado que o ambiente de teste para budgets está pronto
    E que existem "3" alertas para o usuário
    Quando o usuário autenticado lista os alertas com limit "2"
    Então a resposta HTTP deve ter status 200
    E a resposta deve conter exatamente "2" alertas
    E a resposta deve conter cursor de paginação

  Cenário: listar alertas sem itens retorna lista vazia
    Dado que o ambiente de teste para budgets está pronto
    Quando o usuário autenticado lista os alertas sem nenhum alerta cadastrado
    Então a resposta HTTP deve ter status 200
    E a resposta deve conter lista vazia de alertas

  Cenário: listar alertas sem autenticação retorna 401
    Dado que o ambiente de teste para budgets está pronto
    Quando o usuário não autenticado tenta listar alertas
    Então a resposta HTTP deve ter status 401
```

### f09_transaction_consumer.feature

```gherkin
# language: pt
Funcionalidade: Consumer de transações para despesas de orçamento

  Cenário: processar evento transaction.created cria despesa no banco
    Dado que o ambiente de teste para budgets está pronto
    E que existe um orçamento ativo para o usuário na competência "2025-09"
    Quando o consumer recebe um evento "transactions.transaction.created.v1" válido para o usuário
    Então o banco deve conter 1 despesa criada para o aggregate_id do evento
    E o banco deve conter 1 evento do tipo "budgets.expense.committed.v1" em outbox_events

  Cenário: reprocessar o mesmo evento transaction.created é idempotente
    Dado que o ambiente de teste para budgets está pronto
    E que o consumer já processou um evento "transactions.transaction.created.v1" com aggregate_id "txn-001"
    Quando o consumer reprocessa o mesmo evento com aggregate_id "txn-001"
    Então o banco deve conter exatamente 1 despesa com external_transaction_id "txn-001"

  Cenário: processar evento transaction.deleted realiza soft-delete da despesa
    Dado que o ambiente de teste para budgets está pronto
    E que existe uma despesa com source "transactions" e external_id "txn-002"
    Quando o consumer recebe um evento "transactions.transaction.deleted.v1" para "txn-002"
    Então o banco deve conter a despesa "txn-002" com deleted_at preenchido
    E o banco deve conter tombstone para a despesa "txn-002"
```

### f10_external_expense_consumer.feature

```gherkin
# language: pt
Funcionalidade: Consumer de despesas externas com idempotência por event_id

  Cenário: processar evento externo upsert cria despesa via fila de eventos pendentes
    Dado que o ambiente de teste para budgets está pronto
    Quando o consumer recebe um evento "external.expense.v1" de upsert com event_id "ext-evt-001"
    Então o banco deve conter 1 linha em budgets_expense_events_pending para "ext-evt-001"
    E após o reaper processar o evento o banco deve conter 1 despesa criada

  Cenário: reprocessar o mesmo event_id externo não cria duplicata
    Dado que o ambiente de teste para budgets está pronto
    E que o consumer já processou o evento externo com event_id "ext-evt-002"
    Quando o consumer reprocessa o evento com o mesmo event_id "ext-evt-002"
    Então o banco deve conter exatamente 1 linha em budgets_expense_events_pending para "ext-evt-002"

  Cenário: evento externo de delete com event_id único realiza soft-delete
    Dado que o ambiente de teste para budgets está pronto
    E que existe uma despesa externa com source "external" e external_id "ext-003"
    Quando o consumer recebe evento "external.expense.v1" de delete com event_id "ext-evt-003"
    Então após o reaper processar o evento a despesa "ext-003" deve ter deleted_at preenchido
```

### f11_onboarding_consumer.feature

```gherkin
# language: pt
Funcionalidade: Consumer de onboarding cria e ativa orçamento automaticamente

  Cenário: processar evento de onboarding cria e ativa orçamento para o usuário
    Dado que o ambiente de teste para budgets está pronto
    Quando o consumer recebe um evento "onboarding.splits_calculated" para um novo usuário
    Então o banco deve conter 1 orçamento com estado "active" para o usuário
    E a tabela outbox_events deve conter 1 evento do tipo "budgets.budget_activated.v1"

  Cenário: reprocessar evento de onboarding para o mesmo usuário é idempotente
    Dado que o ambiente de teste para budgets está pronto
    E que o consumer já processou o evento de onboarding para o usuário "user-onboard-001"
    Quando o consumer reprocessa o evento de onboarding para "user-onboard-001"
    Então o banco deve conter exatamente 1 orçamento para o usuário "user-onboard-001"
```

### f12_threshold_alerts_job.feature

```gherkin
# language: pt
Funcionalidade: Job de avaliação de alertas de limiar

  Cenário: job detecta limiar ultrapassado e publica evento de alerta
    Dado que o ambiente de teste para budgets está pronto
    E que existe um orçamento ativo com despesas acima do limiar de "80%" em uma categoria
    Quando o job de threshold alerts é executado
    Então a tabela outbox_events deve conter 1 evento do tipo "budgets.threshold_alert_triggered.v1"
    E a tabela budget_alerts_sent deve conter 1 registro para o orçamento

  Cenário: executar job duas vezes não duplica o alerta
    Dado que o ambiente de teste para budgets está pronto
    E que o job já foi executado uma vez para o orçamento com limiar ultrapassado
    Quando o job de threshold alerts é executado novamente
    Então a tabela budget_alerts_sent deve conter exatamente 1 registro para o orçamento
    E a tabela outbox_events não deve conter evento duplicado
```

---

## 5. Definições de Steps — Assinaturas Go (PT-BR regex, inglês nos métodos)

### suite_test.go

```go
//go:build e2e

package e2e_test

import (
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/cucumber/godog"
    "github.com/go-chi/chi/v5"
    "github.com/jmoiron/sqlx"
    "github.com/JailtonJunior94/devkit-go/pkg/observability/noop"

    "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets"
    "github.com/LimaTeixeiraTecnologia/mecontrola/internal/categories"
    "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/postgres"
)

const e2eUserID = "11111111-1111-1111-1111-111111111111"

func TestE2E(t *testing.T) {
    db, _ := postgres.NewTestDatabase(t)
    o11y := noop.NewProvider()

    catModule := categories.NewCategoriesModule(db, o11y, passthroughMiddleware)

    // budgets.NewBudgetsModule requer channelGateway e channelResolver;
    // usar fakes no-op para E2E
    budgetsModule, err := budgets.NewBudgetsModule(
        testConfig(),
        o11y,
        db,
        catModule,
        passthroughMiddleware,
        newNoopChannelGateway(),
        newNoopChannelResolver(),
    )
    if err != nil {
        t.Fatalf("inicializar módulo budgets: %v", err)
    }

    router := chi.NewRouter()
    budgetsModule.BudgetsRouter.Register(router)

    server := httptest.NewServer(router)
    t.Cleanup(server.Close)

    suite := godog.TestSuite{
        Name: "budgets-e2e",
        ScenarioInitializer: func(sc *godog.ScenarioContext) {
            ctx := newBudgetsE2ECtx(server, db, budgetsModule)
            registerAllSteps(sc, ctx)
        },
        Options: &godog.Options{
            Format:   "pretty",
            Paths:    []string{"features"},
            TestingT: t,
        },
    }

    if suite.Run() != 0 {
        t.Fatal("cenários e2e falharam")
    }
}
```

### ctx_test.go — Struct e helpers principais

```go
//go:build e2e

package e2e_test

type budgetsE2ECtx struct {
    server       *httptest.Server
    db           *sqlx.DB
    module       *budgets.BudgetsModule
    lastResp     *http.Response
    lastBody     map[string]any
    lastBodyText string
    lastBudgetID string
    lastExpenseID string
    lastCompetence string
    lastExternalID string
    lastEventID    string
}

// helpers de DB (PT-BR nomes de funções, inglês nos steps)
func (e *budgetsE2ECtx) countBudgets(userID, competence string) (int, error)
func (e *budgetsE2ECtx) budgetState(userID, competence string) (int, error) // 1=draft, 2=active
func (e *budgetsE2ECtx) countExpenses(userID string) (int, error)
func (e *budgetsE2ECtx) expenseDeletedAt(userID, source, externalID string) (bool, error)
func (e *budgetsE2ECtx) countTombstones(userID, source, externalID string) (int, error)
func (e *budgetsE2ECtx) countOutboxByType(eventType string) (int, error)
func (e *budgetsE2ECtx) outboxEventPayload(eventType string) (map[string]any, error)
func (e *budgetsE2ECtx) countPendingEvents(eventID string) (int, error)
func (e *budgetsE2ECtx) countBudgetAlertsSent(budgetID string) (int, error)
func (e *budgetsE2ECtx) insertBudgetDraft(userID, competence string, totalCents int64) error
func (e *budgetsE2ECtx) activateBudget(userID, competence string) error
func (e *budgetsE2ECtx) insertExpense(userID, source, externalID string, amountCents int64, version int64) error
func (e *budgetsE2ECtx) post(path string, body any) error
func (e *budgetsE2ECtx) patch(path string, body any) error
func (e *budgetsE2ECtx) delete(path string) error
func (e *budgetsE2ECtx) get(path string) error
func (e *budgetsE2ECtx) postWithoutAuth(path string, body any) error
```

### steps_shared_test.go — Steps reutilizáveis

```go
func registerSharedSteps(sc *godog.ScenarioContext, e *budgetsE2ECtx) {
    sc.Step(`^que o ambiente de teste para budgets está pronto$`, e.givenTestEnvIsReady)
    sc.Step(`^a resposta HTTP deve ter status (\d+)$`, e.thenHTTPStatusIs)
    sc.Step(`^o banco deve conter exatamente (\d+) orçamento(?:s)? para o usuário na competência "([^"]*)"$`, e.thenBudgetCountForUserCompetenceIs)
    sc.Step(`^o orçamento no banco deve ter estado "([^"]*)"$`, e.thenLastBudgetStateIs)
    sc.Step(`^a tabela outbox_events deve conter (\d+) evento(?:s)? do tipo "([^"]*)"$`, e.thenOutboxCountForTypeIs)
    sc.Step(`^o evento no outbox deve conter o campo "([^"]*)" correto$`, e.thenOutboxEventFieldIsCorrect)
    sc.Step(`^o evento no outbox deve ter mutation_kind "([^"]*)"$`, e.thenOutboxMutationKindIs)
}

func (e *budgetsE2ECtx) givenTestEnvIsReady() error
func (e *budgetsE2ECtx) thenHTTPStatusIs(status int) error
func (e *budgetsE2ECtx) thenBudgetCountForUserCompetenceIs(count int, competence string) error
func (e *budgetsE2ECtx) thenLastBudgetStateIs(state string) error
func (e *budgetsE2ECtx) thenOutboxCountForTypeIs(count int, eventType string) error
func (e *budgetsE2ECtx) thenOutboxEventFieldIsCorrect(field string) error
func (e *budgetsE2ECtx) thenOutboxMutationKindIs(kind string) error
```

### steps_budget_test.go

```go
func registerBudgetSteps(sc *godog.ScenarioContext, e *budgetsE2ECtx) {
    sc.Step(`^que já existe um orçamento para o usuário na competência "([^"]*)"$`, e.givenBudgetExistsForCompetence)
    sc.Step(`^que existe um orçamento rascunho para o usuário na competência "([^"]*)"$`, e.givenDraftBudgetForCompetence)
    sc.Step(`^que existe um orçamento ativo para o usuário na competência "([^"]*)"$`, e.givenActiveBudgetForCompetence)
    sc.Step(`^o usuário autenticado cria um orçamento válido para a competência "([^"]*)" com total de "([^"]*)" centavos$`, e.whenUserCreatesValidBudget)
    sc.Step(`^o usuário autenticado envia um payload inválido para criação de orçamento$`, e.whenUserSendsInvalidBudgetPayload)
    sc.Step(`^o usuário não autenticado tenta criar um orçamento$`, e.whenUnauthenticatedUserCreatesBudget)
    sc.Step(`^o usuário autenticado cria um orçamento com alocações cuja soma não é 10000 basis points$`, e.whenUserCreatesBudgetWithInvalidAllocations)
    sc.Step(`^o usuário autenticado ativa o orçamento da competência "([^"]*)"$`, e.whenUserActivatesBudget)
    sc.Step(`^o usuário não autenticado tenta ativar o orçamento da competência "([^"]*)"$`, e.whenUnauthenticatedUserActivatesBudget)
    sc.Step(`^o usuário autenticado tenta ativar o orçamento da competência "([^"]*)"$`, e.whenUserTriesToActivateBudget)
    sc.Step(`^o usuário autenticado exclui o orçamento rascunho da competência "([^"]*)"$`, e.whenUserDeletesDraftBudget)
    sc.Step(`^o usuário autenticado tenta excluir o orçamento da competência "([^"]*)"$`, e.whenUserTriesToDeleteBudget)
    sc.Step(`^o banco não deve conter orçamento para o usuário na competência "([^"]*)"$`, e.thenNoBudgetForCompetence)
    sc.Step(`^o banco deve conter (\d+) novos orçamentos rascunho para o usuário$`, e.thenNewDraftBudgetsCount)
}
```

### steps_expense_test.go

```go
func registerExpenseSteps(sc *godog.ScenarioContext, e *budgetsE2ECtx) {
    sc.Step(`^que existe uma despesa para o usuário com versão (\d+) e valor "([^"]*)" centavos$`, e.givenExpenseWithVersionAndAmount)
    sc.Step(`^que existe uma despesa para o usuário com source "([^"]*)" e external_id "([^"]*)"$`, e.givenExpenseWithSourceAndExternalID)
    sc.Step(`^que existe uma despesa soft-deleted para o usuário com source "([^"]*)" e external_id "([^"]*)"$`, e.givenSoftDeletedExpense)
    sc.Step(`^o usuário autenticado cria uma despesa válida com valor de "([^"]*)" centavos na competência "([^"]*)"$`, e.whenUserCreatesExpense)
    sc.Step(`^o usuário autenticado envia payload inválido para criação de despesa$`, e.whenUserSendsInvalidExpensePayload)
    sc.Step(`^o usuário não autenticado tenta criar uma despesa$`, e.whenUnauthenticatedUserCreatesExpense)
    sc.Step(`^o usuário autenticado tenta criar despesa com mesmo source "([^"]*)" e external_id "([^"]*)"$`, e.whenUserCreatesDuplicateExpense)
    sc.Step(`^o usuário autenticado atualiza a despesa para "([^"]*)" centavos$`, e.whenUserUpdatesExpense)
    sc.Step(`^o usuário autenticado exclui a despesa com version esperada "([^"]*)"$`, e.whenUserDeletesExpense)
    sc.Step(`^o usuário autenticado tenta excluir uma despesa inexistente$`, e.whenUserDeletesNonexistentExpense)
    sc.Step(`^o usuário autenticado tenta excluir a despesa com version esperada "([^"]*)"$`, e.whenUserDeletesExpenseWithVersion)
    sc.Step(`^o banco deve conter a despesa com amount_cents "([^"]*)" para o usuário$`, e.thenExpenseAmountIs)
    sc.Step(`^o banco deve conter a despesa com amount_cents "([^"]*)" e version "([^"]*)"$`, e.thenExpenseAmountAndVersionIs)
    sc.Step(`^o banco deve conter a despesa com deleted_at preenchido$`, e.thenExpenseSoftDeleted)
    sc.Step(`^o banco deve conter uma linha de tombstone para a despesa$`, e.thenTombstoneExists)
    sc.Step(`^o banco deve conter exatamente (\d+) despesa(?:s)? com external_transaction_id "([^"]*)"$`, e.thenExpenseCountByExternalID)
}
```

### steps_consumer_test.go

```go
func registerConsumerSteps(sc *godog.ScenarioContext, e *budgetsE2ECtx) {
    sc.Step(`^o consumer recebe um evento "([^"]*)" válido para o usuário$`, e.whenConsumerReceivesEventForUser)
    sc.Step(`^o consumer já processou um evento "([^"]*)" com aggregate_id "([^"]*)"$`, e.givenConsumerAlreadyProcessedEvent)
    sc.Step(`^o consumer reprocessa o mesmo evento com aggregate_id "([^"]*)"$`, e.whenConsumerReprocessesEventByAggregateID)
    sc.Step(`^o consumer recebe um evento "([^"]*)" para "([^"]*)"$`, e.whenConsumerReceivesEventForExternalID)
    sc.Step(`^o consumer recebe um evento "([^"]*)" de upsert com event_id "([^"]*)"$`, e.whenConsumerReceivesExternalUpsertEvent)
    sc.Step(`^o consumer já processou o evento externo com event_id "([^"]*)"$`, e.givenExternalEventAlreadyProcessed)
    sc.Step(`^o consumer reprocessa o evento com o mesmo event_id "([^"]*)"$`, e.whenConsumerReprocessesExternalEvent)
    sc.Step(`^o consumer recebe evento "([^"]*)" de delete com event_id "([^"]*)"$`, e.whenConsumerReceivesExternalDeleteEvent)
    sc.Step(`^o consumer recebe um evento "([^"]*)" para um novo usuário$`, e.whenConsumerReceivesOnboardingEvent)
    sc.Step(`^o consumer já processou o evento de onboarding para o usuário "([^"]*)"$`, e.givenOnboardingAlreadyProcessedForUser)
    sc.Step(`^o consumer reprocessa o evento de onboarding para "([^"]*)"$`, e.whenConsumerReprocessesOnboardingEvent)
    sc.Step(`^o banco deve conter (\d+) despesa(?:s)? criada(?:s)? para o aggregate_id do evento$`, e.thenExpenseCountByLastAggregateID)
    sc.Step(`^o banco deve conter a despesa "([^"]*)" com deleted_at preenchido$`, e.thenExpenseSoftDeletedByExternalID)
    sc.Step(`^o banco deve conter tombstone para a despesa "([^"]*)"$`, e.thenTombstoneExistsByExternalID)
    sc.Step(`^o banco deve conter (\d+) linha(?:s)? em budgets_expense_events_pending para "([^"]*)"$`, e.thenPendingEventCount)
    sc.Step(`^após o reaper processar o evento o banco deve conter (\d+) despesa(?:s)? criada(?:s)?$`, e.thenAfterReaperExpenseCount)
    sc.Step(`^após o reaper processar o evento a despesa "([^"]*)" deve ter deleted_at preenchido$`, e.thenAfterReaperExpenseSoftDeleted)
    sc.Step(`^o banco deve conter exatamente (\d+) orçamento(?:s)? para o usuário "([^"]*)"$`, e.thenBudgetCountForNamedUser)
}
```

### steps_jobs_test.go

```go
func registerJobSteps(sc *godog.ScenarioContext, e *budgetsE2ECtx) {
    sc.Step(`^que existe um orçamento ativo com despesas acima do limiar de "([^"]*)" em uma categoria$`, e.givenActiveBudgetAboveThreshold)
    sc.Step(`^que o job já foi executado uma vez para o orçamento com limiar ultrapassado$`, e.givenJobAlreadyRanForBudgetAboveThreshold)
    sc.Step(`^o job de threshold alerts é executado$`, e.whenThresholdAlertsJobRuns)
    sc.Step(`^o job de threshold alerts é executado novamente$`, e.whenThresholdAlertsJobRunsAgain)
    sc.Step(`^a tabela budget_alerts_sent deve conter (\d+) registro(?:s)? para o orçamento$`, e.thenBudgetAlertsSentCount)
    sc.Step(`^a tabela outbox_events não deve conter evento duplicado$`, e.thenNoOutboxDuplicate)
}
```

---

## 6. Testes de Integração por Camada

### 6.1 Handler Unit Tests (sem DB)

Arquivos a criar em `infrastructure/http/server/handlers/`:

| Arquivo | Cenários a cobrir |
|---------|-------------------|
| `activate_budget_handler_test.go` | 200 sucesso, 401 sem auth, 404 not found, 409 conflict, 422 already active |
| `delete_budget_handler_test.go` | 204 sucesso, 401 sem auth, 404 not found, 409 active budget |
| `create_recurrence_handler_test.go` | 207 sucesso, 401 sem auth, 400 inválido, 422 sem orçamento fonte |
| `upsert_expense_handler_test.go` | 201 create, 200 update, 401 sem auth, 400 inválido, 409 duplicata |
| `delete_expense_handler_test.go` | 204 sucesso, 401 sem auth, 404 not found, 409 version conflict |

**Padrão:** seguir `create_budget_handler_test.go` — `testify/suite`, mock inline sem mockery, `auth.WithPrincipal` para injetar principal.

### 6.2 Consumer Integration Tests (DB real)

Arquivos a criar em `infrastructure/messaging/database/consumers/` com `//go:build integration`:

| Arquivo | Cenários mínimos |
|---------|-----------------|
| `transaction_created_consumer_integration_test.go` | cria despesa; idempotência por aggregate_id |
| `transaction_deleted_consumer_integration_test.go` | soft-delete despesa; tombstone criado; idempotência |
| `external_expense_consumer_integration_test.go` | insere em pending_events; idempotência por event_id; upsert e delete |
| `onboarding_budget_consumer_integration_test.go` | cria e ativa budget; idempotência por user_id |
| `expense_committed_consumer_integration_test.go` | cria alerta; idempotência |
| `threshold_alert_notifier_integration_test.go` | insere em budget_alerts_sent; idempotência por (user_id,budget_id,kind,ref_day) |

**Padrão de validação:**
```go
var count int
err := db.QueryRowContext(ctx,
    `SELECT COUNT(*) FROM mecontrola.budgets_expenses
     WHERE user_id = $1 AND source = $2 AND external_transaction_id = $3`,
    userID, source, externalID,
).Scan(&count)
s.Require().NoError(err)
s.Assert().Equal(expectedCount, count)
```

### 6.3 Producer Integration Test (DB real)

`threshold_alert_publisher_integration_test.go` com `//go:build integration`:
- Publica evento na mesma tx → linha em `outbox_events` com `event_type`, `aggregate_id`, `user_id` corretos
- Rollback → linha não persiste

### 6.4 Job Handler Integration Tests (DB real)

| Arquivo | Cenários mínimos |
|---------|-----------------|
| `abandoned_draft_reaper_integration_test.go` | execução dupla → idempotente; `budgets_abandoned_draft_signals` COUNT estável |
| `pending_events_reaper_integration_test.go` | evento Pending → Applied após execução; evento já Applied não muda |
| `retention_purge_integration_test.go` | purga registros antigos; registros recentes preservados |
| `threshold_alerts_job_integration_test.go` | cruza threshold → linha em `budget_alerts_sent`; dupla execução → COUNT estável |

---

## 7. Estratégia de Validação de Banco

### 7.1 Helpers de banco no ctx_test.go

Todos os helpers executam `QueryRowContext` ou `QueryContext` **diretamente na tabela** após a operação:

```go
func (e *budgetsE2ECtx) countOutboxByType(eventType string) (int, error) {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    var n int
    err := e.db.QueryRowContext(ctx,
        `SELECT COUNT(*) FROM mecontrola.outbox_events WHERE event_type = $1`,
        eventType,
    ).Scan(&n)
    return n, err
}

func (e *budgetsE2ECtx) budgetState(userID, competence string) (int, error) {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    var state int
    err := e.db.QueryRowContext(ctx,
        `SELECT state FROM mecontrola.budgets WHERE user_id = $1 AND competence = $2`,
        userID, competence,
    ).Scan(&state)
    return state, err
}
```

### 7.2 Padrão de asserção de estado

| Operação | Asserção obrigatória |
|----------|---------------------|
| Criar orçamento | `COUNT(*) == before+1` na tabela `budgets` |
| Ativar orçamento | `state == 2` + `activated_at IS NOT NULL` |
| Deletar rascunho | `COUNT(*) == before-1` ou `SELECT` retorna erro `ErrNoRows` |
| Criar/atualizar despesa | `amount_cents == expected` + `version == expected` |
| Soft-delete despesa | `deleted_at IS NOT NULL` + tombstone criado |
| Publicar evento | `COUNT(*) em outbox_events` por `event_type` |
| Idempotência consumer | `COUNT` estável após reprocessamento |
| Job idempotência | `COUNT` em tabela de controle estável no 2º run |

### 7.3 Isolamento por cenário

- Cada cenário usa `userID` único (`uuid.New()`) para evitar colisões entre cenários.
- Setup de dados (Dado) sempre usa helpers SQL direto, nunca chamadas HTTP.
- Rollback/cleanup via `t.Cleanup` do `NewTestDatabase`.

---

## 8. Definition of Done Gates

Antes de declarar módulo fechado, todos os gates devem passar:

```bash
# 1. Testes unitários com race detector
task test:unit

# 2. Testes de integração (requer Docker)
task test:integration

# 3. Testes E2E
task test:e2e

# 4. Linting
golangci-lint run ./internal/budgets/...

# 5. Zero comentários em Go de produção (R-ADAPTER-001.1)
grep -rn --include="*.go" --exclude-dir=mocks --exclude="*.pb.go" --exclude="*_test.go" \
  "^[[:space:]]*//" internal/budgets/ \
  | grep -Ev "(//go:|//nolint:|// Code generated)" \
  && echo "FAIL" && exit 1 || true

# 6. Sem SQL direto em adapters (R-ADAPTER-001.2)
grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" \
  "QueryContext\|ExecContext\|db\.Query\|tx\.Exec\|db\.Exec" \
  internal/budgets/infrastructure/http/server/handlers/ \
  internal/budgets/infrastructure/messaging/database/consumers/ \
  internal/budgets/infrastructure/messaging/database/producers/ \
  internal/budgets/infrastructure/jobs/handlers/ \
  && echo "FAIL" && exit 1 || true

# 7. Idempotência por event_id validada em todo consumer (verificar manualmente no relatório)
```

---

## 9. Ordem de Execução — Subagents Paralelos

Ao iniciar implementação, spawnar **1 subagent por camada** em paralelo:

| Subagent | Responsabilidade | Arquivos alvo |
|----------|-----------------|---------------|
| A — Handler | 5 handler unit tests faltantes | `activate_budget_handler_test.go`, `delete_budget_handler_test.go`, `create_recurrence_handler_test.go`, `upsert_expense_handler_test.go`, `delete_expense_handler_test.go` |
| B — Consumer Unit | 2 consumer unit tests faltantes | `transaction_deleted_consumer_test.go`, `expense_committed_consumer_test.go` |
| C — Consumer Integration | 6 consumer integration tests (DB real) | `*_integration_test.go` em `consumers/` |
| D — Producer Integration | 1 producer integration test | `threshold_alert_publisher_integration_test.go` |
| E — Jobs Integration | 4 job integration tests | `*_integration_test.go` em `jobs/handlers/` |
| F — E2E Suite | Suite godog completa | `internal/budgets/e2e/` |

**Síntese final:** após todos os subagents concluírem, executar os 7 gates do Definition of Done e reportar evidências por camada.

---

## 10. Restrições Obrigatórias

- Zero comentários em `.go` de produção — inegociável (R-ADAPTER-001.1)
- Gherkin e regex dos steps em PT-BR
- Métodos Go em inglês
- Build tag `//go:build integration` em todos os testes com Testcontainers
- Build tag `//go:build e2e` em todo o pacote `e2e`
- Idempotência de consumers verificada via `SELECT COUNT(*)` após reprocessamento
- Validação de outbox via SQL direto em `outbox_events`, nunca via retorno de método
- Sem falso positivo: se teste quebrar, corrigir o código, nunca relaxar o teste
