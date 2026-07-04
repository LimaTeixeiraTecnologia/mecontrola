<!-- spec-hash-prd: 4a96b713bde8f8d820a85a3ba0c262be8b6f27ab92219483baff2089ed98f18d -->
<!-- MANDATÓRIO: preenchido por `create-technical-specification` Etapa 7.1 com sha256 do PRD consumido.
     Rastreabilidade: `create-tasks` e `execute-task` comparam este hash com o atual do prd.md
     para detectar drift entre techspec e PRD. NÃO remover este comentário ao editar a techspec. -->

# Especificação Técnica — CRUD Unificado de Transações

## Resumo Executivo

Unificar `internal/transactions` numa única porta de escrita de transações (income + outcome).
A superfície `card-purchase` é **removida** e sua lógica de domínio (parcelamento + fatura) é
**absorvida** pelo fluxo de transação quando `payment_method=credit_card` (`direction=outcome`
obrigatório). A absorção reusa integralmente os `Decide*` puros já existentes
(`InstallmentSplitter`, `BillingCycleResolver`, `CardPurchaseWorkflow`) — nada de lógica pura é
reescrito; o que muda é o agregado que os hospeda e o wiring que os invoca.

Decisão arquitetural central (ADR-001, ADR-003): **uma única tabela `transactions`** ganha colunas
nullable de cartão; `card_invoice_items` re-aponta para `transactions(id)`; `transactions_card_purchases`
é dropada. Como a produção tem ledger vazio (RF-24a), a migration é um corte limpo sem backfill.
O despacho `credit_card` vs. simples é **orquestração no use case selecionando o path de decisão puro**
(não Strategy de classe, não `switch` de domínio) — a regra de negócio permanece 100% em `Decide*`
(ADR-001, coerente com DMMF e R-TXN-001). Os bugs de correção que tocam o fluxo unificado
(double-counting no resumo mensal, swallow de erro, RF-18/RF-21 de categoria, snapshot de nome,
`DeleteTransaction` fora de `Decide*`) são corrigidos no mesmo escopo por serem pré-requisito de "0-gap".

## Arquitetura do Sistema

### Visão Geral dos Componentes

**Domínio (modificado):**
- `entities.Transaction` (`domain/entities/transaction.go`) — ganha campos nullable `cardID`,
  `installmentsTotal`, `billingSnapshot` (só populados quando `credit_card`) + reconstituição.
- `entities.TransactionCreated/Updated/Deleted` (`domain/entities/events.go`) — ganham
  `RefMonthsAffected []RefMonth` e (no Created) `Installments []CardPurchaseInstallment`, nil para
  não-cartão.
- `services.TransactionWorkflow` (`domain/services/transaction_workflow.go`) — `DecideCreate`/
  `DecideUpdate` enriquecidos para, quando `credit_card`, compor `InstallmentSplitter` +
  `BillingCycleResolver` e emitir `[]CardInvoiceItem` + `InvoiceDeltas`; novo `DecideDelete` puro
  (hoje o delete monta evento fora de `Decide*` — corrigido).
- Reusados sem reescrita: `InstallmentSplitter`, `BillingCycleResolver`, `CardBillingSnapshot`,
  `InstallmentCount` (1..24).

**Value Objects (modificado):**
- `PaymentMethod` (`domain/valueobjects/payment_method.go`) — adiciona `PaymentMethodMealVoucher` (9)
  e `PaymentMethodFoodVoucher` (10); atualiza `ParsePaymentMethod`, `String`, `PaymentMethodFromInt`
  (bound 1..10). `doc` permanece bloqueado em `ParsePaymentMethodForCreate`.

**Commands (modificado):**
- `commands.CreateTransaction`/`UpdateTransaction` — ganham `CardID option.Option[CardID]` e
  `Installments option.Option[InstallmentCount]`; smart constructor passa a rejeitar
  `credit_card` sem `card_id` (`ErrCommandCreditCardRequiresCardID`, já existe) e `credit_card`
  com `direction != outcome` (novo `ErrCommandCreditCardRequiresOutcome`).

**Application (modificado):**
- `usecases.CreateTransaction/UpdateTransaction/DeleteTransaction` — injetam `cardLookup`,
  `cardInvoiceRepo` e `cardPublisher`; quando `credit_card`, fazem o IO (`GetForUser`) antes do
  `Decide*` e, dentro do mesmo UoW, upsert de fatura + `ApplyDelta` + `ReplaceItems` + publish.
