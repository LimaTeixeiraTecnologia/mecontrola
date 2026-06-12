<!-- spec-hash-prd: 906baec8051ad3dfa9cd13ce11248fd2621761bebcfaf769937640dc14810170 -->
<!--
Histórico de versões:
- v1 (2026-06-12): versão inicial.
- v1.1 (2026-06-12): aplica 4 decisões de codebase — (1) `internal/card` ganha use case `GetCardForUser` consumido pelo port `CardLookup`; (2) `CategoriesCache` local em `internal/transactions/infrastructure/config/` espelhando o padrão do `budgets`; (3) tabelas no schema `mecontrola` prefixadas com `transactions_*` exceto a tabela principal que permanece `transactions`; (4) `RecurringMaterializerJob`, `MonthlySummaryReconcilerJob` e `MonthlySummaryRecomputeConsumer` registrados apenas em `cmd/worker`, `cmd/api` registra somente router HTTP.
- v1.2 (2026-06-12): fixa contrato inegociável de repositórios — `db database.DBTX` é campo da struct injetado no construtor (`NewXxxRepository(o11y, db)`), espelhando `alert_repository.go:20-27`; métodos de repositório nunca recebem `db` como parâmetro. Producers seguem regra distinta (continuam recebendo `db` em `Publish(ctx, db, env)` para participar da TX).
- v1.3 (2026-06-12): padroniza prefixo de rotas para `/api/v1/...` em toda a tabela de endpoints e exemplos de fluxo (substitui `/v1/...`). PRD atualizado em paralelo (v2.1); spec-hash recalculado para `32776ec5…d4353e81`.
- v1.4 (2026-06-12): propaga ADR-006 (DMMF seletivo) por toda a techspec — seção "Visão Geral dos Componentes" lista `domain/option/`, `domain/entities/events.go` e workflows `Decide*`; seção "Interfaces Chave" ganha "Padrão de Use Case com `Decide*` puro" com esqueleto canônico; "Sequenciamento de Desenvolvimento" expande de 19 para 24 passos cobrindo VOs, events, workflows e nova regra `.claude/rules/transactions-workflows.md`; "Conformidade com Padrões" registra ADR-006 + precedência `domain-modeling.md` em `.claude/rules/governance.md`; "Arquivos Relevantes" detalha pacotes novos. PRD inalterado (DMMF é decisão de implementação).
- v1.5 (2026-06-12): fecha 6 gates de auditoria interna — (1) move tipos de comando para `domain/commands/` exportado (resolve import cycle latente `application → domain/services`), espelhando `internal/budgets/domain/commands/`; (2) crava fórmula `delta = sum(new_items[ref_month]) − sum(old_items[ref_month])` para ajuste de `transactions_card_invoices.items_total_cents` em PATCH; (3) `BillingCycleResolver` aplica clamp `min(day, last_day_of_target_month)` em `closing_at`/`due_at` para meses sem o dia natural; (4) padroniza nome `CardBillingSnapshot` em todas as interfaces e ADRs; (5) substitui "9 interfaces" por inventário nominal de 12 mocks no `mockery.yml`; (6) "triple-layer" → "double-layer" no contrato de idempotência do `RecurringMaterializerJob`. ADRs 002, 003, 005 e 006 atualizados em paralelo.
-->
<!-- MANDATÓRIO: preenchido por `create-technical-specification` Etapa 7.1 com sha256 do PRD consumido.
     Rastreabilidade: `create-tasks` e `execute-task` comparam este hash com o atual do prd.md
     para detectar drift entre techspec e PRD. NÃO remover este comentário ao editar a techspec. -->

# Especificação Técnica — `internal/transactions`

> PRD: `.specs/prd-transactions-monthly/prd.md` (v2, 2026-06-12).
> Skills carregadas: `agent-governance`, `go-implementation` (refs: `architecture.md`, `api.md`, `persistence.md`, `messaging.md`, `observability.md`, `testing.md` — sob demanda por seção).
> Regras transversais aplicadas: R-GOV-001, R-ADAPTER-001, R0–R7 da skill `go-implementation`.

## Resumo Executivo

`internal/transactions` é um bounded context novo em `internal/<modulo>/` seguindo o **Padrão Obrigatório de Módulo** (DI manual em `module.go`, estilo `BudgetsModule`). Implementa CRUD de lançamentos avulsos (`Transaction`), compras parceladas de cartão (`CardPurchase` + `CardInvoice` + `CardInvoiceItem`), templates recorrentes (`RecurringTemplate`), projeção mensal (`MonthlySummary`) e materialização diária de recorrências por `day_of_month`.

A entrega é production-ready inegociável: persistência transacional Postgres com `version BIGINT` (optimistic locking) em todo agregado mutável, idempotência por header (`scope="transactions"`, TTL 24h), publicação de eventos via `outbox.Publisher` na mesma transação SQL do agregado, consumer reativo de `MonthlySummary` com **debounce/coalescing por `(user_id, ref_month)`** em janela de 1500 ms, job diário `RecurringMaterializerJob` com **double-layer idempotency** — `pg_try_advisory_xact_lock` como first-cut (gate da tentativa) + PK `(template_id, ref_month)` como race-final (`INSERT ... ON CONFLICT DO NOTHING`), job de reconciliação diária comparando projeção vs `SUM(transactions)+SUM(card_invoice_items)`, e feature flag `TransactionsConfig.Enabled` para rollback em < 5 min ("stop new writes" + drain do outbox). Snapshot estático de `BillingCycle` no `CardPurchase` (sem consumir eventos do `internal/card`) e edição retroativa silenciosa em faturas fechadas com `ref_months_affected` no evento são trade-offs aceitos (ADR-001 e ADR-005).

## Arquitetura do Sistema

### Visão Geral dos Componentes

**Novos** (todos sob `internal/transactions/`):

