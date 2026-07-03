# Mapa Capacidade → Tool — MeControla Agent

<!-- spec-version: 3 -->
<!-- source-of-truth: internal/agents/module.go#buildFinancialTools -->
<!-- generated: 2026-07-02 -->

## Método de Reprodução

Este mapa é derivado diretamente do código-fonte. Para reproduzir:

```bash
# 1. Ferramentas registradas (25 esperadas)
grep -rn 'tool.NewTool\[' internal/agents/application/tools/*.go \
  | grep -oP '"[a-z_]+"' | sort

# 2. Funções Build* em buildFinancialTools (module.go)
grep 'agenttools\.Build' internal/agents/module.go | wc -l

# 3. Gate de compilação
go mod verify && go build ./... && go vet ./...
```

## Bindings → Interfaces

| Binding | Interface | Arquivo |
|---------|-----------|---------|
| `NewTransactionsLedgerAdapter` | `TransactionsLedger` | `binding/transactions_ledger_adapter.go` |
| `NewCardManagerAdapter` | `CardManager` | `binding/card_manager_adapter.go` |
| `NewBudgetPlannerAdapter` | `BudgetPlanner` | `binding/budget_planner_adapter.go` |
| `NewCategoriesReaderAdapter` | `CategoriesReader` | `binding/categories_reader_adapter.go` |
| `NewRecurrenceManagerAdapter` | `RecurrenceManager` | `binding/recurrence_manager_adapter.go` |

## Bucket 1 — Totalmente Implementadas (25 tools)

Tool existe + binding existe + wired em `buildFinancialTools`.

### Registro (escrita idempotente) — 4 tools

| Tool | Arquivo da Tool | Binding/Interface | Use Cases |
|------|-----------------|-------------------|-----------|
| `register_expense` | `tools/register_expense.go` | `TransactionsLedger.CreateTransaction` | `transactions.CreateTransactionUC` |
| `register_income` | `tools/register_income.go` | `TransactionsLedger.CreateTransaction` | `transactions.CreateTransactionUC` |
| `register_card_purchase` | `tools/register_card_purchase.go` | `TransactionsLedger.CreateCardPurchase` + `CardManager` | `transactions.CreateCardPurchaseUC` |
| `create_recurrence` | `tools/create_recurrence.go` | `RecurrenceManager.CreateRecurrence` | `transactions.CreateRecurringTemplateUC` |

### Consultas de Lançamentos — 5 tools

| Tool | Arquivo da Tool | Binding/Interface | Use Cases |
|------|-----------------|-------------------|-----------|
| `query_month` | `tools/query_month.go` | `TransactionsLedger.ListMonthlyEntries` + `GetMonthlySummary` | `transactions.ListMonthlyEntriesUC` + `GetMonthlySummaryUC` |
| `get_transaction` | `tools/get_transaction.go` | `TransactionsLedger.GetTransaction` | `transactions.GetTransactionUC` |
| `get_card_purchase` | `tools/get_card_purchase.go` | `TransactionsLedger.GetCardPurchase` | `transactions.GetCardPurchaseUC` |
| `list_card_purchases` | `tools/list_card_purchases.go` | `TransactionsLedger.ListCardPurchases` | `transactions.ListCardPurchasesUC` |
| `search_transactions` | `tools/search_transactions.go` | `TransactionsLedger.SearchTransactions` | `transactions.SearchTransactionsUC` |

### Cartões — 5 tools

| Tool | Arquivo da Tool | Binding/Interface | Use Cases |
|------|-----------------|-------------------|-----------|
| `list_cards` | `tools/list_cards.go` | `CardManager.ListCards` | `card.ListCardsUC` |
| `get_card` | `tools/get_card.go` | `CardManager.GetCard` | `card.GetCardUC` |
| `count_cards` | `tools/count_cards.go` | `CardManager.CountCards` | `card.CountCardsUC` |
| `best_purchase_day` | `tools/best_purchase_day.go` | `CardManager.BestPurchaseDay` | `card.BestPurchaseDayUC` |
| `query_card_invoice` | `tools/query_card_invoice.go` | `TransactionsLedger.GetCardInvoice` | `transactions.GetCardInvoiceUC` |

