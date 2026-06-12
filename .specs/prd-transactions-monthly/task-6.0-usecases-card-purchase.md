# Tarefa 6.0: Use cases CardPurchase + 2 repos + producer + cascade com `ApplyDelta`

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Implementa os 6 use cases de `CardPurchase` + `CardInvoice` (`Create`, `Update` com cascade, `Delete`, `Get`, `List`, `GetCardInvoice`), os repositórios postgres (`card_purchase_repository.go` + `card_invoice_repository.go`), e o `card_purchase_event_publisher.go` (1 evento único com `RefMonthsAffected` + `InvoiceDeltas`). A cascade em PATCH usa fórmula `delta = sum(new[ref]) − sum(old[ref])` por invoice afetada (audit fix #4). Cobre clamp de fevereiro (audit fix #3).

<requirements>
- Use cases em `application/usecases/`: `create_card_purchase.go`, `update_card_purchase.go`, `delete_card_purchase.go`, `get_card_purchase.go`, `list_card_purchases.go`, `get_card_invoice.go`. `Create`/`Update`/`Delete` consomem `CardPurchaseWorkflow.Decide*`.
- `Update` cascateia silenciosamente em faturas fechadas (ADR-005); response inclui `RefMonthsAffected`.
- `card_purchase_repository.go` com `db` campo da struct: `Create`, `UpdateWithVersion`, `SoftDelete`, `GetByID`, `ListByCardAndMonth`, `ReplaceItems`.
- `card_invoice_repository.go` com `db` campo: `UpsertByMonth` (`ON CONFLICT (user_id, card_id, ref_month) DO UPDATE`), `ApplyDelta(ctx, invoiceID, deltaCents int64, expectedVersion int64) error` com optimistic locking (audit fix #4), `GetByMonth`, `SumByMonth`.
- `ApplyDelta` aceita delta negativo (remoção/redução), positivo (acréscimo) e zero (skip UPDATE).
- `card_purchase_event_publisher.go` fino: 1 evento único `transactions.card_purchase.{created|updated|deleted}.v1` com array completo de parcelas + `RefMonthsAffected` + `InvoiceDeltas` (ADR-003).
- `CardLookup` chamado apenas em `Create` (snapshot estático — ADR-001); `Update` reusa snapshot da `CardPurchase` existente.
- `CategoryValidator` em `Create` e `Update`.
- Integration tests cobrindo: PATCH 12→3 parcelas (delta negativo em 9 meses); compra com `closing_day=30` em fevereiro (clamp dia 28); compra `purchased_at` no dia de fechamento (limite RF-14); concorrência → 409 via `version`.
- Filtro `user_id` em todas as queries.
</requirements>

## Subtarefas

- [ ] 6.1 `create_card_purchase.go` + unit test (workflow puro + mocks).
- [ ] 6.2 `update_card_purchase.go` + unit test cobrindo cascade 12→3 + `InvoiceDeltas` negativos.
- [ ] 6.3 `delete_card_purchase.go` + unit test (soft-delete pai + items em 1 TX).
- [ ] 6.4 `get_card_purchase.go`, `list_card_purchases.go`, `get_card_invoice.go` + unit tests.
- [ ] 6.5 `card_purchase_repository.go` + integration test (CRUD + `ReplaceItems` atômico).
- [ ] 6.6 `card_invoice_repository.go` (`UpsertByMonth` + `ApplyDelta` com version) + integration test cobrindo delta negativo e race condition.
- [ ] 6.7 `card_purchase_event_publisher.go` + integration test (1 evento por mutação com array de items + `RefMonthsAffected`).

## Detalhes de Implementação

Referência: techspec "Modelos de Dados" (`transactions_card_purchases`, `transactions_card_invoices`, `transactions_card_invoice_items` + constraint `_purchase_uk`); ADR-001 (snapshot estático), ADR-003 (single event), ADR-005 (cascade silencioso), ADR-006 (Decide* puro). RF-11 a RF-22, RF-36.

## Critérios de Sucesso

- `go test -race -count=1 ./internal/transactions/application/usecases/...` passa para os 6 use cases.
- Integration test do `card_invoice_repository.ApplyDelta` valida: delta positivo, negativo, zero (skip), conflict de version.
- Integration test do `update_card_purchase` cobrindo PATCH 12→3 atualiza `items_total_cents` de 12 invoices.
- Integration test cobrindo `closing_day=30` em fevereiro retorna `closing_at` no dia 28.
- `golangci-lint run ./internal/transactions/...` limpo no escopo da task.
- Zero comentários em `.go` de produção.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff). -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Unit tests dos 6 use cases com mocks.
- [ ] Integration test `card_purchase_repository_integration_test.go`.
- [ ] Integration test `card_invoice_repository_integration_test.go` (foco em `ApplyDelta`).
- [ ] Integration test `card_purchase_event_publisher_integration_test.go`.
- [ ] Integration test E2E: PATCH cascade 12→3 com asserts em invoices + outbox.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/transactions/application/usecases/{create,update,delete,get,list}_card_purchase.go` (novos)
- `internal/transactions/application/usecases/get_card_invoice.go` (novo)
- `internal/transactions/application/usecases/*_test.go` (novos)
- `internal/transactions/infrastructure/repositories/postgres/card_purchase_repository.go` (novo)
- `internal/transactions/infrastructure/repositories/postgres/card_invoice_repository.go` (novo)
- `internal/transactions/infrastructure/repositories/postgres/*_integration_test.go` (novos)
- `internal/transactions/infrastructure/messaging/database/producers/card_purchase_event_publisher.go` (novo)
- `internal/transactions/infrastructure/messaging/database/producers/card_purchase_event_publisher_integration_test.go` (novo)