- **domain/** _(aderente ao ADR-006 — Domain Modeling Made Functional seletivo)_
  - `entities/transaction.go` — agregado `Transaction` (lançamento avulso) com `Version`.
  - `entities/card_purchase.go` — agregado `CardPurchase` (compra-pai) com snapshot `CardClosingDay`/`CardDueDay`/`PurchasedAt` e `Version`.
  - `entities/card_invoice.go` — agregado `CardInvoice` (fatura simulada por `card_id`/`ref_month`) com `Version` e `ItemsTotalCents` denormalizado.
  - `entities/card_invoice_item.go` — entidade `CardInvoiceItem` (parcela) sob raiz `CardInvoice`/`CardPurchase`.
  - `entities/recurring_template.go` — agregado `RecurringTemplate` com `Version`.
  - `entities/monthly_summary.go` — projeção `MonthlySummary` (não é agregado de escrita do usuário).
  - **`entities/events.go`** — domain events tipados (ADR-006 §3): `TransactionCreated`, `TransactionUpdated`, `TransactionDeleted`, `CardPurchaseCreated`, `CardPurchaseUpdated`, `CardPurchaseDeleted`, `RecurringTemplateCreated`, `RecurringTemplateUpdated`, `RecurringTemplateDeleted`. Producers só mapeiam estes tipos para `outbox.Envelope`; cálculo de `ref_months_affected`, `event_id`, `aggregate_*`, `occurred_at` vive no `Decide*`.
  - `valueobjects/` — VOs com **smart constructors obrigatórios** (ADR-006 §1) e tipos opacos: `RefMonth` (`YYYY-MM` em `America/Sao_Paulo`), `Direction` (`income`/`outcome`), `PaymentMethod` (enum com `pix`, `ted`, `debit_in_account`, `debit_card`, `cash`, `boleto`, `credit_card`, `doc` legado read-only), `Frequency` (`monthly`/`yearly`), `Money` (`int64` em centavos, `> 0`), `InstallmentCount` (1..24), `DayOfMonth` (1..28), `Description` (não-vazia, ≤ N chars). Validação fora do construtor é proibida.
  - **`option/option.go`** — pacote utilitário com `Option[T any]` (`Some`, `None`, `Get`, `IsPresent`) para opcionais semanticamente significativos (`SubcategoryID`, `EndedAt`, `CardID` em template de débito). ≤ 30 linhas (ADR-006 §4).
  - **`commands/`** _(novo pacote, ADR-006 + audit fix #1)_ — tipos exportados de comando construídos por smart constructors que rodam fora da camada `application` para evitar import cycle. Arquivos: `create_transaction.go`, `update_transaction.go`, `create_card_purchase.go`, `update_card_purchase.go`, `create_recurring_template.go`, `update_recurring_template.go`, `materialize_recurring.go`. Cada um define um struct exportado (`CreateCardPurchase`, etc.) e um construtor `NewCreateCardPurchase(raw RawCreateCardPurchase, principal auth.Principal) (CreateCardPurchase, error)` que acumula erros com `errors.Join`. Espelha o padrão `internal/budgets/domain/commands/`. Workflows em `domain/services/` consomem esses tipos diretamente; `application/usecases/` apenas converte raw HTTP em `RawCreate*` e delega ao construtor do comando.
  - **`CardBillingSnapshot`** _(audit fix #2)_ — tipo único em `domain/valueobjects/card_billing_snapshot.go` com `ClosingDay DayOfMonth`, `DueDay DayOfMonth`. Substitui qualquer ocorrência anterior de `CardBillingSnapshot`. Construtor `NewCardBillingSnapshot(closing, due int) (CardBillingSnapshot, error)` valida 1..31 ao receber inteiros vindos do `internal/card`.
  - `services/` — funções puras + workflows `Decide*` (ADR-006 §2):
    - `installment_splitter.go` — `InstallmentSplitter.Split(Money, InstallmentCount) []Money` (divisão determinística de centavos; pura).
    - `billing_cycle_resolver.go` — `BillingCycleResolver.Resolve(purchasedAt, snapshot CardBillingSnapshot, n InstallmentCount) (refMonths []RefMonth, closings []time.Time, dues []time.Time)` (pura). **Audit fix #3 — clamp obrigatório**: ao construir `closing_at` ou `due_at` para um `target_month` onde o dia natural não existe (ex: `due_day=30` em fevereiro), aplicar `effective_day = min(day, last_day_of_month(target_month))`. Política alinhada à realidade brasileira (fatura cai no último dia do mês quando o dia natural inexiste); evita `time.Date(...)` overflow para mês seguinte.
    - `ref_month_resolver.go` — `RefMonthResolver.From(t, loc) RefMonth` (pura).
    - **`transaction_workflow.go`** — `DecideCreate(cmd, ids, now) TransactionDecision`, `DecideUpdate(current, cmd, now) TransactionDecision` (puro; ref_months_affected = `{old.RefMonth, new.RefMonth}` deduped quando muda).
    - **`card_purchase_workflow.go`** — `DecideCreate(cmd, snapshot, ids, now) CardPurchaseDecision`, `DecideUpdate(current, currentItems, cmd, now) CardPurchaseDecision` (puro; `ref_months_affected = old ∪ new`).
    - **`recurring_workflow.go`** — `DecideMaterializeForDay(template, today, ids) MaterializeDecision` (decide se materializa `Transaction` ou `CardPurchase` e com qual `occurred_at`).

- **application/**
  - `dtos/input/` — `CreateTransactionInput`, `UpdateTransactionInput`, `DeleteTransactionInput`, `CreateCardPurchaseInput`, `UpdateCardPurchaseInput`, `DeleteCardPurchaseInput`, `CreateRecurringTemplateInput`, `UpdateRecurringTemplateInput`, `MaterializeRecurringInput`, `RecomputeMonthlySummaryInput`, `ReconcileMonthlySummaryInput`.
  - `dtos/output/` — `TransactionOutput`, `CardPurchaseOutput` (inclui `RefMonthsAffected []string` em response de update/delete), `CardInvoiceOutput`, `RecurringTemplateOutput`, `MonthlySummaryOutput`, `MonthlyEntriesPageOutput`.
  - `interfaces/` — `RepositoryFactory`, `TransactionRepository`, `CardPurchaseRepository`, `CardInvoiceRepository`, `RecurringTemplateRepository`, `MonthlySummaryRepository`, `RecurringMaterializationRepository`, `CardLookup` (porta consumidora para `internal/card`), `CategoryValidator` (porta consumidora para `internal/categories`).
  - `usecases/` — `CreateTransaction`, `UpdateTransaction`, `DeleteTransaction`, `GetTransaction`, `ListTransactions`, `CreateCardPurchase`, `UpdateCardPurchase`, `DeleteCardPurchase`, `GetCardPurchase`, `ListCardPurchases`, `GetCardInvoice`, `CreateRecurringTemplate`, `UpdateRecurringTemplate`, `DeleteRecurringTemplate`, `GetRecurringTemplate`, `ListRecurringTemplates`, `MaterializeRecurringForDay`, `RecomputeMonthlySummary`, `ReconcileMonthlySummary`, `GetMonthlySummary`, `ListMonthlyEntries`.

- **infrastructure/**
  - `config/` — `Config` (lê `TransactionsConfig` do `configs.Config`).
  - `http/server/handlers/` — 1 handler fino por endpoint (RF-01, 05–08, 11, 17–21, 23–24, 29–30); apenas decode → usecase → encode (R-ADAPTER-001.2).
  - `http/server/transactions_router.go` — `Register(chi.Router)`.
  - `repositories/postgres/` — `transaction_repository.go`, `card_purchase_repository.go`, `card_invoice_repository.go`, `recurring_template_repository.go`, `monthly_summary_repository.go`, `recurring_materialization_repository.go`, `categories_reader_adapter.go` (porta `CategoriesReader` consumida pelo `CategoriesCache` local; espelha `internal/budgets/infrastructure/repositories/postgres/categories_reader_adapter.go`).
  - `repositories/repository_factory.go` — implementa `application/interfaces.RepositoryFactory`.
  - `config/categories_cache.go` — **`CategoriesCache` local** com `Boot(ctx)` (carrega raízes oficiais + cache TTL 60s para subcategorias), espelhando `internal/budgets/infrastructure/config/categories_cache.go`. Implementa `application/interfaces.CategoryValidator`.
  - `http/client/card_lookup_adapter.go` — adapter outbound que delega ao **novo use case `internal/card/application/usecases.GetCardForUser`** (in-process); converte erro em `502 card_lookup_failed` apenas em falha não-determinística; recurso ausente vira `404 card_not_found`.
  - `jobs/handlers/recurring_materializer_job.go` — job diário (`@daily` ou cron configurável).
  - `jobs/handlers/monthly_summary_reconciler_job.go` — job diário de reconciliação.
  - `messaging/database/consumers/monthly_summary_recompute_consumer.go` — consumer dos eventos `transactions.transaction.*` + `transactions.card_purchase.*`, com debounce/coalescing por chave.
  - `messaging/database/producers/transaction_event_publisher.go` — publica `transactions.transaction.{created|updated|deleted}.v1`.
  - `messaging/database/producers/card_purchase_event_publisher.go` — publica `transactions.card_purchase.{created|updated|deleted}.v1` (evento único com array de parcelas + `ref_months_affected`).
  - `messaging/database/producers/recurring_template_event_publisher.go` — publica `transactions.recurring_template.{created|updated|deleted}.v1`.
  - `observability/metrics.go` — instrumenta métricas listadas em RT-13 + RF-32.
  - `module.go` — `NewTransactionsModule(cfg, o11y, mgr, cardModule, categoriesModule) (TransactionsModule, error)`. Struct expõe `Router`, `MonthlySummaryRecomputeConsumer`, `RecurringMaterializerJob`, `MonthlySummaryReconcilerJob` e `EventHandlers`. Quando `cfg.TransactionsConfig.Enabled == false`, todos os campos são `nil`/`zero value` e `cmd/api`/`cmd/worker` simplesmente não registram. Boot do módulo aciona `CategoriesCache.Boot(ctx)` (espelhando o padrão `budgets`).

**Modificados**:

- `configs/config.go` — adiciona `TransactionsConfig` (com `Enabled`, `IdempotencyTTL`, `MonthlySummaryDebounceWindow`, `RecurringMaterializerCron`, `MonthlySummaryReconcilerCron`, `MonthlySummaryReconcilerLookbackHours`, `BrazilTimezone`).
- `cmd/api/main.go` — wiring do módulo, registro condicional **apenas do router HTTP** (handlers + middlewares). API **não** registra jobs nem consumer.
- `cmd/worker/main.go` — wiring do módulo, registro condicional do `MonthlySummaryRecomputeConsumer`, `RecurringMaterializerJob` e `MonthlySummaryReconcilerJob` no `WorkerManager`. Worker **não** registra router HTTP.
- `internal/card/application/usecases/get_card_for_user.go` — **novo use case** fino que retorna snapshot `BillingCycle` para consumidores cross-module (incluindo `transactions`); reusa `RepositoryFactory.CardRepository.GetByIDForUser`.
- `internal/card/module.go` — expõe o novo `GetCardForUser` no struct `CardModule` (campo `CardLookup *usecases.GetCardForUser`); `InvoiceFor` existente permanece intacto.
- `migrations/000014_create_transactions_baseline.{up,down}.sql` — schema completo do módulo (numeração confirmada: última é `000013`).
- `mockery.yml` — adicionar 12 mocks de `internal/transactions/application/interfaces/`: `RepositoryFactory`, `TransactionRepository`, `CardPurchaseRepository`, `CardInvoiceRepository`, `RecurringTemplateRepository`, `MonthlySummaryRepository`, `RecurringMaterializationRepository`, `CardLookup`, `CategoryValidator`, `TransactionEventPublisher`, `CardPurchaseEventPublisher`, `RecurringTemplateEventPublisher`.

### Relacionamentos

```
[chi router] ──► [handlers] ──► [usecases] ──► [repositories postgres / cardLookup / categoryValidator]
                                       │
                                       ├──► outbox.Publisher (mesma TX SQL)
                                       │
[outbox.Dispatcher] ──► EventHandlers[“transactions.transaction.*”, “transactions.card_purchase.*”]
                                       │
                                       ▼
                          [MonthlySummaryRecomputeConsumer]
                                       │ (coalescing 1500ms por (user_id, ref_month))
                                       ▼
                          [RecomputeMonthlySummary usecase]
                                       │
                                       ▼
                          [monthly_summary_repository UPSERT]

[WorkerManager] ──► [RecurringMaterializerJob] (cron @daily, lock advisory por (template_id, ref_month))
                ──► [MonthlySummaryReconcilerJob] (cron diário; lookback 48h via TransactionsConfig)
```

### Fluxo de dados — caso crítico (criar compra parcelada)

1. `POST /api/v1/card-purchases` chega no `chi.Router`.
2. Middleware `RequireUser` injeta `auth.Principal` no `ctx`.
3. Middleware `idempotency.Middleware{scope="transactions"}` verifica `Idempotency-Key`.
4. Handler decodifica body → `CreateCardPurchaseInput` → chama `usecases.CreateCardPurchase.Execute(ctx, input)`.
5. Use case:
   - Resolve `CardLookup.GetForUser(ctx, card_id, user_id)` (porta consumidora; impl outbound) → snapshot `closing_day`, `due_day`.
   - Valida `category_id` (e `subcategory_id` se presente) via `CategoryValidator.Validate(ctx, ids)`.
   - `InstallmentSplitter.Split(total_cents, installments_total)` → centavos determinísticos.
   - `BillingCycleResolver.ResolveFirst(purchased_at, closing_day, due_day)` → `ref_month` da parcela 1; incrementa mês a mês para as demais.
   - Abre transação via `uow.New[entities.CardPurchase](mgr)`:
     - Insere `CardPurchase` (com `version=1`).
     - Para cada parcela: `INSERT INTO mecontrola.transactions_card_invoices ... ON CONFLICT (user_id, card_id, ref_month) DO UPDATE SET ... RETURNING id` (upsert idempotente) → insere `CardInvoiceItem(purchase_id, invoice_id, installment_index, ref_month, amount_cents)` → atualiza `transactions_card_invoices.items_total_cents` por `UPDATE ... SET items_total_cents = items_total_cents + $delta, version = version+1 WHERE id = $invoice_id AND version = $expected`. **Em PATCH** (`UpdateCardPurchase`), o `delta` é calculado pelo `Decide*` como `sum(new_items[ref_month]) − sum(old_items[ref_month])` por invoice afetada em `ref_months_affected`: positivo se a parcela cresceu, negativo se diminuiu/foi removida, zero quando não muda valor naquele mês (skip do UPDATE). Optimistic locking via `version` rejeita race com `409 conflict`.
     - `CardPurchaseEventPublisher.Publish(ctx, db, env)` na mesma `db` da TX (DBTX recebido pelo UoW).
   - Commit.
6. Handler responde `201` com `CardPurchaseOutput`.
7. Outbox `Dispatcher` (loop independente) entrega evento.
8. `MonthlySummaryRecomputeConsumer` recebe → debounce 1500 ms por `(user_id, ref_month)` → `RecomputeMonthlySummary(ctx, user_id, ref_month)` para cada competência em `ref_months_affected`.

### Contrato Inegociável de Adapters (R-ADAPTER-001.2)

Os quatro caminhos abaixo são **portas finas** com fluxo único `adapter → usecase → service/repository/client`. Nenhum deles pode conter regra de negócio, SQL direto, branching de domínio, decisão de trigger, cálculo de janela/competência ou orquestração cross-repository. Esta regra é validada pelos gates de R-ADAPTER-001.

| Caminho | Arquivos | Responsabilidade ÚNICA permitida |
|---------|----------|----------------------------------|
| `infrastructure/http/server/handlers/` | `create_transaction_handler.go`, `update_transaction_handler.go`, `delete_transaction_handler.go`, `get_transaction_handler.go`, `list_transactions_handler.go`, `create_card_purchase_handler.go`, `update_card_purchase_handler.go`, `delete_card_purchase_handler.go`, `get_card_purchase_handler.go`, `list_card_purchases_handler.go`, `get_card_invoice_handler.go`, `create_recurring_template_handler.go`, `update_recurring_template_handler.go`, `delete_recurring_template_handler.go`, `get_recurring_template_handler.go`, `list_recurring_templates_handler.go`, `get_monthly_summary_handler.go`, `list_monthly_entries_handler.go` | Decode `chi.Request` → DTO de input; injetar `auth.Principal` no input; invocar **um** use case; encode output ou erro via `responses.*`. Nada mais. |
| `infrastructure/messaging/database/consumers/` | `monthly_summary_recompute_consumer.go` | Decode `outbox.Envelope` → extrair `(user_id, ref_month)` de `ref_months_affected`/payload; delegar a **um** use case `RecomputeMonthlySummary` (via coalescer). Coalescer fica em `infrastructure/messaging/database/consumers/internal/coalescer.go` mas opera só sobre chaves opacas — sem regra de domínio. |
| `infrastructure/jobs/handlers/` | `recurring_materializer_job.go`, `monthly_summary_reconciler_job.go` | Receber tick do `WorkerManager`; calcular `today` em fuso `America/Sao_Paulo`; invocar use case (`MaterializeRecurringForDay` ou `ReconcileMonthlySummary`). Sem SQL, sem decisão de quem materializa, sem split de parcelas — tudo no use case. |
| `infrastructure/messaging/database/producers/` | `transaction_event_publisher.go`, `card_purchase_event_publisher.go`, `recurring_template_event_publisher.go` | Receber `EventEnvelope` (já decidido pelo use case) + `database.DBTX` (TX em curso); serializar payload via `json.Marshal`; chamar `outbox.NewPostgresPublisher(storage, cfg).Publish`. Sem decidir `event_id`, `event_type`, `aggregate_*`, `ref_months_affected` ou trigger. Use case constrói o `EventEnvelope` e entrega ao publisher. |

**Implicações práticas no design dos use cases**:

- Use cases passam `database.DBTX` explícito aos publishers (mesma TX do agregado) e ao repositório; nada de injetar `manager.Manager` no publisher.
- `ref_months_affected` (ADR-003) é calculado **dentro do use case** `UpdateCardPurchase`/`DeleteCardPurchase`, nunca no publisher.
- Coalescer do consumer é estrutura técnica de timer (memória + `sync.Mutex` + `time.Timer`); nenhuma referência a `RefMonth.IsClosed()` ou similar — ele só agenda `func()` que chama o use case.
- Jobs apenas medem o tempo (`time.Now().In(loc)`), montam input do use case e chamam — sem `if today.Day() == template.DayOfMonth` no handler (esse filtro vive no repositório via `FindActiveByDayOfMonth`).

## Design de Implementação

### Interfaces Chave

> **R6.3 (interface no consumidor)**: cada interface declarada no pacote `application/interfaces/` é consumida pelo use case; implementações ficam em `infrastructure/`. Nenhum `var _ Interface = (*Type)(nil)` (R6.4).
>
> **Contrato Inegociável de Repositórios** — espelha `internal/budgets/infrastructure/repositories/postgres/alert_repository.go` (linhas 20–27): `db database.DBTX` é **campo da struct**, injetado no construtor; **nenhum método de repositório pode receber `db` como parâmetro**. Métodos recebem apenas `ctx context.Context` + argumentos de domínio. A `RepositoryFactory` é o único ponto que aceita `db` (no método-fábrica) e o repassa ao construtor.
>
> ```go
> // Padrão obrigatório (espelhando alert_repository.go)
> type transactionRepository struct {
>     db   database.DBTX
>     o11y observability.Observability
> }
>
> func NewTransactionRepository(o11y observability.Observability, db database.DBTX) interfaces.TransactionRepository {
>     return &transactionRepository{db: db, o11y: o11y}
> }
>
> // Métodos NUNCA recebem db — apenas ctx + argumentos de domínio:
> func (r *transactionRepository) GetByID(ctx context.Context, id, userID uuid.UUID) (*entities.Transaction, error) { ... }
> ```
>
> A factory respeita o mesmo contrato:
>
> ```go
> // infrastructure/repositories/repository_factory.go
> func (f *repositoryFactory) TransactionRepository(db database.DBTX) interfaces.TransactionRepository {
>     return postgres.NewTransactionRepository(f.o11y, db)
> }
> ```
>
> Aplica-se **a todos os 6 repositórios** do módulo: `TransactionRepository`, `CardPurchaseRepository`, `CardInvoiceRepository`, `RecurringTemplateRepository`, `MonthlySummaryRepository`, `RecurringMaterializationRepository`. Esta regra é gate de revisão — qualquer método de repositório com `db database.DBTX` na assinatura é rejeitado.
>
> **Distinção** (publishers/use cases): producers **continuam** recebendo `db` como parâmetro em `Publish(ctx, db, env)` porque participam da TX em curso decidida pelo use case (padrão do `internal/budgets/infrastructure/messaging/database/producers/expense_committed_publisher.go`). A regra acima vale **só** para repositórios.

### Padrão de Use Case com `Decide*` puro (ADR-006)

Os 5 workflows não-triviais (`CreateTransaction`, `UpdateTransaction`, `CreateCardPurchase`, `UpdateCardPurchase`, `MaterializeRecurringForDay`) seguem o esqueleto:

```go
// application/usecases/create_card_purchase.go (esqueleto canônico)

// domain/commands/create_card_purchase.go (exportado)
type CreateCardPurchase struct {
    UserID        valueobjects.UserID
    CardID        valueobjects.CardID
    TotalAmount   valueobjects.Money
    Installments  valueobjects.InstallmentCount
    Description   valueobjects.Description
    CategoryID    valueobjects.CategoryID
    SubcategoryID option.Option[valueobjects.SubcategoryID]
    PurchasedAt   time.Time
}

func NewCreateCardPurchase(raw RawCreateCardPurchase, principal auth.Principal) (CreateCardPurchase, error) {
    var errs []error
    amount, err := valueobjects.NewMoney(raw.TotalAmountCents)
    if err != nil { errs = append(errs, fmt.Errorf("total_amount_cents: %w", err)) }
    inst, err := valueobjects.NewInstallmentCount(raw.InstallmentsTotal)
    if err != nil { errs = append(errs, fmt.Errorf("installments_total: %w", err)) }
    // ... demais smart constructors ...
    if len(errs) > 0 {
        return CreateCardPurchase{}, fmt.Errorf("commands/create_card_purchase: %w", errors.Join(errs...))
    }
    return CreateCardPurchase{ /* campos */ }, nil
}

