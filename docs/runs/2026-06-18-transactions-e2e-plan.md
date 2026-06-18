# Plano: Cobertura E2E 100% — Módulo `internal/transactions`

**Data:** 2026-06-18
**Skill obrigatória:** `go-implementation`
**Restrição:** zero comentários em `.go`; Gherkin PT-BR; steps Go em inglês

---

## 1. Inventário Completo do Módulo

### 1.1 Endpoints HTTP (`internal/transactions/infrastructure/http/server/transactions_router.go`)

Todos os grupos usam: `rt.gatewayAuth` + `middleware.InjectPrincipalFromHeaderWithO11y` + `middleware.RequireUser`
Operações mutantes usam middleware `idempotency.Middleware`.

| Grupo | Método | Rota | Handler | Status esperados |
|-------|--------|------|---------|-----------------|
| Transactions | POST | `/api/v1/transactions` | CreateTransactionHandler | 201, 400, 401, 422 |
| Transactions | GET | `/api/v1/transactions` | ListTransactionsHandler | 200, 401 |
| Transactions | GET | `/api/v1/transactions/{id}` | GetTransactionHandler | 200, 401, 404 |
| Transactions | PATCH | `/api/v1/transactions/{id}` | UpdateTransactionHandler | 200, 400, 401, 404, 422 |
| Transactions | DELETE | `/api/v1/transactions/{id}` | DeleteTransactionHandler | 204, 401, 404 |
| CardPurchases | POST | `/api/v1/card-purchases` | CreateCardPurchaseHandler | 201, 400, 401, 422 |
| CardPurchases | GET | `/api/v1/card-purchases` | ListCardPurchasesHandler | 200, 401 |
| CardPurchases | GET | `/api/v1/card-purchases/{id}` | GetCardPurchaseHandler | 200, 401, 404 |
| CardPurchases | PATCH | `/api/v1/card-purchases/{id}` | UpdateCardPurchaseHandler | 200, 400, 401, 404, 422 |
| CardPurchases | DELETE | `/api/v1/card-purchases/{id}` | DeleteCardPurchaseHandler | 204, 401, 404 |
| Cards | GET | `/api/v1/cards/{card_id}/invoices/{ref_month}` | GetCardInvoiceHandler | 200, 401, 404 |
| RecurringTemplates | POST | `/api/v1/recurring-templates` | CreateRecurringTemplateHandler | 201, 400, 401, 422 |
| RecurringTemplates | GET | `/api/v1/recurring-templates` | ListRecurringTemplatesHandler | 200, 401 |
| RecurringTemplates | GET | `/api/v1/recurring-templates/{id}` | GetRecurringTemplateHandler | 200, 401, 404 |
| RecurringTemplates | PATCH | `/api/v1/recurring-templates/{id}` | UpdateRecurringTemplateHandler | 200, 400, 401, 404, 422 |
| RecurringTemplates | DELETE | `/api/v1/recurring-templates/{id}` | DeleteRecurringTemplateHandler | 204, 401, 404 |
| Monthly | GET | `/api/v1/months/{ref_month}` | GetMonthlySummaryHandler | 200, 401, 404 |
| Monthly | GET | `/api/v1/months/{ref_month}/entries` | ListMonthlyEntriesHandler | 200, 401 |

### 1.2 Use Cases (`application/usecases/`)

| Use Case | Arquivo | Publisher disparado |
|----------|---------|-------------------|
| CreateTransaction | create_transaction.go | TransactionEventPublisher.PublishCreated |
| UpdateTransaction | update_transaction.go | TransactionEventPublisher.PublishUpdated |
| DeleteTransaction | delete_transaction.go | TransactionEventPublisher.PublishDeleted |
| GetTransaction | get_transaction.go | — |
| ListTransactions | list_transactions.go | — |
| CreateCardPurchase | create_card_purchase.go | CardPurchaseEventPublisher.PublishCreated |
| UpdateCardPurchase | update_card_purchase.go | CardPurchaseEventPublisher.PublishUpdated |
| DeleteCardPurchase | delete_card_purchase.go | CardPurchaseEventPublisher.PublishDeleted |
| GetCardPurchase | get_card_purchase.go | — |
| ListCardPurchases | list_card_purchases.go | — |
| GetCardInvoice | get_card_invoice.go | — |
| CreateRecurringTemplate | create_recurring_template.go | RecurringTemplateEventPublisher.PublishCreated |
| UpdateRecurringTemplate | update_recurring_template.go | RecurringTemplateEventPublisher.PublishUpdated |
| DeleteRecurringTemplate | delete_recurring_template.go | RecurringTemplateEventPublisher.PublishDeleted |
| GetRecurringTemplate | get_recurring_template.go | — |
| ListRecurringTemplates | list_recurring_templates.go | — |
| GetMonthlySummary | get_monthly_summary.go | — |
| ListMonthlyEntries | list_monthly_entries.go | — |
| RecomputeMonthlySummary | recompute_monthly_summary.go | — |
| ReconcileMonthlySummary | reconcile_monthly_summary.go | — |
| MaterializeRecurringForDay | materialize_recurring_for_day.go | via CreateTransaction/CreateCardPurchase |

### 1.3 Eventos de Domínio + Producers

