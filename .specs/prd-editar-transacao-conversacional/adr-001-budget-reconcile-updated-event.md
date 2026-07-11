# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Reflexo no orçamento da edição via evento `TransactionUpdated` enriquecido + usecase de reconciliação
- **Data:** 2026-07-10
- **Status:** Aceita
- **Decisores:** Autor da techspec, aprovação do solicitante (múltipla escolha, recomendação aceita)
- **Relacionados:** PRD `.specs/prd-editar-transacao-conversacional/prd.md` (D-01, D-15, RF-27..30), techspec.md, `.claude/rules/transactions-workflows.md`, `.claude/rules/go-adapters.md`

## Contexto

Editar uma transação publica `transactions.transaction.updated.v1`, mas `internal/budgets` só consome `created.v1` e `deleted.v1` (`internal/budgets/module.go:144-145`) e o evento `TransactionUpdated` não carrega categoria (`internal/transactions/domain/entities/events.go:26-36`). Além disso, para compras de cartão, o lado transactions recria os `CardInvoiceItem` com **novos** `ItemID` a cada edição (`transaction_workflow.go:226-238` + `ReplaceItems`), enquanto o orçamento indexa cada parcela por `external_transaction_id = ItemID` (`source=transactions_card`). Sem reconciliação, editar uma compra parcelada deixa as parcelas antigas como consumo fantasma e não insere as novas. `UpsertExpense` não é upsert cego: existente + `ExpectedVersion==nil` é no-op (`upsert_expense.go:150-152`).

## Decisão

1. Enriquecer `TransactionUpdated` (aditivo, compatível) com `CategoryID uuid.UUID`, `SubcategoryID uuid.UUID`, `Installments []CardPurchaseInstallment` (conjunto atual; vazio para não-cartão) e `PreviousItemIDs []uuid.UUID` (itens de fatura antigos a remover). O `DecideUpdate` (puro) popula a partir de `cmd` e `currentItems`.
2. Criar `internal/budgets/application/usecases/reconcile_transaction_update.go` (`ReconcileTransactionUpdate`) que, numa UoW: remove a representação anterior (não-cartão: por `aggregateID`; cartão: por `PreviousItemIDs`) e aplica a atual (não-cartão: update in-place por version, `MutationKindUpdate`; cartão: insere as parcelas atuais). Trata migração pix↔crédito.
3. Criar `TransactionUpdatedConsumer` fino (molde `transaction_created_consumer.go`) que decodifica o envelope, filtra `direction=outcome` e delega ao usecase; registrar em `internal/budgets/module.go`.

## Alternativas Consideradas

- **Re-key estável `(transactionID, index)` + upsert in-place + migração de dados:** identidade mais limpa a longo prazo, mas exige migração dos `budgets_expenses` de cartão existentes e altera os consumers created/deleted — blast radius e risco em produção maiores. Rejeitada.
- **Snapshot completo (delete-tudo-do-tx + reinsere):** `budgets_expenses` não tem coluna `transaction_id`; exigiria `PreviousItemIDs` de qualquer forma e perderia continuidade/version de todas as linhas (inclusive não-cartão), degradando a semântica de alertas. Rejeitada.

## Consequências

### Benefícios Esperados

- Fecha o consumo fantasma e a defasagem de resumo/alertas na edição, inclusive parcelas e migração de forma de pagamento.
- Zero migração de dados; enriquecimento aditivo e compatível.
- Consumidor fino; regra de reconciliação isolada em usecase (R-ADAPTER-001).

### Trade-offs e Custos

- Payload do evento cresce (parcelas + IDs antigos).
- Cartão emite Delete+Create de `ExpenseCommitted` por competência (correto por mês, maior volume de eventos).

### Riscos e Mitigações

- Risco: reprocessamento do evento. Mitigação: reconcile idempotente (delete-por-identidade + upsert-por-identidade); teste de redelivery.
- Risco: parcela extinta em 3x→2x. Mitigação: `PreviousItemIDs` cobre a remoção; teste de integração dedicado.
- Rollback: reverter o registro do consumidor em `module.go` desabilita o reflexo sem afetar a escrita da transação.

## Plano de Implementação

1. Enriquecer struct + `DecideUpdate` (puro) + testes.
2. `ReconcileTransactionUpdate` + mocks + testes.
3. `TransactionUpdatedConsumer` + registro + testes.
4. Integração transactions→budgets (testcontainers).

## Monitoramento e Validação

- Métrica `budgets_reconcile_transaction_update_total{outcome}` (enum: `updated`/`recreated`/`noop`/`conflict`), decode/skip counters. Sem `user_id`/`category_id` como label.
- Sucesso: após editar valor/categoria/data e parcelas, `GetMonthlySummary` e thresholds refletem o estado novo, sem linha fantasma.
- Reverter se surgir divergência sistemática de consumo pós-edição.

## Impacto em Documentação e Operação

- Runbook do orçamento: novo consumidor e métricas. Dashboards: painel de reconcile. Contrato de evento `updated.v1` documentado.

## Revisão Futura

- Revisar se a identidade de parcela por `ItemID` gerar custo operacional recorrente (candidata a re-key estável numa iniciativa futura).