type CreateCardPurchase struct {
    uow                uow.UnitOfWork[entities.CardPurchase]
    factory            interfaces.RepositoryFactory
    cardLookup         interfaces.CardLookup
    categoryValidator  interfaces.CategoryValidator
    workflow           *services.CardPurchaseWorkflow
    publisher          interfaces.CardPurchaseEventPublisher
    idGen              id.Generator
    o11y               observability.Observability
}

func (uc *CreateCardPurchase) Execute(ctx context.Context, raw input.RawCreateCardPurchase) (output.CardPurchase, error) {
    principal, ok := auth.FromContext(ctx)
    if !ok { return output.CardPurchase{}, ErrUnauthorized }

    cmd, err := commands.NewCreateCardPurchase(raw, principal)        // smart constructor + errors.Join (no domain)
    if err != nil { return output.CardPurchase{}, err }

    snapshot, err := uc.cardLookup.GetForUser(ctx, cmd.CardID, cmd.UserID)
    if err != nil { return output.CardPurchase{}, fmt.Errorf("transactions/create_card_purchase: lookup cartão: %w", err) }

    if err := uc.categoryValidator.Validate(ctx, cmd.CategoryID, cmd.SubcategoryID); err != nil {
        return output.CardPurchase{}, fmt.Errorf("transactions/create_card_purchase: validar categoria: %w", err)
    }

    decision := uc.workflow.DecideCreate(cmd, snapshot, uc.idGen.NewID(), uc.idGen.NewID(), time.Now().UTC())  // PURO

    if _, err := uc.uow.Execute(ctx, func(ctx context.Context, db database.DBTX) (entities.CardPurchase, error) {
        purchases := uc.factory.CardPurchaseRepository(db)
        invoices  := uc.factory.CardInvoiceRepository(db)
        if err := purchases.Create(ctx, decision.Purchase); err != nil { return entities.CardPurchase{}, err }
        for _, item := range decision.Items {
            inv, err := invoices.UpsertByMonth(ctx, cmd.UserID, cmd.CardID, item.RefMonth, snapshot)
            if err != nil { return entities.CardPurchase{}, err }
            if err := invoices.AppendItem(ctx, inv, item); err != nil { return entities.CardPurchase{}, err }
        }
        return decision.Purchase, uc.publisher.Publish(ctx, db, decision.Event)
    }); err != nil {
        return output.CardPurchase{}, fmt.Errorf("transactions/create_card_purchase: efetivar: %w", err)
    }
    return output.From(decision), nil
}
```

> **Padrão `Decide*` puro** vive em `domain/services/<aggregate>_workflow.go`:
>
> ```go
> type CardPurchaseDecision struct {
>     Purchase entities.CardPurchase
>     Items    []entities.CardInvoiceItem
>     Event    entities.CardPurchaseCreated  // tipo de domínio, não outbox
> }
>
> func (w *CardPurchaseWorkflow) DecideCreate(
>     cmd commands.CreateCardPurchase,
>     snapshot valueobjects.CardBillingSnapshot,
>     purchaseID, eventID uuid.UUID,
>     now time.Time,
> ) CardPurchaseDecision { ... }
> ```
>
> Sem `ctx`, sem repo, sem `time.Now()` interno. Teste = `suite.Run` puro, zero mocks.
>
> **Publisher fino** consome o domain event direto:
>
> ```go
> type CardPurchaseEventPublisher interface {
>     Publish(ctx context.Context, db database.DBTX, evt entities.CardPurchaseCreated) error
> }
> ```
>
> O `Publish` faz `json.Marshal(evt)` + `outbox.NewPostgresPublisher(...).Publish(ctx, envelope)`. Nada mais. `ref_months_affected` e `event_id` já vêm decididos do `Decide*`.

```go
package interfaces