### Recorrências — 3 tools

| Tool | Arquivo da Tool | Binding/Interface | Use Cases |
|------|-----------------|-------------------|-----------|
| `list_recurrences` | `tools/list_recurrences.go` | `RecurrenceManager.ListRecurrences` | `transactions.ListRecurringTemplatesUC` |
| `update_recurrence` | `tools/update_recurrence.go` | `workflow.Engine[ConfirmState]` → `RecurrenceManager.UpdateRecurrence` | `transactions.UpdateRecurringTemplateUC` |
| `delete_recurrence` | `tools/delete_recurrence.go` | `workflow.Engine[ConfirmState]` → `RecurrenceManager.DeleteRecurrence` | `transactions.DeleteRecurringTemplateUC` |

### Categorias e Orçamento — 5 tools

| Tool | Arquivo da Tool | Binding/Interface | Use Cases |
|------|-----------------|-------------------|-----------|
| `list_categories` | `tools/list_categories.go` | `CategoriesReader.ListCategories` | `categories.ListCategoriesUC` |
| `classify_category` | `tools/classify_category.go` | `CategoriesReader.SearchDictionary` | `categories.SearchDictionaryUC` |
| `query_plan` | `tools/query_plan.go` | `BudgetPlanner.GetMonthlySummary` + `ListAlerts` | `budgets.GetMonthlySummaryUC` + `ListAlertsUC` |
| `adjust_allocation` | `tools/adjust_allocation.go` | `BudgetPlanner.EditCategoryPercentage` | `budgets.EditCategoryPercentageUC` |
| `suggest_allocation` | `tools/suggest_allocation.go` | `BudgetPlanner.SuggestAllocation` | `budgets.SuggestAllocationUC` |

### Edição e Exclusão (com confirmação) — 3 tools

| Tool | Arquivo da Tool | Binding/Interface | Use Cases |
|------|-----------------|-------------------|-----------|
| `edit_entry` | `tools/edit_entry.go` | `workflow.Engine[ConfirmState]` → `TransactionsLedger.UpdateTransaction/UpdateCardPurchase` | `transactions.UpdateTransactionUC` + `UpdateCardPurchaseUC` |
| `delete_entry` | `tools/delete_entry.go` | `workflow.Engine[ConfirmState]` → `TransactionsLedger.DeleteTransaction/DeleteCardPurchase` + `CardManager.SoftDeleteCard` | `transactions.DeleteTransactionUC` + `DeleteCardPurchaseUC` + `card.SoftDeleteCardUC` |
| `update_card` | `tools/update_card.go` | `workflow.Engine[ConfirmState]` → `CardManager.UpdateCard` | `card.UpdateCardUC` |

## Bucket 2 — Parcialmente Implementadas

Nenhuma. Todas as 25 tools têm funcionalidade completa conforme os contratos de suas interfaces.

## Bucket 3 — Não Expostas (use case existe mas sem tool registrada)

Nenhuma. Todos os use cases expostos pelo wiring em `module.go` possuem tool correspondente.

Use cases intencionalmente não expostos como tool do agente:

| Use Case | Motivo |
|----------|--------|
| `budgets.CreateBudgetUC` | Lifecycle de orçamento gerenciado pelo onboarding; não exposto ao agente diretamente |
| `budgets.DeleteDraftBudgetUC` | Lifecycle de orçamento gerenciado pelo onboarding |
| `budgets.ActivateBudgetUC` | Lifecycle de orçamento gerenciado pelo onboarding |
| `budgets.CreateRecurrenceUC` (budgets) | Criação de recorrência orçamentária é responsabilidade do onboarding |
| `card.CreateCardUC` | Criação de cartão é responsabilidade do onboarding |
| `categories.ResolveBySlug` | Utilitário interno do binding; não é capacidade do usuário |

## Resumo de Gaps

| Bucket | Total | Gap Aberto |
|--------|-------|------------|
| 1 — Totalmente implementadas | 25 | 0 |
| 2 — Parcialmente implementadas | 0 | 0 |
| 3 — Não expostas | 0 | 0 |
| **Total de tools registradas** | **25** | **0** |

**Gaps abertos = 0**