| Evento | `event_type` (outbox) | Aggregate type | Producer |
|--------|-----------------------|---------------|---------|
| TransactionCreated | `transactions.transaction.created.v1` | `transactions.transaction` | transaction_event_publisher.go |
| TransactionUpdated | `transactions.transaction.updated.v1` | `transactions.transaction` | transaction_event_publisher.go |
| TransactionDeleted | `transactions.transaction.deleted.v1` | `transactions.transaction` | transaction_event_publisher.go |
| CardPurchaseCreated | `transactions.card_purchase.created.v1` | (card_purchase_event_publisher) | card_purchase_event_publisher.go |
| CardPurchaseUpdated | `transactions.card_purchase.updated.v1` | (card_purchase_event_publisher) | card_purchase_event_publisher.go |
| CardPurchaseDeleted | `transactions.card_purchase.deleted.v1` | (card_purchase_event_publisher) | card_purchase_event_publisher.go |
| RecurringTemplateCreated | (recurring_template publisher) | — | recurring_template_event_publisher.go |
| RecurringTemplateUpdated | (recurring_template publisher) | — | recurring_template_event_publisher.go |
| RecurringTemplateDeleted | (recurring_template publisher) | — | recurring_template_event_publisher.go |

### 1.4 Consumer

| Consumer | Eventos consumidos | Idempotência | Mecanismo especial |
|----------|---------------------|-------------|-------------------|
| MonthlySummaryRecomputeConsumer | `transactions.transaction.{created,updated,deleted}.v1`, `transactions.card_purchase.{created,updated,deleted}.v1` | por `event_id` | Coalescer (debounce por `user_id+ref_month`) |

### 1.5 Jobs

| Job | Nome (`Name()`) | Schedule padrão | Timeout | Use Case |
|-----|----------------|-----------------|---------|---------|
| RecurringMaterializerJob | `transactions-recurring-materializer` | `@daily` (ou cfg) | 5 min | MaterializeRecurringForDay |
| MonthlySummaryReconcilerJob | `transactions-monthly-summary-reconciler` | `@daily` (ou cfg) | 5 min | ReconcileMonthlySummary |

### 1.6 Repositórios e Tabelas

| Repositório | Tabela principal | Operações |
|------------|-----------------|-----------|
| TransactionRepository | `mecontrola.transactions` | Create, GetByID, Update (optimistic lock), SoftDelete, List paginado |
| CardPurchaseRepository | `mecontrola.card_purchases` | Create, GetByID, Update, SoftDelete, List |
| CardInvoiceRepository | `mecontrola.card_invoices` + `mecontrola.card_invoice_items` | Get, Upsert |
| RecurringTemplateRepository | `mecontrola.recurring_templates` | Create, GetByID, Update, SoftDelete, List |
| RecurringMaterializationRepository | `mecontrola.recurring_materializations` | InsertIfAbsent (idempotência do job) |
| MonthlySummaryRepository | `mecontrola.monthly_summaries` | Upsert, GetByUserIDAndRefMonth |
| CategoriesReaderAdapter | (lê de `categories` cross-module) | ResolveBySlug, ValidateSubcategory |

### 1.7 Mocks gerados (`.mockery.yml`)

Mocks de interface: `CardInvoiceRepository`, `CardLookup`, `CardPurchaseEventPublisher`, `CardPurchaseRepository`, `CategoryValidator`, `MonthlySummaryRepository`, `RecurringMaterializationRepository`, `RecurringTemplateEventPublisher`, `RecurringTemplateRepository`, `RepositoryFactory`, `TransactionEventPublisher`, `TransactionRepository`
Mocks de UoW (use cases/mocks/): unit_of_work_* por fluxo

---

## 2. Análise de Gaps — O Que Está Faltando

### 2.1 Camada Domínio — Commands (Unit puro)

| Arquivo | Status |
|---------|--------|
| `domain/commands/create_card_purchase_test.go` | ✅ existe |
| `domain/commands/create_transaction_test.go` | ✅ existe |
| `domain/commands/create_recurring_template_test.go` | ❌ **AUSENTE** |
| `domain/commands/update_transaction_test.go` | ❌ **AUSENTE** |
| `domain/commands/update_card_purchase_test.go` | ❌ **AUSENTE** |
| `domain/commands/update_recurring_template_test.go` | ❌ **AUSENTE** |

### 2.2 Camada HTTP Handlers (Unit com mock UC)

O arquivo `handlers_test.go` cobre apenas: CreateTransaction (3 cenários), UpdateTransaction (1), DeleteTransaction (1), GetTransaction (2), ListTransactions (1), GetMonthlySummary (1). **Faltam:**

| Handler | Cenários faltantes |
|---------|-------------------|
| CreateCardPurchase | Success 201, InvalidPayload 400, NotFound (card) 404, Unauthorized 401 |
| UpdateCardPurchase | Success 200, NotFound 404, Unauthorized 401 |
| DeleteCardPurchase | Success 204, NotFound 404, Unauthorized 401 |
| GetCardPurchase | Success 200, NotFound 404, Unauthorized 401 |
| ListCardPurchases | Success 200 |
| GetCardInvoice | Success 200, NotFound 404, Unauthorized 401 |
| CreateRecurringTemplate | Success 201, InvalidPayload 400, Unauthorized 401 |
| UpdateRecurringTemplate | Success 200, NotFound 404, Unauthorized 401 |
| DeleteRecurringTemplate | Success 204, NotFound 404, Unauthorized 401 |
| GetRecurringTemplate | Success 200, NotFound 404, Unauthorized 401 |
| ListRecurringTemplates | Success 200 |
| ListMonthlyEntries | Success 200 |
| UpdateTransaction | NotFound 404, Unauthorized 401 |
| DeleteTransaction | NotFound 404, Unauthorized 401 |

### 2.3 Camada Integração (já existente — confirmar cobertura)