type RepositoryFactory interface {
    TransactionRepository(db database.DBTX) TransactionRepository
    CardPurchaseRepository(db database.DBTX) CardPurchaseRepository
    CardInvoiceRepository(db database.DBTX) CardInvoiceRepository
    RecurringTemplateRepository(db database.DBTX) RecurringTemplateRepository
    MonthlySummaryRepository(db database.DBTX) MonthlySummaryRepository
    RecurringMaterializationRepository(db database.DBTX) RecurringMaterializationRepository
}

type TransactionRepository interface {
    Create(ctx context.Context, tx *entities.Transaction) error
    UpdateWithVersion(ctx context.Context, tx *entities.Transaction, expectedVersion int64) error
    SoftDelete(ctx context.Context, id uuid.UUID, userID uuid.UUID, expectedVersion int64, now time.Time) error
    GetByID(ctx context.Context, id, userID uuid.UUID) (*entities.Transaction, error)
    ListByMonth(ctx context.Context, userID uuid.UUID, refMonth valueobjects.RefMonth, cursor Cursor, limit int) ([]*entities.Transaction, Cursor, error)
    SumByMonth(ctx context.Context, userID uuid.UUID, refMonth valueobjects.RefMonth) (incomeCents, outcomeCents int64, err error)
}

type CardPurchaseRepository interface {
    Create(ctx context.Context, p *entities.CardPurchase) error
    UpdateWithVersion(ctx context.Context, p *entities.CardPurchase, expectedVersion int64) error
    SoftDelete(ctx context.Context, id, userID uuid.UUID, expectedVersion int64, now time.Time) error
    GetByID(ctx context.Context, id, userID uuid.UUID) (*entities.CardPurchase, error)
    ListByCardAndMonth(ctx context.Context, userID, cardID uuid.UUID, refMonth *valueobjects.RefMonth, cursor Cursor, limit int) ([]*entities.CardPurchase, Cursor, error)
    ReplaceItems(ctx context.Context, purchaseID uuid.UUID, items []*entities.CardInvoiceItem) error
}

type CardInvoiceRepository interface {
    UpsertByMonth(ctx context.Context, userID, cardID uuid.UUID, refMonth valueobjects.RefMonth, closingAt, dueAt time.Time) (*entities.CardInvoice, error)
    // ApplyDelta aplica `delta = sum(new_items[ref_month]) − sum(old_items[ref_month])` em items_total_cents
    // com optimistic locking. delta pode ser negativo (remoção de parcela), positivo (acréscimo) ou zero.
    ApplyDelta(ctx context.Context, invoiceID uuid.UUID, deltaCents int64, expectedVersion int64) error
    GetByMonth(ctx context.Context, userID, cardID uuid.UUID, refMonth valueobjects.RefMonth) (*entities.CardInvoice, []*entities.CardInvoiceItem, error)
    SumByMonth(ctx context.Context, userID uuid.UUID, refMonth valueobjects.RefMonth) (outcomeCents int64, err error)
}