- `interfaces.CategoryValidator`/`CategorySnapshot` — `CategorySnapshot` ganha `Kind` e `ParentID`;
  validação passa a exigir raiz (RF-17), filha direta (RF-18) e coerência kind↔direction (RF-21).
- Novo método de repositório `GetItemsByTransactionID(ctx, txID) ([]CardInvoiceItem, error)` —
  substitui a re-derivação por resolver na leitura de estado (elimina fragilidade).

**Infraestrutura (removido/modificado):**
- Removidos: 5 handlers `*_card_purchase_handler.go`, 5 rotas `/api/v1/card-purchases`, 5 use cases
  `*_card_purchase.go`, DTOs `raw_*_card_purchase.go`, `output/card_purchase.go` (após mover
  `CardInvoiceItemOutput`), `CardPurchaseEventPublisher`.
- Migração de schema (ADR-003): colunas de cartão em `transactions`, re-target da FK de
  `card_invoice_items`, drop de `transactions_card_purchases`, drop de `materialized_purchase_id`.

**Consumidor Agente (modificado — decisão do usuário):**
- `internal/agents`: `register_expense` passa a aceitar `payment_method=credit_card` + `card_id` +
  `installments`; removidas as tools `register_card_purchase`, `get_card_purchase`,
  `list_card_purchases` e os 5 métodos correspondentes do binding/interface.

### Fluxo de Dados (create credit_card unificado)

```
POST /api/v1/transactions {payment_method:"credit_card", direction:"outcome", card_id, installments, ...}
  → idempotency.Middleware → CreateTransactionHandler.Handle (adapter fino)
  → CreateTransaction.Execute:
      raw.Validate()  (surface: card_id p/ credit_card, installments 1..24)
      cmd = NewCreateTransaction(...)  (smart ctor: credit_card⇒outcome, credit_card⇒card_id)
      guard outcome⇒subcategory (RF-19); categoryValidator.Validate (RF-17/18/21)
      snapshot = cardLookup.GetForUser(card_id, user)         ← IO, só credit_card
      decision = workflow.DecideCreate(cmd, snapshot, txID, eventID, now)   ← PURO
         → Transaction(+card fields) , []CardInvoiceItem , TransactionCreated(RefMonthsAffected,Installments)
      uow.Do(tx):
         repo.Create(transaction)                              (ON CONFLICT origin DO NOTHING)
         for item: invoiceRepo.UpsertByMonth → ApplyDelta(OCC)
         repo.ReplaceItems(txID, items)
         publisher.PublishCreated(TransactionCreated)          (outbox, EventID = PK)
  → outbox → MonthlySummaryRecomputeConsumer (ref_months_affected) → RecomputeMonthlySummary
```

## Design de Implementação

### Interfaces Chave

```go
type TransactionWorkflow struct{ cards services.BillingCycleResolver }

func (w TransactionWorkflow) DecideCreate(
    cmd commands.CreateTransaction, snap option.Option[valueobjects.CardBillingSnapshot],
    txID, eventID uuid.UUID, now time.Time,
) TransactionDecision

func (w TransactionWorkflow) DecideUpdate(
    current entities.Transaction, currentItems []entities.CardInvoiceItem,
    cmd commands.UpdateTransaction, eventID uuid.UUID, now time.Time,
) TransactionDecision

func (w TransactionWorkflow) DecideDelete(
    current entities.Transaction, currentItems []entities.CardInvoiceItem,
    eventID uuid.UUID, now time.Time,
) (TransactionDecision, error)

type TransactionDecision struct {
    Transaction  entities.Transaction
    Items        []entities.CardInvoiceItem   // nil quando != credit_card
    InvoiceDeltas map[string]int64             // nil quando != credit_card
    Event        any                           // TransactionCreated|Updated|Deleted
}
```

```go
type TransactionRepository interface {
    Create(ctx, *Transaction) (uuid.UUID, bool, error)
    UpdateWithVersion(ctx, *Transaction, expectedVersion int64) error
    SoftDelete(ctx, id, userID uuid.UUID, expectedVersion int64, now time.Time) error
    GetByID(ctx, id, userID uuid.UUID) (*Transaction, error)
    GetItemsByTransactionID(ctx, txID uuid.UUID) ([]*CardInvoiceItem, error) // novo
    ReplaceItems(ctx, txID uuid.UUID, items []*CardInvoiceItem) error        // movido de CardPurchaseRepo
    ExistsActiveCreditByCard(ctx, cardID, userID uuid.UUID) (bool, error)    // movido; payment_method=7
    ListByMonth(ctx, userID, refMonth, cursor, limit) ([]*Transaction, Cursor, error)
    SearchByDescription(ctx, userID, q, refMonth, limit) ([]*Transaction, error)
    SumByMonthExcludingCredit(ctx, userID, refMonth) (income, outcome int64, err error) // WHERE payment_method<>7
}
```