| Arquivo | Status |
|---------|--------|
| `transaction_repository_integration_test.go` | ✅ existe (verificar optimistic lock + soft-delete) |
| `card_purchase_repository_integration_test.go` | ✅ existe |
| `card_invoice_repository_integration_test.go` | ✅ existe |
| `recurring_template_repository_integration_test.go` | ✅ existe |
| `recurring_materialization_repository_integration_test.go` | ✅ existe |
| `monthly_summary_repository_integration_test.go` | ✅ existe |
| `transaction_event_publisher_integration_test.go` | ✅ existe |
| `card_purchase_event_publisher_integration_test.go` | ✅ existe |
| `recurring_template_event_publisher_integration_test.go` | ✅ existe |
| `monthly_summary_recompute_consumer_integration_test.go` | ✅ existe |
| `recurring_materializer_job_integration_test.go` | ✅ existe |
| `monthly_summary_reconciler_job_integration_test.go` | ✅ existe |

> Os integration tests existentes devem ser auditados durante execução para verificar: optimistic lock + conflito, soft-delete + listagem, idempotência de consumer por `event_id`, idempotência de job por `InsertIfAbsent`.

### 2.4 Camada E2E godog (gap principal)

Cenários existentes: apenas `f01_transaction_flow.feature` (1 cenário: POST /transactions).

**Faltam os seguintes feature files:**

| Feature file | Escopo |
|-------------|--------|
| `f01_transactions_crud.feature` | CRUD completo de transactions + outbox |
| `f02_card_purchases_crud.feature` | CRUD completo de card-purchases + outbox |
| `f03_recurring_templates_crud.feature` | CRUD completo de recurring-templates + outbox |
| `f04_monthly_summary.feature` | Get summary + list entries |
| `f05_card_invoice.feature` | Get card invoice (depende de card-purchases) |
| `f06_consumer_recompute.feature` | Evento → consumer → monthly_summary atualizado |
| `f07_jobs.feature` | RecurringMaterializer + MonthlySummaryReconciler |

---

## 3. Estrutura de Pastas Proposta

```
internal/transactions/e2e/
  features/
    f01_transactions_crud.feature
    f02_card_purchases_crud.feature
    f03_recurring_templates_crud.feature
    f04_monthly_summary.feature
    f05_card_invoice.feature
    f06_consumer_recompute.feature
    f07_jobs.feature
  suite_test.go
  ctx_test.go
  helpers_test.go
  steps_transactions_test.go
  steps_card_purchases_test.go
  steps_recurring_templates_test.go
  steps_monthly_test.go
  steps_card_invoice_test.go
  steps_consumer_test.go
  steps_jobs_test.go
  steps_shared_test.go
```

**Rationale:** isolado em `internal/transactions/e2e/` para autonomia do módulo; reutiliza `transactions.NewTransactionsModule` sem depender do wiring global de `internal/e2e/`.

---

## 4. Arquivos Gherkin Propostos (PT-BR)

### f01_transactions_crud.feature

```gherkin
# language: pt
Funcionalidade: CRUD de transactions via HTTP

  Cenário: criar transação de despesa persiste no banco e enfileira evento no outbox
    Dado que o ambiente E2E de transactions está pronto
    E que não existe nenhuma transação para o usuário autenticado em "2026-06"
    Quando o usuário cria uma transação de 5800 centavos com método "pix" e direção "outcome" em "2026-06-15"
    Então a resposta HTTP deve ter status 201
    E o corpo da resposta deve conter o campo "id"
    E o banco deve conter exatamente 1 transação nova para o usuário
    E a tabela outbox_events deve conter 1 evento com event_type "transactions.transaction.created.v1"

  Cenário: criar transação com payload inválido retorna 400
    Dado que o ambiente E2E de transactions está pronto
    Quando o usuário envia uma requisição POST para "/api/v1/transactions" com payload inválido
    Então a resposta HTTP deve ter status 400

  Cenário: criar transação sem autenticação retorna 401
    Dado que o ambiente E2E de transactions está pronto
    Quando uma requisição não autenticada envia POST para "/api/v1/transactions"
    Então a resposta HTTP deve ter status 401

  Cenário: obter transação existente retorna 200 com dados corretos
    Dado que o ambiente E2E de transactions está pronto
    E que existe uma transação criada de 3000 centavos com método "credit_card" em "2026-06-10"
    Quando o usuário obtém a transação pelo ID
    Então a resposta HTTP deve ter status 200
    E o corpo da resposta deve conter o campo "id"
    E o corpo da resposta deve conter o campo "amount_cents" com valor 3000

  Cenário: obter transação inexistente retorna 404
    Dado que o ambiente E2E de transactions está pronto
    Quando o usuário tenta obter uma transação com ID inexistente
    Então a resposta HTTP deve ter status 404

  Cenário: listar transações retorna somente as do usuário autenticado
    Dado que o ambiente E2E de transactions está pronto
    E que existem 2 transações criadas para o usuário em "2026-06"
    Quando o usuário lista transações do mês "2026-06"
    Então a resposta HTTP deve ter status 200
    E o corpo da resposta deve conter exatamente 2 transações

  Cenário: atualizar transação persiste mudança no banco e enfileira evento de atualização
    Dado que o ambiente E2E de transactions está pronto
    E que existe uma transação criada de 5000 centavos com método "pix" em "2026-06-10"
    Quando o usuário atualiza a transação para 7000 centavos
    Então a resposta HTTP deve ter status 200
    E o banco deve conter a transação com valor 7000 centavos
    E a tabela outbox_events deve conter 1 evento com event_type "transactions.transaction.updated.v1"

  Cenário: atualizar transação inexistente retorna 404
    Dado que o ambiente E2E de transactions está pronto
    Quando o usuário tenta atualizar uma transação com ID inexistente
    Então a resposta HTTP deve ter status 404

  Cenário: deletar transação faz soft-delete no banco e enfileira evento de deleção
    Dado que o ambiente E2E de transactions está pronto
    E que existe uma transação criada de 2000 centavos com método "pix" em "2026-06-01"
    Quando o usuário deleta a transação
    Então a resposta HTTP deve ter status 204
    E a transação deve ter deleted_at preenchido no banco
    E a transação não deve aparecer na listagem do mês "2026-06"
    E a tabela outbox_events deve conter 1 evento com event_type "transactions.transaction.deleted.v1"

  Cenário: deletar transação inexistente retorna 404
    Dado que o ambiente E2E de transactions está pronto
    Quando o usuário tenta deletar uma transação com ID inexistente
    Então a resposta HTTP deve ter status 404
```