type RecurringTemplateRepository interface {
    Create(ctx context.Context, t *entities.RecurringTemplate) error
    UpdateWithVersion(ctx context.Context, t *entities.RecurringTemplate, expectedVersion int64) error
    SoftDelete(ctx context.Context, id, userID uuid.UUID, expectedVersion int64, now time.Time) error
    GetByID(ctx context.Context, id, userID uuid.UUID) (*entities.RecurringTemplate, error)
    List(ctx context.Context, userID uuid.UUID, activeOnly bool, cursor Cursor, limit int) ([]*entities.RecurringTemplate, Cursor, error)
    FindActiveByDayOfMonth(ctx context.Context, day int, asOf time.Time, cursor Cursor, batchSize int) ([]*entities.RecurringTemplate, Cursor, error)
}

type MonthlySummaryRepository interface {
    Upsert(ctx context.Context, userID uuid.UUID, refMonth valueobjects.RefMonth, incomeCents, outcomeCents int64, updatedAt time.Time) error
    Get(ctx context.Context, userID uuid.UUID, refMonth valueobjects.RefMonth) (*entities.MonthlySummary, error)
    ListActiveSince(ctx context.Context, since time.Time, cursor Cursor, batchSize int) ([]MonthlySummaryKey, Cursor, error)
}

type RecurringMaterializationRepository interface {
    InsertIfAbsent(ctx context.Context, templateID uuid.UUID, refMonth valueobjects.RefMonth, materializedTransactionID, materializedPurchaseID *uuid.UUID, now time.Time) (inserted bool, err error)
    TryAdvisoryLock(ctx context.Context, templateID uuid.UUID, refMonth valueobjects.RefMonth) (acquired bool, release func(), err error)
}

type CardLookup interface {
    GetForUser(ctx context.Context, cardID, userID uuid.UUID) (valueobjects.CardBillingSnapshot, error)
}

type CategoryValidator interface {
    Validate(ctx context.Context, categoryID uuid.UUID, subcategoryID *uuid.UUID) (CategorySnapshot, error)
}
```

> **Producers** declaram interface no use case consumidor (R6.3) recebendo `database.DBTX` para participar da TX do agregado:
>
> ```go
> type TransactionEventPublisher interface {
>     Publish(ctx context.Context, db database.DBTX, env TransactionEventEnvelope) error
> }
> ```

### Modelos de Dados

**Tabelas (mecontrola schema, migration `000014`)**

```sql
CREATE TABLE mecontrola.transactions (
    id                          UUID        NOT NULL,
    user_id                     UUID        NOT NULL,
    direction                   SMALLINT    NOT NULL, -- 1=income, 2=outcome (iota+1)
    payment_method              SMALLINT    NOT NULL, -- enum iota+1; 8=doc legado (somente leitura)
    amount_cents                BIGINT      NOT NULL CHECK (amount_cents > 0),
    description                 TEXT        NOT NULL,
    category_id                 UUID        NOT NULL,
    subcategory_id              UUID        NULL,
    category_name_snapshot      TEXT        NOT NULL,
    subcategory_name_snapshot   TEXT        NULL,
    ref_month                   CHAR(7)     NOT NULL, -- YYYY-MM
    occurred_at                 TIMESTAMPTZ NOT NULL,
    version                     BIGINT      NOT NULL DEFAULT 1,
    deleted_at                  TIMESTAMPTZ NULL,
    created_at                  TIMESTAMPTZ NOT NULL,
    updated_at                  TIMESTAMPTZ NOT NULL,
    CONSTRAINT transactions_pkey PRIMARY KEY (id)
);
CREATE INDEX transactions_user_month_idx
    ON mecontrola.transactions (user_id, ref_month) WHERE deleted_at IS NULL;
CREATE INDEX transactions_user_created_idx
    ON mecontrola.transactions (user_id, created_at DESC, id DESC) WHERE deleted_at IS NULL;

-- Nota: tabela principal `transactions` mantém nome sem prefixo redundante `transactions_transactions`.
-- Demais tabelas do módulo usam prefixo `transactions_*` para alinhar com convenção `<modulo>_<tabela>` do `budgets`.

CREATE TABLE mecontrola.transactions_card_purchases (
    id                          UUID        NOT NULL,
    user_id                     UUID        NOT NULL,
    card_id                     UUID        NOT NULL,
    direction                   SMALLINT    NOT NULL CHECK (direction = 2), -- outcome-only (OUT-16)
    total_amount_cents          BIGINT      NOT NULL CHECK (total_amount_cents > 0),
    installments_total          SMALLINT    NOT NULL CHECK (installments_total BETWEEN 1 AND 24),
    description                 TEXT        NOT NULL,
    category_id                 UUID        NOT NULL,
    subcategory_id              UUID        NULL,
    category_name_snapshot      TEXT        NOT NULL,
    subcategory_name_snapshot   TEXT        NULL,
    purchased_at                TIMESTAMPTZ NOT NULL,
    card_closing_day            SMALLINT    NOT NULL CHECK (card_closing_day BETWEEN 1 AND 31),
    card_due_day                SMALLINT    NOT NULL CHECK (card_due_day BETWEEN 1 AND 31),
    version                     BIGINT      NOT NULL DEFAULT 1,
    deleted_at                  TIMESTAMPTZ NULL,
    created_at                  TIMESTAMPTZ NOT NULL,
    updated_at                  TIMESTAMPTZ NOT NULL,
    CONSTRAINT transactions_card_purchases_pkey PRIMARY KEY (id)
);
CREATE INDEX transactions_card_purchases_user_card_idx
    ON mecontrola.transactions_card_purchases (user_id, card_id, created_at DESC, id DESC) WHERE deleted_at IS NULL;

CREATE TABLE mecontrola.transactions_card_invoices (
    id                  UUID        NOT NULL,
    user_id             UUID        NOT NULL,
    card_id             UUID        NOT NULL,
    ref_month           CHAR(7)     NOT NULL,
    closing_at          TIMESTAMPTZ NOT NULL,
    due_at              TIMESTAMPTZ NOT NULL,
    items_total_cents   BIGINT      NOT NULL DEFAULT 0,
    version             BIGINT      NOT NULL DEFAULT 1,
    created_at          TIMESTAMPTZ NOT NULL,
    updated_at          TIMESTAMPTZ NOT NULL,
    CONSTRAINT transactions_card_invoices_pkey PRIMARY KEY (id),
    CONSTRAINT transactions_card_invoices_uk   UNIQUE (user_id, card_id, ref_month)
);

CREATE TABLE mecontrola.transactions_card_invoice_items (
    id                  UUID        NOT NULL,
    invoice_id          UUID        NOT NULL REFERENCES mecontrola.transactions_card_invoices(id),
    purchase_id         UUID        NOT NULL REFERENCES mecontrola.transactions_card_purchases(id),
    user_id             UUID        NOT NULL,
    ref_month           CHAR(7)     NOT NULL,
    installment_index   SMALLINT    NOT NULL,
    amount_cents        BIGINT      NOT NULL CHECK (amount_cents > 0),
    deleted_at          TIMESTAMPTZ NULL,
    created_at          TIMESTAMPTZ NOT NULL,
    updated_at          TIMESTAMPTZ NOT NULL,
    CONSTRAINT transactions_card_invoice_items_pkey        PRIMARY KEY (id),
    CONSTRAINT transactions_card_invoice_items_purchase_uk  UNIQUE (purchase_id, installment_index)
);
CREATE INDEX transactions_card_invoice_items_user_month_idx
    ON mecontrola.transactions_card_invoice_items (user_id, ref_month) WHERE deleted_at IS NULL;

