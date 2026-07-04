# Runbook — Módulo `internal/transactions` (CRUD Unificado)

- Escopo: CRUD unificado de transações (receita + despesa, todos os meios de pagamento).
- Origem: `.specs/prd-transactions-crud-unificado/` (PRD, techspec, ADR-001/002/003).

## Visão do fluxo unificado

Existe uma única porta de escrita de transações: `POST/PATCH/DELETE /api/v1/transactions`.
O meio de pagamento é um enum fechado. Quando `payment_method=credit_card` (válido apenas para
`direction=outcome`), o fluxo resolve/abre a fatura de competência do cartão e distribui as
parcelas (1..24) determinísticamente; para os demais meios, é lançamento simples e imediato.

- Regra de cartão embutida: `CardLookup.GetForUser` → `CardBillingSnapshot` (congelado na criação)
  → `InstallmentSplitter` + `BillingCycleResolver` (puros, em `Decide*`) → `card_invoice_items`
  vinculados a `transactions(id)` + upsert de `transactions_card_invoices` com `ApplyDelta` (OCC).
- Atomicidade: create/update/delete de crédito ocorrem em um único `uow.Do`. Falha em qualquer
  passo → rollback total; nenhuma fatura fica com saldo residual. Conflito OCC → 409.
- Edição não pode migrar `payment_method` de/para `credit_card` (muda a forma do agregado):
  rejeitada com HTTP 422 `payment_migration_forbidden`.

## Fonte única do resumo mensal (ADR-003) — evita double-counting

A transação `credit_card` (`payment_method = 7`) é **excluída** do ramo `transactions` na agregação
mensal:

- `TransactionRepository.SumByMonthExcludingCredit` usa `WHERE payment_method <> 7`.
- `ListEntries` (UNION ALL) filtra `payment_method <> 7` no ramo `transactions`.
- O valor de crédito entra **apenas** via `CardInvoiceRepository.SumByMonth` (soma de
  `card_invoice_items`).

Cada real é contado uma única vez. Se um total mensal divergir da soma independente das faturas,
verifique se algum agregador voltou a usar `SumByMonth` do ramo transactions (regressão proibida).

## Integração com recorrência e visão mensal

- Recorrência: `materialize_recurring_for_day` materializa via `CreateTransaction` unificado
  (inclusive `credit_card`, montando `RawCreateTransaction{payment_method: credit_card, card_id,
  installments}`); a recorrência não é reimplementada.
- Visão mensal: `MonthlySummaryRecomputeConsumer` processa `transactions.transaction.{created,
  updated,deleted}.v1` iterando `ref_months_affected` (recompute por mês afetado), com coalescing.

## Observabilidade

- Spans por use case: `transactions.usecase.create_transaction`, `.update_transaction`,
  `.delete_transaction`, `.reconcile_monthly_summary`, `.materialize_recurring_for_day`, etc.
- Atributos do ramo crédito (sem PII): `installments_total`, `ref_months_affected_count`.
- Métrica de coalescing do recompute: `transactions_monthly_summary_coalesce_factor`.
- Cardinalidade controlada (R-TXN-004): labels permitidos `operation`, `payment_method`,
  `installments_bucket`. Proibido `user_id`, `category_id`, `card_id` como label de métrica.
- Dashboard: `docs/dashboards/transactions-overview.json`. Alertas: `docs/alerts/transactions.yaml`.

## Descontinuação de `card-purchase` (RF-24 / RF-24a / ADR-002)

A superfície `card-purchase` foi removida (rotas `/api/v1/card-purchases`, handlers, use cases,
producer e a tabela `transactions_card_purchases`). Compras no crédito são criadas exclusivamente
pelo CRUD unificado. As rotas legadas retornam 404.

### Gate pré-release obrigatório (irreversível — confirmar contra produção antes do drop)

A migration `000003` executa `DROP TABLE transactions_card_purchases` sem backfill. Antes de
aplicar o release em produção, confirme que a tabela está vazia. Abortar o release se a contagem
for maior que zero.

```sql
SELECT count(*) AS card_purchases_remaining
  FROM mecontrola.transactions_card_purchases;
```

- Resultado esperado: `0`. Se `> 0`, **abortar o release**: existem dados reais de compras no
  crédito que seriam perdidos pelo drop (a decisão RF-24a só é válida com ledger vazio).
- Rollback: `migrate down` recria `transactions_card_purchases` vazia + revert do PR.

Verificação operacional pós-corte:

- `POST|GET /api/v1/card-purchases` → 404 (rotas removidas).
- Nenhum evento `transactions.card_purchase.*` no outbox após o deploy.