### f02_card_purchases_crud.feature

```gherkin
# language: pt
Funcionalidade: CRUD de card-purchases via HTTP

  Cenário: criar card-purchase em 3 parcelas persiste itens e enfileira evento no outbox
    Dado que o ambiente E2E de transactions está pronto
    E que existe um cartão configurado para o usuário
    Quando o usuário cria uma compra de 9000 centavos em 3 parcelas no cartão
    Então a resposta HTTP deve ter status 201
    E o banco deve conter 3 parcelas (card_invoice_items) para o aggregate_id
    E a tabela outbox_events deve conter 1 evento com event_type "transactions.card_purchase.created.v1"

  Cenário: criar card-purchase com payload inválido retorna 400
    Dado que o ambiente E2E de transactions está pronto
    E que existe um cartão configurado para o usuário
    Quando o usuário envia uma requisição POST para "/api/v1/card-purchases" com payload inválido
    Então a resposta HTTP deve ter status 400

  Cenário: obter card-purchase existente retorna dados corretos
    Dado que o ambiente E2E de transactions está pronto
    E que existe uma card-purchase criada de 6000 centavos em 2 parcelas no cartão
    Quando o usuário obtém a card-purchase pelo ID
    Então a resposta HTTP deve ter status 200
    E o corpo da resposta deve conter o campo "id"

  Cenário: obter card-purchase inexistente retorna 404
    Dado que o ambiente E2E de transactions está pronto
    Quando o usuário tenta obter uma card-purchase com ID inexistente
    Então a resposta HTTP deve ter status 404

  Cenário: listar card-purchases retorna lista do usuário
    Dado que o ambiente E2E de transactions está pronto
    E que existem 2 card-purchases criadas para o usuário
    Quando o usuário lista card-purchases
    Então a resposta HTTP deve ter status 200
    E o corpo da resposta deve conter exatamente 2 card-purchases

  Cenário: atualizar card-purchase persiste mudança e enfileira evento de atualização
    Dado que o ambiente E2E de transactions está pronto
    E que existe uma card-purchase criada de 6000 centavos em 2 parcelas no cartão
    Quando o usuário atualiza a descrição da card-purchase
    Então a resposta HTTP deve ter status 200
    E a tabela outbox_events deve conter 1 evento com event_type "transactions.card_purchase.updated.v1"

  Cenário: deletar card-purchase faz soft-delete e enfileira evento de deleção
    Dado que o ambiente E2E de transactions está pronto
    E que existe uma card-purchase criada de 3000 centavos em 1 parcela no cartão
    Quando o usuário deleta a card-purchase
    Então a resposta HTTP deve ter status 204
    E a card-purchase deve ter deleted_at preenchido no banco
    E a tabela outbox_events deve conter 1 evento com event_type "transactions.card_purchase.deleted.v1"
```

### f03_recurring_templates_crud.feature

```gherkin
# language: pt
Funcionalidade: CRUD de recurring-templates via HTTP

  Cenário: criar recurring-template persiste no banco e enfileira evento no outbox
    Dado que o ambiente E2E de transactions está pronto
    Quando o usuário cria um recurring-template de 1500 centavos com frequência "monthly" no dia 5
    Então a resposta HTTP deve ter status 201
    E o banco deve conter 1 recurring-template novo para o usuário
    E a tabela outbox_events deve conter 1 evento com event_type de recurring_template created

  Cenário: criar recurring-template com payload inválido retorna 400
    Dado que o ambiente E2E de transactions está pronto
    Quando o usuário envia uma requisição POST para "/api/v1/recurring-templates" com payload inválido
    Então a resposta HTTP deve ter status 400

  Cenário: obter recurring-template existente retorna 200
    Dado que o ambiente E2E de transactions está pronto
    E que existe um recurring-template criado de 2000 centavos
    Quando o usuário obtém o recurring-template pelo ID
    Então a resposta HTTP deve ter status 200
    E o corpo da resposta deve conter o campo "id"

  Cenário: obter recurring-template inexistente retorna 404
    Dado que o ambiente E2E de transactions está pronto
    Quando o usuário tenta obter um recurring-template com ID inexistente
    Então a resposta HTTP deve ter status 404

  Cenário: listar recurring-templates retorna lista do usuário
    Dado que o ambiente E2E de transactions está pronto
    E que existem 3 recurring-templates criados para o usuário
    Quando o usuário lista recurring-templates
    Então a resposta HTTP deve ter status 200
    E o corpo da resposta deve conter exatamente 3 recurring-templates

  Cenário: atualizar recurring-template persiste mudança e enfileira evento de atualização
    Dado que o ambiente E2E de transactions está pronto
    E que existe um recurring-template criado de 1000 centavos
    Quando o usuário atualiza o recurring-template para 1200 centavos
    Então a resposta HTTP deve ter status 200
    E a tabela outbox_events deve conter 1 evento com event_type de recurring_template updated

  Cenário: deletar recurring-template faz soft-delete e enfileira evento de deleção
    Dado que o ambiente E2E de transactions está pronto
    E que existe um recurring-template criado de 800 centavos
    Quando o usuário deleta o recurring-template
    Então a resposta HTTP deve ter status 204
    E o recurring-template deve ter deleted_at preenchido no banco
    E a tabela outbox_events deve conter 1 evento com event_type de recurring_template deleted
```

### f04_monthly_summary.feature