### Modelos de Dados

`entities.Transaction` (campos adicionados, nil-safe):
```go
cardID            option.Option[valueobjects.CardID]
installmentsTotal option.Option[valueobjects.InstallmentCount]
billingSnapshot   option.Option[valueobjects.CardBillingSnapshot]  // congelado na criação
```

Regra de fonte única para resumo mensal (evita double-counting, ADR-003):
- Transação `credit_card` NÃO entra em `SumByMonthExcludingCredit` (ramo `transactions`).
- Valor da fatura vem exclusivamente de `CardInvoiceRepository.SumByMonth` (via `card_invoice_items`).
- `ListEntries` (UNION ALL) filtra `payment_method <> 7` no ramo `transactions`.

### Migration (ver ADR-003 para o SQL completo e idempotente)

`000003_unify_transactions.up.sql` — incremental, idempotente (`IF EXISTS`/`IF NOT EXISTS`):
1. `ALTER TABLE transactions ADD COLUMN card_id, installments_total, card_closing_day, card_due_day`
   + CHECK de completude quando `payment_method=7` + FK `card_id→cards(id)`.
2. `card_invoice_items`: `RENAME purchase_id → transaction_id`; UNIQUE `(transaction_id, installment_index)`;
   FK `→ transactions(id)`.
3. `transactions_recurring_materializations DROP COLUMN materialized_purchase_id`.
4. `DROP TABLE transactions_card_purchases`.
Sem backfill (ledger vazio, RF-24a). `transactions_card_invoices` permanece intacta.

### Endpoints de API

Mantidos (contrato de request estendido):
- `POST /api/v1/transactions` — campos novos opcionais `card_id`, `installments` (default 1);
  obrigatórios quando `payment_method=credit_card`.
- `PATCH /api/v1/transactions/{id}` — `installments` opcional; `payment_method` não pode migrar
  de/para `credit_card` (novo sentinel — muda de forma de agregado).
- `GET /api/v1/transactions`, `GET /api/v1/transactions/{id}`, `DELETE /api/v1/transactions/{id}` —
  response ganha `card_id`, `installments_total`, `ref_months_affected`, `items` (omitempty).
- `GET /api/v1/cards/{id}/invoices/{ref_month}` — inalterado (fatura permanece).

Removidos: `POST|GET|GET{id}|PATCH{id}|DELETE{id} /api/v1/card-purchases`.

OpenAPI: schema de `RawCreateTransaction` completo com condicional (`if payment_method==credit_card
then required [card_id]`), via OpenAPI 3.1 `if/then`. `RawCreateCardPurchase` removido.

### Validação (fronteira → smart constructor → use case)

| Regra | Camada | RF |
|---|---|---|
| campos obrigatórios, amount>0, card_id p/ credit_card, installments 1..24 | `Raw*.Validate()` (errors.Join) | RF-03, RF-13, RF-11b |
| enum payment_method válido, `doc` bloqueado | `ParsePaymentMethodForCreate` | RF-05, RF-07 |
| credit_card ⇒ outcome; credit_card ⇒ card_id | smart ctor `NewCreateTransaction` | RF-11a, RF-11b |
| outcome ⇒ subcategory obrigatória; income ⇒ opcional | use case (antes do validator) | RF-19, RF-20 |
| category_id é raiz (parent_id null) | `CategoriesCache.Validate` | RF-17 |
| subcategory é filha direta de category_id | `ValidateSubcategory(expectedParentID)` | RF-18 |
| kind da categoria == toKind(direction) | use case pós-validator | RF-21 |
| split determinístico, soma = total | `InstallmentSplitter` (Decide*) | RF-15 |
| distribuição por mês de fatura | `BillingCycleResolver` (Decide*) | RF-12, RF-15 |

## Pontos de Integração

- `internal/card` via `CardLookup.GetForUser` → `BillingCycle{closing,due}` (só credit_card). Erro
  `ErrCardNotFound` mapeado para 404/422. Sem mudança no módulo card.
- `internal/categories` via `ResolveBySlug`/`ValidateSubcategory` — `ValidateSubcategory` ganha
  `expectedParentID` (RF-18) e retorna `Kind`/`CategoryName`/`ParentName` (corrige bug de snapshot).