CREATE TABLE mecontrola.transactions_recurring_templates (
    id                          UUID        NOT NULL,
    user_id                     UUID        NOT NULL,
    direction                   SMALLINT    NOT NULL,
    payment_method              SMALLINT    NOT NULL,
    card_id                     UUID        NULL,
    amount_cents                BIGINT      NOT NULL CHECK (amount_cents > 0),
    description                 TEXT        NOT NULL,
    category_id                 UUID        NOT NULL,
    subcategory_id              UUID        NULL,
    category_name_snapshot      TEXT        NOT NULL,
    subcategory_name_snapshot   TEXT        NULL,
    frequency                   SMALLINT    NOT NULL, -- 1=monthly, 2=yearly
    day_of_month                SMALLINT    NOT NULL CHECK (day_of_month BETWEEN 1 AND 28),
    installments_total          SMALLINT    NOT NULL DEFAULT 1 CHECK (installments_total BETWEEN 1 AND 24),
    started_at                  TIMESTAMPTZ NOT NULL,
    ended_at                    TIMESTAMPTZ NULL,
    version                     BIGINT      NOT NULL DEFAULT 1,
    deleted_at                  TIMESTAMPTZ NULL,
    created_at                  TIMESTAMPTZ NOT NULL,
    updated_at                  TIMESTAMPTZ NOT NULL,
    CONSTRAINT transactions_recurring_templates_pkey PRIMARY KEY (id),
    CONSTRAINT transactions_recurring_templates_credit_chk
        CHECK ((payment_method <> 7 /* credit_card */) OR (card_id IS NOT NULL))
);
CREATE INDEX transactions_recurring_templates_user_day_idx
    ON mecontrola.transactions_recurring_templates (user_id, day_of_month) WHERE deleted_at IS NULL;

CREATE TABLE mecontrola.transactions_recurring_materializations (
    template_id                 UUID        NOT NULL REFERENCES mecontrola.transactions_recurring_templates(id),
    ref_month                   CHAR(7)     NOT NULL,
    materialized_transaction_id UUID        NULL,
    materialized_purchase_id    UUID        NULL,
    materialized_at             TIMESTAMPTZ NOT NULL,
    CONSTRAINT transactions_recurring_materializations_pkey PRIMARY KEY (template_id, ref_month)
);

CREATE TABLE mecontrola.transactions_monthly_summary (
    user_id        UUID        NOT NULL,
    ref_month      CHAR(7)     NOT NULL,
    income_cents   BIGINT      NOT NULL DEFAULT 0,
    outcome_cents  BIGINT      NOT NULL DEFAULT 0,
    total_cents    BIGINT      NOT NULL DEFAULT 0,
    version        BIGINT      NOT NULL DEFAULT 1,
    updated_at     TIMESTAMPTZ NOT NULL,
    CONSTRAINT transactions_monthly_summary_pkey PRIMARY KEY (user_id, ref_month)
);
```

> **Notas de schema**
> - Enum `payment_method` (iota+1): `pix=1`, `ted=2`, `debit_in_account=3`, `debit_card=4`, `cash=5`, `boleto=6`, `credit_card=7`, `doc=8` (R5.8).
> - Sem FK entre módulos (RT-22): `transactions.category_id`/`transactions.subcategory_id` apontam logicamente para `internal/categories`, sem `REFERENCES`.
> - `transactions_card_invoice_items_purchase_uk(purchase_id, installment_index)` é a garantia de "uma parcela por índice" em re-edição/cascata.
> - `transactions_recurring_materializations` PK `(template_id, ref_month)` é a garantia única de idempotência (RF-32).
> - Convenção de nomes: módulo `transactions` segue `<modulo>_<tabela>` (espelhando `budgets_expenses`, `budgets_allocations`); única exceção é a tabela principal `mecontrola.transactions` para evitar redundância `transactions_transactions`.

### Endpoints de API

| Método | Path | Use case | Idempotente | Locking |
|--------|------|----------|-------------|---------|
| POST   | `/api/v1/transactions` | `CreateTransaction` | header | — |
| PATCH  | `/api/v1/transactions/{id}` | `UpdateTransaction` | header | `version` |
| DELETE | `/api/v1/transactions/{id}` | `DeleteTransaction` | header | `version` |
| GET    | `/api/v1/transactions/{id}` | `GetTransaction` | — | — |
| GET    | `/api/v1/transactions?ref_month=&cursor=&limit=` | `ListTransactions` | — | — |
| POST   | `/api/v1/card-purchases` | `CreateCardPurchase` | header | — |
| PATCH  | `/api/v1/card-purchases/{id}` | `UpdateCardPurchase` | header | `version` |
| DELETE | `/api/v1/card-purchases/{id}` | `DeleteCardPurchase` | header | `version` |
| GET    | `/api/v1/card-purchases/{id}` | `GetCardPurchase` | — | — |
| GET    | `/api/v1/card-purchases?card_id=&ref_month=&cursor=&limit=` | `ListCardPurchases` | — | — |
| GET    | `/api/v1/cards/{card_id}/invoices/{ref_month}` | `GetCardInvoice` | — | — |
| POST   | `/api/v1/recurring-templates` | `CreateRecurringTemplate` | header | — |
| PATCH  | `/api/v1/recurring-templates/{id}` | `UpdateRecurringTemplate` | header | `version` |
| DELETE | `/api/v1/recurring-templates/{id}` | `DeleteRecurringTemplate` | header | `version` |
| GET    | `/api/v1/recurring-templates/{id}` | `GetRecurringTemplate` | — | — |
| GET    | `/api/v1/recurring-templates?active=&cursor=&limit=` | `ListRecurringTemplates` | — | — |
| GET    | `/api/v1/months/{ref_month}` | `GetMonthlySummary` | — | — |
| GET    | `/api/v1/months/{ref_month}/entries?cursor=&limit=` | `ListMonthlyEntries` | — | — |

> **Códigos de erro** (RF-44/45): `{ "message", "code" }` via `responses.ErrorWithDetails`. Códigos: `validation_error`, `not_found`, `conflict`, `idempotency_conflict`, `transaction_version_conflict`, `card_purchase_version_conflict`, `card_lookup_failed`, `category_not_found`.

## Pontos de Integração

- **`internal/identity`** — `auth.Principal` via `RequireUser` (reuso direto). Sem mudança contratual.
- **`internal/card`** — `CardLookup` (porta consumidora declarada em `internal/transactions/application/interfaces/`); adapter em `infrastructure/http/client/card_lookup_adapter.go` chama `cardModule.CardLookup` (atual `InvoiceFor`/`GetByIDForUser`). Sem retry adicional (httpclient wrapper já cuida). Falha não-determinística → `502 card_lookup_failed`.
- **`internal/categories`** — `CategoryValidator` (porta consumidora); adapter delega ao use case `ValidateSubcategory` do módulo. Recurso de outro usuário ou inexistente → `404 category_not_found`.
- **`internal/platform/outbox`** — publish atrelado à TX SQL do agregado; entrega at-least-once + DLQ lógica controlados por `OutboxConfig` existente.
- **`internal/platform/idempotency`** — `Middleware` aplicado às rotas de mutação com `scope="transactions"` e `TTL=24h` (cravado em `TransactionsConfig.IdempotencyTTL`).
- **`internal/platform/worker`** — `RecurringMaterializerJob` e `MonthlySummaryReconcilerJob` via `job.NewAdapter`; `MonthlySummaryRecomputeConsumer` via `consumer.NewAdapter` + `consumer.Registry`.

## Abordagem de Testes

### Testes Unitários (`mockery` + `testify/suite`)

> Conforme `references/testing.md` e `R3`/`R4`.

- **Domínio**: `InstallmentSplitter`, `BillingCycleResolver`, `RefMonthResolver` — 100% table-driven; cenários: total ímpar, parcela única, 24 parcelas, virada de mês, fuso `America/Sao_Paulo`.
- **Use cases**: cada usecase com mocks de repos + `CardLookup` + `CategoryValidator`. Cenários críticos: `Idempotency-Key` replay (cache hit), `version` mismatch (409), `card_lookup_failed`, `category_not_found`, `ref_month` mudou em PATCH (recompute de duas competências), `ref_months_affected` cobre antiga ∪ nova em PATCH de `CardPurchase`.
- **Producers**: garantir que payload contém `ref_months_affected` ordenado e sem duplicatas; `event_id` UUID válido; `metadata.trace_id` propagado.
- **Consumer `MonthlySummaryRecompute`**: debounce coalescing — 10 eventos da mesma chave em 200 ms → 1 recompute; eventos de chaves diferentes não coalescem; idempotência de reprocesso.
- **Job recorrência**: lock advisory adquirido/liberado; `InsertIfAbsent` retorna `false` → métrica `skipped_total` incrementada; templates de crédito chamam `CreateCardPurchase`; templates de débito chamam `CreateTransaction`.

### Testes de Integração

Critérios "sim" para: (a) fronteiras de IO críticas (Postgres, outbox), (b) optimistic locking real só valida sob concorrência real, (c) testcontainers já é padrão do repo (`internal/platform/testcontainer.Postgres`).

- **Build tag**: `//go:build integration`.
- **Cobertura mínima**:
  - `repositories/postgres/*_integration_test.go` — CRUD + cursor + version conflict + soft-delete + `UPSERT ON CONFLICT` em `card_invoices`.
  - `producers/*_integration_test.go` — evento gravado em `platform.outbox` na mesma TX do agregado; rollback do agregado descarta o evento.
  - `consumers/monthly_summary_recompute_consumer_integration_test.go` — entrega at-least-once + debounce + idempotência.
  - `jobs/recurring_materializer_job_integration_test.go` — duas execuções no mesmo dia → uma única materialização por `(template_id, ref_month)`; lock advisory bloqueia concorrência.
  - `jobs/monthly_summary_reconciler_job_integration_test.go` — drift artificial → métrica incrementada + projeção corrigida.
  - `migrations/migrations_integration_test.go` — migra up/down do `000014` sem perda.