```gherkin
# language: pt
Funcionalidade: Resumo mensal e entradas mensais

  Cenário: obter resumo mensal após criação de transação reflete os valores
    Dado que o ambiente E2E de transactions está pronto
    E que existem transações de outcome 5000 centavos e income 10000 centavos em "2026-06"
    Quando o usuário obtém o resumo do mês "2026-06"
    Então a resposta HTTP deve ter status 200
    E o corpo da resposta deve conter o campo "ref_month" com valor "2026-06"

  Cenário: obter resumo de mês sem dados retorna 404
    Dado que o ambiente E2E de transactions está pronto
    Quando o usuário obtém o resumo do mês "2020-01"
    Então a resposta HTTP deve ter status 404

  Cenário: listar entradas mensais retorna todas as entradas do mês
    Dado que o ambiente E2E de transactions está pronto
    E que existem 2 transações e 1 card-purchase criados em "2026-06"
    Quando o usuário lista as entradas do mês "2026-06"
    Então a resposta HTTP deve ter status 200
    E o corpo da resposta deve conter itens de entradas
```

### f05_card_invoice.feature

```gherkin
# language: pt
Funcionalidade: Fatura do cartão

  Cenário: obter fatura do cartão após criação de card-purchase retorna dados corretos
    Dado que o ambiente E2E de transactions está pronto
    E que existe um cartão configurado para o usuário com ciclo de fechamento dia 10
    E que existe uma card-purchase de 3000 centavos em 1 parcela no cartão para "2026-06"
    Quando o usuário obtém a fatura do cartão para "2026-06"
    Então a resposta HTTP deve ter status 200
    E o corpo da resposta deve conter o campo "card_id"

  Cenário: obter fatura de mês sem compras retorna 404
    Dado que o ambiente E2E de transactions está pronto
    E que existe um cartão configurado para o usuário
    Quando o usuário obtém a fatura do cartão para "2020-01"
    Então a resposta HTTP deve ter status 404
```

### f06_consumer_recompute.feature

```gherkin
# language: pt
Funcionalidade: Consumer recomputa monthly summary ao receber eventos do outbox

  Cenário: criar transação dispara evento e consumer atualiza o monthly summary
    Dado que o ambiente E2E de transactions está pronto
    E que não existe nenhuma transação para o usuário em "2026-06"
    Quando o usuário cria uma transação de 4000 centavos com método "pix" em "2026-06-20"
    E o consumer processa o evento da outbox
    Então a tabela monthly_summaries deve conter um registro para "2026-06" com amount_cents atualizado

  Cenário: reprocessar o mesmo evento é idempotente para o monthly summary
    Dado que o ambiente E2E de transactions está pronto
    E que o consumer já processou o evento da última transação criada
    Quando o mesmo evento é reprocessado pelo consumer
    Então a tabela monthly_summaries deve continuar com exatamente 1 registro para o mês
    E o valor do monthly summary não deve ser duplicado

  Cenário: deletar transação dispara evento e consumer remove contribuição do monthly summary
    Dado que o ambiente E2E de transactions está pronto
    E que existe uma transação de 5000 centavos criada em "2026-06-10"
    E que o consumer processou o evento de criação
    Quando o usuário deleta a transação
    E o consumer processa o evento de deleção da outbox
    Então a tabela monthly_summaries deve refletir a remoção dos 5000 centavos
```

### f07_jobs.feature

```gherkin
# language: pt
Funcionalidade: Jobs de materialização e reconciliação

  Cenário: job de materialização cria transação para recurring-template ativo no dia
    Dado que o ambiente E2E de transactions está pronto
    E que existe um recurring-template ativo de 2000 centavos com frequência "monthly" para o dia de hoje
    Quando o job recurring-materializer é executado para hoje
    Então o banco deve conter 1 transação gerada para o recurring-template
    E a tabela recurring_materializations deve registrar a execução do dia

  Cenário: job de materialização é idempotente ao ser executado duas vezes no mesmo dia
    Dado que o ambiente E2E de transactions está pronto
    E que existe um recurring-template ativo de 2000 centavos para o dia de hoje
    E que o job recurring-materializer já foi executado hoje
    Quando o job recurring-materializer é executado novamente para hoje
    Então o banco deve continuar com exatamente 1 transação gerada para o recurring-template
    E a tabela recurring_materializations deve conter exatamente 1 registro para o dia de hoje

  Cenário: job de reconciliação monthly-summary corrige entradas desatualizadas
    Dado que o ambiente E2E de transactions está pronto
    E que existem transações de "2026-06" no banco
    Quando o job monthly-summary-reconciler é executado
    Então a tabela monthly_summaries deve estar consistente com as transações do período
```

---

## 5. Definições de Steps Go (Assinaturas — `//go:build e2e`)

### suite_test.go

```go
//go:build e2e
package transactions_e2e_test

// TestE2ETransactions sobe Postgres via testcontainer, monta o módulo transactions completo
// e executa todos os cenários godog em features/.
func TestE2ETransactions(t *testing.T)
func buildTransactionsServer(t *testing.T, cfg *configs.Config, db *sqlx.DB, o11y observability.Observability) *txE2ERuntime
func loadE2EConfig(t *testing.T) *configs.Config
func seedE2EUserAndCard(t *testing.T, db *sqlx.DB)
```

### ctx_test.go

```go
//go:build e2e
package transactions_e2e_test

type txE2ERuntime struct {
    server              *httptest.Server
    db                  *sqlx.DB
    recurringJob        worker.Job
    reconcilerJob       worker.Job
    recomputeConsumer   *consumers.MonthlySummaryRecomputeConsumer
}

type txE2ECtx struct {
    server             *httptest.Server
    db                 *sqlx.DB
    userID             uuid.UUID
    cardID             uuid.UUID
    lastResp           *http.Response
    lastBody           map[string]any
    lastBodyText       string
    capturedTxID       string
    capturedCPID       string
    capturedRTID       string
    recurringJob       worker.Job
    reconcilerJob      worker.Job
    recomputeConsumer  *consumers.MonthlySummaryRecomputeConsumer
}

func (e *txE2ECtx) makeRequest(method, path string, payload any) error
func (e *txE2ECtx) makeRequestUnauthenticated(method, path string, payload any) error
```