- `internal/agents` — binding/tools refatorados para o CRUD unificado (decisão do usuário).
- Outbox/idempotência inalterados: `EventID` (PK, ON CONFLICT DO NOTHING) e
  `idempotency.Middleware` preservam exactly-once por chave.

## Abordagem de Testes

### Testes Unitários (padrão R-TESTING-001, testify/suite whitebox, fake.NewProvider)
- `TransactionWorkflow.DecideCreate/Update/Delete`: puros, sem mock — casos income, outcome simples,
  credit_card 1x (à vista), credit_card 12x (split + refMonths + soma=total), edição que recompõe
  deltas, delete que reverte deltas. Cobrem RF-12..RF-16a.
- Smart constructors: credit_card sem card_id → erro; credit_card income → erro; installments 0/25 →
  erro; `doc` na criação → erro; vale_refeicao/vale_alimentacao válidos. RF-05..RF-14.
- `CategoriesCache.Validate`: category não-raiz → erro (RF-17); subcategory de outra raiz → erro
  (RF-18); kind≠direction → erro (RF-21); income sem subcategory → ok (RF-20).
- Use cases (mock repo/lookup/publisher): outcome sem subcategory → erro (RF-19, create E update);
  double-count guard (SumByMonthExcludingCredit chamado).

### Testes de Integração
Critérios do template atendidos (fronteira de IO crítica: Postgres + outbox + OCC de fatura;
correção não garantida por mock). **Adotar** testcontainers-go, build tag `//go:build integration`:
- create credit_card 12x → 12 `card_invoice_items` em 12 faturas, cada `items_total_cents` correto,
  soma = total; um único `TransactionCreated` com `ref_months_affected` de 12 meses.
- update de parcelas (12→6) → deltas aplicados atômicos em todas as faturas afetadas; falha
  simulada no meio → rollback total (nenhuma fatura alterada). RF-16.
- delete → reversão de todos os deltas, faturas sem saldo residual. RF-16a.
- resumo mensal: credit_card NÃO é double-counted (soma via invoice apenas). ADR-003.
- migration up/down em banco limpo: colunas/constraints/FK criadas; `card_purchases` inexistente.

### Testes E2E
- Jornada HTTP: criar despesa pix; criar compra credit_card parcelada; listar mês; editar; remover;
  confirmar 404 nas rotas `/card-purchases` removidas. Reexecutar tool do agente `register_expense`
  com credit_card (validação real-LLM conforme feedback do projeto).

## Sequenciamento de Desenvolvimento

### Ordem de Build
1. VOs + commands (PaymentMethod 9/10; card fields; sentinels credit_card⇒outcome/card_id). Base pura.
2. `TransactionWorkflow` enriquecido + `DecideDelete` + testes unitários puros (reusa splitter/resolver).
3. Migration `000003` + repos (`GetItemsByTransactionID`, `ReplaceItems`, `SumByMonthExcludingCredit`,
   `ExistsActiveCreditByCard`) + `ListEntries` filtrado.
4. Use cases unificados (create/update/delete) com IO de cartão condicional dentro do UoW.
5. Categorias: `ValidateSubcategory(expectedParentID)` + `CategorySnapshot{Kind,ParentID,Name}` +
   correção do adapter (snapshot de nome) + RF-17/18/21 no cache.
6. HTTP: DTOs unificados + Validate; remover rotas/handlers/DTOs/use cases de card-purchase; OpenAPI.
7. Agente: refatorar `register_expense` + binding + remover tools card-purchase.
8. Wiring `module.go`; integração + e2e; gates de conformidade.

### Dependências Técnicas
- Postgres (migration 000003 aplicável sobre baseline atual). Sem novas infra externas.
- `internal/card` e `internal/categories` já expõem as portas necessárias (extensão mínima em
  `ValidateSubcategory`).

## Monitoramento e Observabilidade

- Spans já existentes por use case (`transactions.usecase.*`) preservados; branch credit_card
  adiciona atributos `installments_total`, `ref_months_affected_count` (sem PII).
- Métricas Prometheus com cardinalidade controlada (R-TXN-004): labels `operation`
  (`create_transaction`|`update_transaction`|`delete_transaction`), `payment_method`,
  `installments_bucket`. Proibido `user_id`/`category_id`/`card_id` como label.
- Reusar `transactions_idempotency_replay_total` (label `operation`). Recompute mensal segue
  debounced pelo consumer existente.
- Dashboards/alertas de `docs/dashboards/transactions-overview.json` e `docs/alerts/transactions.yaml`
  revisados p/ remover séries de `card_purchase.*` e cobrir credit_card unificado.