### Testes E2E

Fora do escopo deste PRD (módulo backend; consumers próprios validam ponta-a-ponta no pipeline de release). Smoke test via `k6` (AS-07) é responsabilidade de homologação, não obrigatório no MVP.

## Sequenciamento de Desenvolvimento

### Ordem de Build

1. **`internal/card/application/usecases/get_card_for_user.go`** — novo use case fino que retorna `BillingCycle` snapshot; pré-requisito para `transactions` consumir snapshot estático sem acoplar ao repositório.
2. **Migration `000014_create_transactions_baseline`** + `migrations_integration_test.go` — base inegociável.
3. **`domain/option/`** (ADR-006 §4) — `Option[T any]` genérico (`Some`, `None`, `Get`, `IsPresent`); ≤ 30 linhas; testes 100% coverage.
4. **`domain/valueobjects/`** (ADR-006 §1) — smart constructors obrigatórios para `Money`, `InstallmentCount`, `DayOfMonth`, `Description`, `RefMonth`, `Direction`, `PaymentMethod`, `Frequency`, `UserID`, `CardID`, `CategoryID`, `SubcategoryID`, **`CardBillingSnapshot`** (audit fix #2). Cada VO com sentinel errors + table-driven unit tests.
5. **`domain/commands/`** (audit fix #1) — tipos exportados (`CreateTransaction`, `UpdateTransaction`, `CreateCardPurchase`, `UpdateCardPurchase`, `CreateRecurringTemplate`, `UpdateRecurringTemplate`, `MaterializeRecurring`) + smart constructors `NewXxx(raw RawXxx, principal auth.Principal) (Xxx, error)`. Vive no domínio para evitar import cycle quando workflows recebem o tipo. Espelha `internal/budgets/domain/commands/`.
6. **`domain/entities/`** — agregados (`Transaction`, `CardPurchase`, `CardInvoice`, `CardInvoiceItem`, `RecurringTemplate`, `MonthlySummary`) construídos exclusivamente via factories que recebem VOs já validados.
7. **`domain/entities/events.go`** (ADR-006 §3) — 9 structs de domain event (`TransactionCreated/Updated/Deleted`, `CardPurchaseCreated/Updated/Deleted`, `RecurringTemplateCreated/Updated/Deleted`); cada um carrega `EventID`, `AggregateID`, `UserID`, `OccurredAt` + campos específicos (`RefMonthsAffected`, `InvoiceDeltas` quando aplicável).
8. **`domain/services/`** — funções puras `InstallmentSplitter`, `BillingCycleResolver` (com clamp `min(day, last_day_of_target_month)` — audit fix #3), `RefMonthResolver` + 3 workflows `Decide*` (`transaction_workflow.go`, `card_purchase_workflow.go`, `recurring_workflow.go`). Testes table-driven sem mocks.
8. **`application/interfaces/` + `dtos/input,output/`** — contratos consumidos pelos use cases; `CardPurchaseEventPublisher.Publish` recebe `entities.CardPurchaseCreated` (não `EventEnvelope` genérico).
9. **`CategoriesCache` local + `categories_reader_adapter`** — espelha o `budgets`; `Boot(ctx)` carrega raízes oficiais; satisfaz `CategoryValidator`.
10. **`card_lookup_adapter`** — delega ao `internal/card.GetCardForUser`; converte erros para `404 card_not_found` / `502 card_lookup_failed`.
11. **Use cases de Transaction** (`Create/Update` com `Decide*` puro + validate; `Delete/Get/List` orquestradores diretos sem `Decide*` — ADR-006 §"Práticas proibidas") com mocks + integração contra Postgres.
12. **Use cases de CardPurchase** — `Create/Update` consomem `CardPurchaseWorkflow.Decide*` com `cascade` via `ReplaceItems`; `ref_months_affected` calculado no `Decide*`, nunca no use case ou producer.
13. **Use cases de RecurringTemplate** — CRUD direto; `MaterializeRecurringForDay` consome `RecurringWorkflow.DecideMaterializeForDay` e delega criação a `CreateTransaction` ou `CreateCardPurchase` conforme `payment_method` (decisão também vem do `Decide*`).
14. **Use cases de MonthlySummary** (`Recompute`, `Reconcile`, `Get`, `ListEntries`) + projection upsert.
15. **Producers finos** — 3 publishers; cada `Publish(ctx, db, evt)` recebe domain event tipado e faz só `json.Marshal` + `outbox.Publisher`.
16. **Consumer** — `MonthlySummaryRecomputeConsumer` (fino: decode envelope → coalescer → use case `RecomputeMonthlySummary`).
17. **HTTP handlers + router** — fino (R-ADAPTER-001.2); `RequireUser` + `idempotency.Middleware` + decode raw → use case.
18. **Job handlers** — adaptados a `worker.Job` via `job.NewAdapter` (fino: tick → use case).
19. **`module.go`** — DI manual no padrão `BudgetsModule`; gating por `TransactionsConfig.Enabled`; injeta `*services.<Aggregate>Workflow` nos use cases.
20. **Wiring em `cmd/api`** — registra **apenas router HTTP** (handlers/middleware) se `Enabled`.
21. **Wiring em `cmd/worker`** — registra **apenas consumer + jobs** no `WorkerManager` se `Enabled`.
22. **Observabilidade** — métricas + dashboard `transactions-overview` + 4 alertas (Grafana).
23. **`.claude/rules/transactions-workflows.md`** (ADR-006) — regra hard codificando lista dos 5 workflows com `Decide*` obrigatório + gate de revisão "regra de domínio fora de `Decide*` = bloqueia PR".
24. **Runbook** `docs/runbooks/transactions.md` — 3 cenários mínimos (AS-12).

### Dependências Técnicas

- Postgres ≥ 14 (advisory locks; `ON CONFLICT`).
- `internal/card.CardLookup` precisa expor `GetForUser(ctx, cardID, userID)` — método já existe via `RepositoryFactory`.
- `internal/categories.ValidateSubcategory` — já existe.
- `internal/platform/outbox`, `internal/platform/idempotency`, `internal/platform/worker` — já existem; sem mudança.
- `mockery.yml` precisa registrar 9 novas interfaces antes de qualquer unit test (R3).

## Monitoramento e Observabilidade

> `observability.md` da skill — `log/slog`, métricas RED + saúde de pipeline.

### Métricas Prometheus

- `transactions_transactions_created_total{direction,payment_method}`
- `transactions_transactions_updated_total{direction,payment_method}`
- `transactions_transactions_deleted_total{direction,payment_method}`
- `transactions_card_purchases_created_total{installments_bucket}` (buckets: `1`, `2-6`, `7-12`, `13-24`)
- `transactions_card_purchases_updated_total{installments_bucket}`
- `transactions_card_purchases_deleted_total{installments_bucket}`
- `transactions_recurring_template_created_total{frequency,payment_method}`
- `transactions_recurring_materialize_attempt_total{frequency}`
- `transactions_recurring_materialize_skipped_total{reason}` (`reason="already_materialized"` | `"lock_not_acquired"`)
- `transactions_recurring_materialize_duration_seconds` (Histogram)
- `transactions_write_duration_seconds{operation}` (Histogram)
- `transactions_read_duration_seconds{operation}` (Histogram)
- `transactions_monthly_summary_recompute_duration_seconds` (Histogram)
- `transactions_monthly_summary_coalesce_factor` (Histogram — eventos colapsados por recompute)
- `transactions_monthly_summary_drift_total{kind}` (`kind="detected"` | `"corrected"`)
- `transactions_outbox_consumer_lag_seconds` (Gauge — re-exposto a partir do consumer)
- `transactions_outbox_dead_letter_total`
- `transactions_idempotency_replay_total{operation}`
- `transactions_card_lookup_failure_total`

> **Cardinalidade** (RT-15): nenhum label carrega `user_id` ou `category_id`. `installments_bucket` em vez de `installments_total` direto.

### Logs

`log/slog` com chaves obrigatórias: `module="transactions"`, `operation`, `user_id`, `aggregate_id`, `ref_month`, `trace_id`. **Proibido logar**: `description`, `amount_cents`, `category_name_snapshot` (RT-13).

### Spans OTel

Spans `transactions.<layer>.<operation>` (ex.: `transactions.usecase.create_card_purchase`, `transactions.repository.upsert_invoice`, `transactions.consumer.recompute_monthly_summary`). `trace_id` propagado em `outbox.Event.Metadata` (RT-14).

### Alertas Grafana

1. **Drift** — `increase(transactions_monthly_summary_drift_total{kind="detected"}[1d]) > 0` por 15 min → warning.
2. **Write p99** — `histogram_quantile(0.99, sum by (le) (rate(transactions_write_duration_seconds_bucket[5m]))) > 0.3` por 10 min → critical.
3. **Consumer lag** — `transactions_outbox_consumer_lag_seconds > 5` por 5 min → critical.
4. **Dead-letter** — `increase(transactions_outbox_dead_letter_total[15m]) > 0` → critical.

Dashboard `transactions-overview` com painéis: RED por endpoint, consumer lag, drift, idempotency replay, recorrência (attempt vs skipped), card_lookup failures.

## Considerações Técnicas

### Decisões Chave (ADRs vinculadas)

- **ADR-001** — Snapshot estático de `BillingCycle` em `CardPurchase`; sem consumir eventos `card.*` no MVP.
- **ADR-002** — Materialização de recorrência via job **diário** filtrando templates por `day_of_month == today` (vs lote único dia 1).
- **ADR-003** — UM ÚNICO evento por `CardPurchase` (`v1`) carregando array de parcelas + `ref_months_affected` (vs N eventos por parcela).
- **ADR-004** — Debounce/coalescing por `(user_id, ref_month)` em janela 1500 ms no consumer de `MonthlySummary`.
- **ADR-005** — Edição retroativa silenciosa em faturas fechadas com `ref_months_affected` no response/evento (vs bloquear ou flag opt-in).
- **ADR-006** — Adoção seletiva de Domain Modeling Made Functional: smart constructors obrigatórios em VOs, passo `Decide*` puro em `domain/services/` para os 5 workflows não-triviais (Create/Update `Transaction`, Create/Update `CardPurchase`, `MaterializeRecurringForDay`), domain events como tipos do `domain/entities/events.go` (producers só mapeiam → outbox envelope), `Option[T]` para opcionais, `errors.Join` para acumular validação. Proibido: `Result[T,E]` monad, function-as-DI, `Decide*` em CRUD trivial.

### Riscos Conhecidos

| Risco | Impacto | Mitigação |
|-------|---------|-----------|
| `time.AfterFunc` para debounce vaza timer em shutdown | Lifecycle quebrado (`graceful-lifecycle.md`) | Consumer mantém `map[key]*time.Timer` sob mutex; `Stop()` cancela todos no `ctx.Done()`; pendentes são drenados sincronamente até `ShutdownTimeout`. |
| Cascata de edição em `CardPurchase` em fatura com muitas parcelas (24) em TX longa | Lock no Postgres > 1s no p99 | `SET LOCAL lock_timeout='5s'` na TX; `ReplaceItems` faz `DELETE+INSERT` em batch único; benchmark obrigatório no integration test. |
| Reconciliador diário corrige drift, mas se houver bug no usecase, drift cresce e métrica só dispara warning | Diferença silenciosa de até 1d | Alerta de drift escalado para **critical** se `> 1% de usuários ativos no dia` (futuro); MVP fica em warning. Runbook obriga investigação na primeira ocorrência. |
| Snapshot estático causa surpresa quando usuário muda dia de vencimento do cartão | UX confuso ("por que minha fatura velha não mudou?") | Documentado em OUT-17. Front-end deve exibir tooltip "competência baseada nas datas do cartão no momento da compra". |
| `RecurringMaterializerJob` diário com 100k templates ativos no mesmo `day_of_month==15` causa spike de carga | Latência write p99 > 300 ms entre 00:00 e 01:00 | Job processa em batches de 200 templates com `FindActiveByDayOfMonth(cursor)`; metric `recurring_materialize_duration_seconds` observada; cron configurável para fora do horário comercial. |
| Premissa de base vazia (AS-11) falsa em produção | `monthly_summary` ausente confunde cliente | RF-28 já cobre (200 com totais zerados, `updated_at=null`). Sem alteração. |

### Conformidade com Padrões

- **R-GOV-001** — fonte de verdade: este techspec carrega `architecture.md`, `api.md`, `persistence.md`, `messaging.md`, `observability.md`, `testing.md` sob demanda (≤ 4 por seção). Inclui a nova regra de precedência `domain-modeling.md` (DMMF adaptado) sobre estilo idiomático genérico para regras de **tipo e estado** (smart constructor, discriminated union, state-as-type, workflow pipeline). Anti-padrões `hard` permanecem proibidos: `Result/Either` customizado, currying, DSL de pipeline (ver ADR-006 e `.claude/rules/governance.md`).
- **ADR-006 (Domain Modeling Made Functional seletivo)** — smart constructors obrigatórios em todos os VOs; passo `Decide*` puro em `domain/services/` para os 5 workflows não-triviais; domain events tipados em `domain/entities/events.go`; `Option[T]` para opcionais; `errors.Join` para acumular validação. Gate de revisão: regra de domínio fora de `Decide*` bloqueia PR.
- **R-ADAPTER-001.1** — zero comentários em `.go` de produção. Gate de verificação documentado no CLAUDE.md.
- **R-ADAPTER-001.2** — todos os handlers, consumers, jobs e producers são adapters finos (`adapter → usecase`); SQL direto e branching de domínio proibidos.
- **R-ADAPTER-001.3** — referências do go-implementation por tipo de adapter respeitadas; `patterns-structural.md` não é carregado.
- **R0** — sem `init()`.
- **R1** — toda função é método de struct, exceto factories `New*`.
- **R5.8** — enums `iota+1`; zero value reservado.
- **R5.10** — wrapping com `%w` e contexto PT-BR (`"transactions/usecase: criar lançamento: %w"`).
- **R5.12** — sem `panic` em produção.
- **R6** — `context.Context` em toda fronteira de IO; interface no consumidor.
- **R6.4** — sem `var _ Interface = (*Type)(nil)`.
- **R6.7** — sem `clock.Clock`; `time.Now().UTC()` inline; `now` recebido por command quando determinístico em teste.
- **R7.2** — `log/slog` via `observability.Logger`.
- **R7.6** — `errors.Join` para agregar erros (ex.: validação de input).
- **Padrão Obrigatório de Módulo** — DI manual em `module.go` estilo `BudgetsModule`.
- **LGPD baseline** — TLS + criptografia at-rest gerenciada pelo Postgres; sem PII em logs (RT-13); sem `pgcrypto` aplicativo (OUT-11).

### Arquivos Relevantes e Dependentes

**Criados** (resumo):
- `internal/transactions/**` (estrutura completa do módulo)
  - `domain/option/option.go` (ADR-006 §4)
  - `domain/valueobjects/{money,installment_count,day_of_month,description,ref_month,direction,payment_method,frequency,user_id,card_id,category_id,subcategory_id}.go` (ADR-006 §1)
  - `domain/entities/events.go` (ADR-006 §3 — 9 domain events tipados)
  - `domain/services/{installment_splitter,billing_cycle_resolver,ref_month_resolver,transaction_workflow,card_purchase_workflow,recurring_workflow}.go` (ADR-006 §2)
- `migrations/000014_create_transactions_baseline.{up,down}.sql`
- `docs/runbooks/transactions.md`
- `.claude/rules/transactions-workflows.md` (ADR-006 — gate hard de "regra de domínio fora de `Decide*` bloqueia PR")
- `.specs/prd-transactions-monthly/adr-001-billing-cycle-snapshot.md`
- `.specs/prd-transactions-monthly/adr-002-recurring-job-daily-by-day-of-month.md`
- `.specs/prd-transactions-monthly/adr-003-single-card-purchase-event.md`
- `.specs/prd-transactions-monthly/adr-004-monthly-summary-debounce.md`
- `.specs/prd-transactions-monthly/adr-005-silent-retroactive-edit.md`
- `.specs/prd-transactions-monthly/adr-006-dmmf-selective-adoption.md`

**Modificados**:
- `configs/config.go` — `TransactionsConfig`.
- `cmd/api/main.go` — wiring + registro condicional.
- `cmd/worker/main.go` (equivalente) — jobs + consumer.
- `mockery.yml` — 9 interfaces novas.
- Dashboard Grafana `transactions-overview` (não-Go, separado).

**Não modificados (apenas consumidos)**:
- `internal/identity/application/auth/principal.go`
- `internal/card/application/usecases/invoice_for.go` (intacto; usado por `internal/card` próprio)
- `internal/card/infrastructure/repositories/**` (reuso via `GetCardForUser`)
- `internal/categories/**` (consumido via `ValidateSubcategory` no `categories_reader_adapter`)
- `internal/platform/outbox/**`
- `internal/platform/idempotency/**`
- `internal/platform/worker/**`