### helpers_test.go

```go
//go:build e2e
package transactions_e2e_test

// countOutboxByEventType conta eventos na outbox_events para um event_type específico e aggregate_id
func countOutboxByEventType(ctx context.Context, db *sqlx.DB, eventType, aggregateID string) (int, error)

// latestTransaction busca a transação mais recente do usuário no banco
func latestTransaction(ctx context.Context, db *sqlx.DB, userID uuid.UUID, refMonth string) (transactionRow, error)

// latestCardPurchase busca a card_purchase mais recente do usuário
func latestCardPurchase(ctx context.Context, db *sqlx.DB, userID uuid.UUID) (cardPurchaseRow, error)

// latestRecurringTemplate busca o recurring_template mais recente do usuário
func latestRecurringTemplate(ctx context.Context, db *sqlx.DB, userID uuid.UUID) (recurringTemplateRow, error)

// countMonthlySummary conta registros em monthly_summaries para user+refMonth
func countMonthlySummary(ctx context.Context, db *sqlx.DB, userID uuid.UUID, refMonth string) (int, error)

// countRecurringMaterializations conta materializações do dia
func countRecurringMaterializations(ctx context.Context, db *sqlx.DB, templateID uuid.UUID, day time.Time) (int, error)

// drainConsumerEvents processa todos os eventos pendentes no outbox via consumer
func drainConsumerEvents(ctx context.Context, consumer *consumers.MonthlySummaryRecomputeConsumer) error
```

### steps_transactions_test.go

```go
//go:build e2e
package transactions_e2e_test

func registerTransactionSteps(sc *godog.ScenarioContext, e *txE2ECtx)

func (e *txE2ECtx) ambienteE2EDeTransactionsEstaPronto() error
func (e *txE2ECtx) naoExisteNenhumaTransacaoParaOUsuarioEm(refMonth string) error
func (e *txE2ECtx) oUsuarioCriaUmaTransacao(centavos int, metodo, direcao, data string) error
func (e *txE2ECtx) oUsuarioEnviaRequisicaoPOSTComPayloadInvalido(path string) error
func (e *txE2ECtx) requisicaoNaoAutenticadaEnviaPost(path string) error
func (e *txE2ECtx) existeUmaTransacaoCriadaDe(centavos int, metodo, data string) error
func (e *txE2ECtx) oUsuarioObtemATransacaoPeloID() error
func (e *txE2ECtx) oUsuarioTentaObterUmaTransacaoComIDInexistente() error
func (e *txE2ECtx) existemNTransacoesCriadasParaOUsuarioEm(n int, refMonth string) error
func (e *txE2ECtx) oUsuarioListaTransacoesDoMes(refMonth string) error
func (e *txE2ECtx) oUsuarioAtualizaATransacaoParaCentavos(centavos int) error
func (e *txE2ECtx) oUsuarioTentaAtualizarUmaTransacaoComIDInexistente() error
func (e *txE2ECtx) oUsuarioDeletaATransacao() error
func (e *txE2ECtx) oUsuarioTentaDeletarUmaTransacaoComIDInexistente() error
func (e *txE2ECtx) aTransacaoDeveEstarSalvaComValorCentavos(centavos int) error
func (e *txE2ECtx) aTransacaoDeveTermDeletedAtPreenchidoNoBanco() error
func (e *txE2ECtx) aTransacaoNaoDeveAparecerNaListagemDoMes(refMonth string) error
```

### steps_card_purchases_test.go

```go
//go:build e2e
package transactions_e2e_test

func registerCardPurchaseSteps(sc *godog.ScenarioContext, e *txE2ECtx)

func (e *txE2ECtx) existeUmCartaoConfiguradoParaOUsuario() error
func (e *txE2ECtx) oUsuarioCriaUmaCompraDeN3ParcelasNoCartao(centavos, parcelas int) error
func (e *txE2ECtx) oBancoDeveConterNParcelasParaOAggregateID(n int) error
func (e *txE2ECtx) existeUmaCardPurchaseCriadaDe(centavos, parcelas int) error
func (e *txE2ECtx) oUsuarioObtemACardPurchasePeloID() error
func (e *txE2ECtx) oUsuarioTentaObterUmaCardPurchaseComIDInexistente() error
func (e *txE2ECtx) existemNCardPurchasesCriadasParaOUsuario(n int) error
func (e *txE2ECtx) oUsuarioListaCardPurchases() error
func (e *txE2ECtx) oUsuarioAtualizaADescricaoDaCardPurchase() error
func (e *txE2ECtx) oUsuarioDeletaACardPurchase() error
func (e *txE2ECtx) aCardPurchaseDeveTermDeletedAtPreenchidoNoBanco() error
```

### steps_recurring_templates_test.go