## Considerações Técnicas

### Decisões Chave (cada uma com ADR)
- **ADR-001** — Despacho por `PaymentMethod`: orquestração no use case seleciona o path de decisão
  puro; regra vive em `Decide*`. Rejeitado Strategy de classe (over-engineering; só credit_card
  diverge) e `switch` de domínio no use case (viola R-TXN-001).
- **ADR-002** — Corte e descarte de `card-purchase`: remoção imediata da superfície + drop de dados
  (ledger vazio) + refactor das tools do agente. Rollback = revert do PR + migration down.
- **ADR-003** — Schema unificado + fonte única do resumo mensal: uma tabela `transactions`,
  `card_invoice_items→transactions`, credit_card fora de `SumByMonth`/`ListEntries` do ramo
  transactions (evita double-counting).

### Riscos Conhecidos
- **OCC em N faturas numa operação** (edição/remoção parcelada): até 24 `ApplyDelta` sequenciais no
  mesmo tx. Mitigação: escrita serializada por usuário (idempotency middleware + outbox UNIQUE
  `aggregate_user_id WHERE status=2`) torna conflito concorrente improvável; conflito → rollback
  total + 409 ao cliente. Retry automático fica fora deste escopo (documentado, não silenciado).
- **`billingSnapshot` congelado**: edição usa o snapshot da criação (plano de parcelas não muda
  retroativamente se o cartão mudar fechamento). Comportamento correto e testado.
- **Bug de snapshot de nome** (adapter grava slug composto): corrigido; risco de dados legados é nulo
  (ledger vazio).
- **Contrato do agente durante transição**: mitigado por refatorar tools no mesmo PR (sem intervalo
  quebrado).

### Conformidade com Padrões
- `.claude/rules/transactions-workflows.md` (R-TXN-001..004): `Decide*` puro, validação em smart
  ctor, producers só mapeiam evento, cardinalidade de métricas.
- `.claude/rules/go-adapters.md` (R-ADAPTER-001): zero comentários, handlers finos, sem SQL/branch
  de domínio em adapter.
- `.claude/rules/input-dto-validate.md` (R-DTO-001..004): `Validate()` com `errors.Join`, sem
  duplicar semântica de enum/whitelist.
- `.claude/rules/go-testing.md` (R-TESTING-001): suite whitebox, `fake.NewProvider`, IIFE por mock.
- `.claude/skills/go-implementation/` + DMMF: state-as-type, smart constructors, sem abstração de
  tempo, `time.Now().UTC()` inline; refactoring.guru consultado só p/ o veredito de despacho (ADR-001).

### Arquivos Relevantes e Dependentes
Ver a lista consolidada `file:line` nos 5 relatórios de exploração (transaction core, card-purchase,
persistência/migrations, HTTP/DTO, categorias). Núcleo: `domain/services/transaction_workflow.go`,
`domain/services/card_purchase_workflow.go`, `domain/entities/{transaction,events,card_invoice_item}.go`,
`domain/valueobjects/payment_method.go`, `application/usecases/{create,update,delete}_transaction.go`,
`infrastructure/repositories/postgres/*`, `infrastructure/http/server/transactions_router.go`,
`internal/transactions/openapi.yaml`, `migrations/000001_initial_schema.up.sql`,
`internal/agents/{application/tools,infrastructure/binding}`.

## Mapa Requisito → Decisão → Teste

| RF | Decisão/Local | Teste |
|---|---|---|
| RF-01/02 | CRUD único, soft-delete+version | unit + e2e CRUD |
| RF-05..09 | enum PaymentMethod 9/10; doc bloqueado | unit smart ctor |
| RF-10/11 | branch só credit_card em Decide* | unit workflow |
| RF-11a/11b | sentinels credit_card⇒outcome/card_id | unit smart ctor |
| RF-12/14 | BillingCycleResolver + upsert fatura; installments default 1 | integ create credit_card |
| RF-13/15 | InstallmentCount 1..24; InstallmentSplitter soma=total | unit workflow |
| RF-16/16a | DecideUpdate/DecideDelete deltas atômicos (UoW) | integ update/delete |
| RF-17/18/21 | CategoriesCache raiz + filha direta + kind↔direction | unit categorias |
| RF-19/20 | guard subcategory por direction | unit use case create+update |
| RF-22/23 | recorrência materializa via CreateTransaction; resumo via eventos | integ + e2e |
| RF-24/24a | remoção rotas + drop tabela | integ migration + e2e 404 |
