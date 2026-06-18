# Run: Execute docs/melhorias/2026-06-17-transactions.md

**Data:** 2026-06-17
**Skill:** go-implementation

## Contexto

Capacitou o `internal/agent` com as operações completas do módulo `internal/transactions` nos dois caminhos do agente:

- **Intent Router** (determinístico): já estava completo com card purchases e recurring templates.
- **LLM Dispatcher** (openrouter): não tinha suporte a `create_card_purchase`, `create_recurring` e `list_recurring`. Implementado neste run.

## Arquivos alterados

| Arquivo | Tipo de mudança |
|---|---|
| `internal/agent/application/prompting/persona.system.tmpl` | Enriqueceu item 5 (Lançamentos) com compras parceladas e recorrentes |
| `internal/agent/application/services/prompt_builder.go` | Adicionou boundary e schemas para `create_card_purchase`, `create_recurring`, `list_recurring` |
| `internal/agent/application/interfaces/module_ports.go` | Adicionou `CardPurchasesCreatePort`, `RecurringCreatePort`, `RecurringListPort` |
| `internal/agent/infrastructure/dispatcher/intent_dispatcher.go` | Adicionou 3 cases em `routeTransactions` |
| `internal/agent/infrastructure/dispatcher/transactions_adapter.go` | Adicionou 3 interfaces, 3 campos, atualização do construtor e 3 métodos |
| `internal/agent/domain/valueobjects/intent_action.go` | Adicionou `IntentActionCreateCardPurchase`, `IntentActionCreateRecurring`, `IntentActionListRecurring` |
| `internal/agent/module.go` | Atualização do wiring `NewTransactionsAdapterFull` + 3 novos ports |
| `internal/agent/application/prompting/persona_test.go` | Assertions para "parcelada" e "recorrentes" |
| `internal/agent/infrastructure/dispatcher/intent_dispatcher_test.go` | 3 testes nil-port para os novos routes |
| `internal/agent/infrastructure/dispatcher/transactions_create_adapter_test.go` | Atualização das chamadas `NewTransactionsAdapterFull` |
| `internal/agent/infrastructure/dispatcher/source_regression_test.go` | Atualização das chamadas `NewTransactionsAdapterFull` |

## Validação executada

```
go build ./internal/agent/...       → OK
go vet ./internal/agent/...         → OK
go test -race -count=1 ./internal/agent/...   → 14 pacotes PASS
gate R-ADAPTER-001.1 (zero comentários)  → OK
gate R-ADAPTER-001.2 (sem SQL em adapters) → OK
```

## Regras aplicadas

- R-ADAPTER-001: zero comentários, adaptadores finos handler→usecase.
- R6.4: sem `var _ Interface = (*Type)(nil)`.
- R5.12: sem `panic`.
- R6.7: `time.Now().UTC()` inline para `StartedAt`/`PurchasedAt` quando ausente.
- Erros com `fmt.Errorf("ctx: %w", ErrTransactionsCreateInvalidPayload)`.
- `auth.WithPrincipal` injetado em todos os métodos do adapter seguindo padrão existente.