```go
//go:build e2e
package transactions_e2e_test

func registerRecurringTemplateSteps(sc *godog.ScenarioContext, e *txE2ECtx)

func (e *txE2ECtx) oUsuariaCriaUmRecurringTemplate(centavos int, frequencia string, dia int) error
func (e *txE2ECtx) oBancoDeveConter1RecurringTemplateNovoParaOUsuario() error
func (e *txE2ECtx) existeUmRecurringTemplateCriadoDe(centavos int) error
func (e *txE2ECtx) oUsuarioObtemORecurringTemplatePeloID() error
func (e *txE2ECtx) oUsuarioTentaObterUmRecurringTemplateComIDInexistente() error
func (e *txE2ECtx) existemNRecurringTemplatesCriadosParaOUsuario(n int) error
func (e *txE2ECtx) oUsuarioListaRecurringTemplates() error
func (e *txE2ECtx) oUsuarioAtualizaORecurringTemplateParaCentavos(centavos int) error
func (e *txE2ECtx) oUsuarioDeletaORecurringTemplate() error
func (e *txE2ECtx) oRecurringTemplateDeveTermDeletedAtPreenchidoNoBanco() error
```

### steps_monthly_test.go

```go
//go:build e2e
package transactions_e2e_test

func registerMonthlySteps(sc *godog.ScenarioContext, e *txE2ECtx)

func (e *txE2ECtx) existemTransacoesDeOutcomeEIncomeEm(outcomeC, incomeC int, refMonth string) error
func (e *txE2ECtx) oUsuarioObtemOResumoDoMes(refMonth string) error
func (e *txE2ECtx) existemNTransacoesE1CardPurchaseEm(nTx int, refMonth string) error
func (e *txE2ECtx) oUsuarioListaAsEntradasDoMes(refMonth string) error
```

### steps_consumer_test.go

```go
//go:build e2e
package transactions_e2e_test

func registerConsumerSteps(sc *godog.ScenarioContext, e *txE2ECtx)

func (e *txE2ECtx) oConsumerProcessaOEventoDaOutbox() error
func (e *txE2ECtx) oConsumerJaProcessouOEventoDaUltimaTransacaoCriada() error
func (e *txE2ECtx) oMesmoEventoEReprocessadoPeloConsumer() error
func (e *txE2ECtx) aTabelaMonthlySummariesDeveConterUmRegistroPara(refMonth string) error
func (e *txE2ECtx) aTabelaMonthlySummariesDeveConterExatamente1RegistroPara(refMonth string) error
func (e *txE2ECtx) oValorDoMonthlySummaryNaoDeveSerDuplicado(refMonth string) error
```

### steps_jobs_test.go

```go
//go:build e2e
package transactions_e2e_test

func registerJobSteps(sc *godog.ScenarioContext, e *txE2ECtx)

func (e *txE2ECtx) existeUmRecurringTemplateAtivoParaODiaDeHoje(centavos int, frequencia string) error
func (e *txE2ECtx) oJobRecurringMaterializerEExecutadoParaHoje() error
func (e *txE2ECtx) oBancoDeveConter1TransacaoGeradaParaORecurringTemplate() error
func (e *txE2ECtx) aTabelaRecurringMaterializationsDeveRegistrarAExecucaoDoDia() error
func (e *txE2ECtx) oJobRecurringMaterializerJaFoiExecutadoHoje() error
func (e *txE2ECtx) oJobMonthlySummaryReconcilerEExecutado() error
func (e *txE2ECtx) aTabelaMonthlySummariesDeveEstarConsistente() error
```

### steps_shared_test.go

```go
//go:build e2e
package transactions_e2e_test

func registerSharedSteps(sc *godog.ScenarioContext, e *txE2ECtx)

func (e *txE2ECtx) aRespostaHTTPDeveTerStatus(status int) error
func (e *txE2ECtx) oCorpoDeveConterOCampo(campo string) error
func (e *txE2ECtx) oCorpoDeveConterOCampoComValor(campo string, valor any) error
func (e *txE2ECtx) oCorpoDeveConterExatamenteN(n int, entidade string) error
func (e *txE2ECtx) aTabelaOutboxDeveConter1EventoComEventType(eventType string) error
func (e *txE2ECtx) oUsuarioEnviaRequisicaoPOSTParaComPayloadInvalido(path string) error
func (e *txE2ECtx) requisicaoNaoAutenticadaEnviaPostPara(path string) error
func (e *txE2ECtx) oUsuarioTentaObterComIDInexistente(recurso string) error
```

---

## 6. Estratégia de Validação de Banco e Outbox

### 6.1 Helpers de banco (padrão `countOutboxByType` de billing)

Todo teste que escreve no banco deve chamar um helper `QueryRowContext` direto na tabela após a operação:

```
criou?        → COUNT(*) WHERE id = $capturedID AND deleted_at IS NULL == 1
atualizou?    → SELECT campo FROM tabela WHERE id = $id → comparar valor
soft-delete?  → SELECT deleted_at FROM tabela WHERE id = $id → NOT NULL
idempotência? → COUNT(*) antes e depois do reprocesso == idêntico
outbox?       → COUNT(*) FROM mecontrola.outbox_events WHERE event_type = $type AND aggregate_id = $id
```

### 6.2 Validação de outbox

O step `aTabelaOutboxDeveConter1EventoComEventType` executa:

```sql
SELECT COUNT(*) FROM mecontrola.outbox_events
WHERE event_type = $1 AND aggregate_id = $2
```

O `aggregate_id` é capturado do campo `id` retornado no corpo HTTP (capturedTxID / capturedCPID / capturedRTID).

### 6.3 Consumer: drainConsumerEvents

Para cenários de consumer, o step `oConsumerProcessaOEventoDaOutbox` chama `storage.ClaimBatch` e entrega os eventos para o handler do consumer, aguardando o coalescer liberar com timeout. O padrão é:

```
1. storage.ClaimBatch(ctx, "e2e-test-consumer", 100)
2. Para cada evento: consumer.Handle(ctx, event)
3. Aguardar coalescer com sleep(2 × debounceWindow + 200ms)
4. Afirmar estado do banco via SELECT
```

### 6.4 Idempotência do consumer por event_id

O step de reprocesso reusa o mesmo `event_id` do evento original. O helper `countMonthlySummary` compara o `amount_cents` antes e depois para garantir ausência de duplicação.

### 6.5 Jobs: idempotência via recurring_materializations

O step `oJobRecurringMaterializerJaFoiExecutadoHoje` executa o job uma vez, captura o count de `recurring_materializations` e de `transactions` geradas. Na segunda execução, o mesmo count deve se manter (InsertIfAbsent retorna false na colisão de unique constraint).

---

## 7. Gaps de Command Unit Tests — Escopo Mínimo

Os 4 arquivos de test ausentes em `domain/commands/` devem cobrir:

| Arquivo | Cenários mínimos |
|---------|-----------------|
| `create_recurring_template_test.go` | construtor com dados válidos; erros acumulados para campos inválidos (amount_cents ≤ 0, day_of_month inválido, frequency inválida) |
| `update_transaction_test.go` | construtor válido; erros por amount_cents ≤ 0; propagação de Option nil |
| `update_card_purchase_test.go` | construtor válido; erros por amount_cents ≤ 0 |
| `update_recurring_template_test.go` | construtor válido; erros por campos inválidos |

---

## 8. Gaps de Handler Unit Tests — Escopo Mínimo

Adicionar ao `handlers_test.go` (ou criar `handlers_card_purchase_test.go`, `handlers_recurring_test.go`):

| Handler | Cenários mínimos a adicionar |
|---------|------------------------------|
| CreateCardPurchase | Success 201, InvalidPayload 400, Unauthorized 401, NotFound(card) 404 |
| UpdateCardPurchase | Success 200, NotFound 404, Unauthorized 401 |
| DeleteCardPurchase | Success 204, NotFound 404, Unauthorized 401 |
| GetCardPurchase | Success 200, NotFound 404 |
| ListCardPurchases | Success 200 |
| GetCardInvoice | Success 200, NotFound 404 |
| CreateRecurringTemplate | Success 201, InvalidPayload 400, Unauthorized 401 |
| UpdateRecurringTemplate | Success 200, NotFound 404, Unauthorized 401 |
| DeleteRecurringTemplate | Success 204, NotFound 404, Unauthorized 401 |
| GetRecurringTemplate | Success 200, NotFound 404 |
| ListRecurringTemplates | Success 200 |
| ListMonthlyEntries | Success 200 |
| UpdateTransaction (adicionar) | NotFound 404, Unauthorized 401 |
| DeleteTransaction (adicionar) | NotFound 404, Unauthorized 401 |

---

## 9. Definition of Done

Antes de declarar o módulo 100% coberto, todos os gates abaixo devem passar:

```bash
task test:unit      # -race, deve ser verde
task test:integration  # requer Docker, deve ser verde
task test:e2e          # build tag e2e, deve ser verde
```

```bash
# Zero comentários em .go de produção (R-ADAPTER-001.1)
grep -rn --include="*.go" --exclude-dir=mocks --exclude="*.pb.go" --exclude="*_test.go" \
  "^[[:space:]]*//" internal/transactions/ configs/ cmd/ \
  | grep -Ev "(//go:|//nolint:|// Code generated)" \
  && echo "FAIL" && exit 1 || true

# Sem SQL direto em adapters (R-ADAPTER-001.2)
grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" \
  "QueryContext\|ExecContext\|db\.Query\|tx\.Exec\|db\.Exec" \
  internal/transactions/infrastructure/http/server/handlers/ \
  && echo "FAIL" && exit 1 || true

# Regra de domínio fora de Decide* bloqueia (R-TXN-001)
grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" \
  "amount_cents\|direction\|installments\|payment_method\|day_of_month" \
  internal/transactions/application/usecases/ \
  internal/transactions/infrastructure/http/server/handlers/ \
  | grep -v "Decide\|command\|input\." \
  && echo "FAIL" && exit 1 || true
```

```bash
golangci-lint run ./internal/transactions/...
go vet ./internal/transactions/...
```

---

## 10. Plano de Subagents (Execução Paralela Obrigatória)

Ao iniciar a implementação, spawnar 1 subagent por camada em paralelo:

| Subagent | Escopo | Arquivo(s) alvo |
|----------|--------|----------------|
| A — Domain Commands | Criar 4 test files ausentes | `domain/commands/*_test.go` |
| B — Handler Unit Tests | Cobrir 14 handlers faltantes | `infrastructure/http/server/handlers/handlers_test.go` (+ novos arquivos) |
| C — Integração Audit | Auditar integration tests existentes para optimistic lock, soft-delete, idempotência | `infrastructure/repositories/postgres/*_integration_test.go`, `infrastructure/messaging/database/consumers/*_integration_test.go`, `infrastructure/jobs/handlers/*_integration_test.go` |
| D — E2E Transactions | Criar f01 feature + steps | `internal/transactions/e2e/features/f01_transactions_crud.feature`, `steps_transactions_test.go` |
| E — E2E CardPurchases | Criar f02 feature + steps | `internal/transactions/e2e/features/f02_card_purchases_crud.feature`, `steps_card_purchases_test.go` |
| F — E2E RecurringTemplates | Criar f03 feature + steps | `internal/transactions/e2e/features/f03_recurring_templates_crud.feature`, `steps_recurring_templates_test.go` |
| G — E2E Monthly + CardInvoice | Criar f04 + f05 features + steps | `steps_monthly_test.go`, `steps_card_invoice_test.go` |
| H — E2E Consumer + Jobs | Criar f06 + f07 features + steps | `steps_consumer_test.go`, `steps_jobs_test.go` |

Subagents D–H devem compartilhar `suite_test.go`, `ctx_test.go` e `helpers_test.go` criados por um passo de setup antes do fan-out.

**Síntese final:** consolidar evidências de cada subagent (testes criados, gates executados, estado do banco verificado) antes de declarar módulo fechado.
