# Registro de Decisão Arquitetural (ADR-003)

## Metadados

- **Título:** Fronteira de ferramentas + binding consumer-side + exposição de use cases faltantes
- **Data:** 2026-06-30
- **Status:** Aceita
- **Decisores:** Time de plataforma
- **Relacionados:** PRD (RF-25.2/RF-31..RF-36, D-12/D-13), techspec.md; `.claude/rules/go-adapters.md`; `.claude/rules/agent-workflows-tools.md`

## Contexto

As ferramentas do agente devem derivar exclusivamente dos módulos reais (RF-31), com fronteira clara para evitar dupla contagem: lançamentos só via `internal/transactions`; o orçamento (resumo/alertas) é atualizado automaticamente pelos consumers de outbox em `internal/budgets` (`budgets/module.go:134-149`, populando `budgets_expenses` com unique constraint + idempotência) (D-13). O padrão do repositório é **interface no consumidor + adapter no consumidor + wiring em `cmd/server`** (ex.: `budgets/.../categories_reader_adapter.go`; `transactions/.../card_lookup_adapter.go`). O `TransactionsModule` constrói mas não expõe `UpdateCardPurchase`, `DeleteCardPurchase`, `GetMonthlySummary`, `ListMonthlyEntries` (`internal/transactions/module.go:165-248`).

## Decisão

1. **Fronteira de escrita**: tools `register_expense`/`register_income`/`register_card_purchase`/`edit_entry`/`delete_entry` chamam **apenas** use cases de `internal/transactions`. O agente **nunca** chama `budgets.UpsertExpense`. Tools de planejamento (`create budget`, `activate`, `recurrence`, `adjust_allocation`) chamam `internal/budgets`. Inferência de categoria via `categories` dicionário (D-12).
2. **Binding consumer-side**: definir interfaces em `internal/agents/application/interfaces` (`TransactionsLedger`, `BudgetPlanner`, `CardManager`, `CategoriesReader`) e adapters em `internal/agents/infrastructure/binding`, chamando os use cases públicos dos módulos. Tools permanecem finas (R-AGENT-WF-001.2): validam input, mapeiam DTO, invocam binding, mapeiam saída, fazem wrapping de erro; zero regra/SQL/branching.
3. **Exposição faltante**: adicionar campos públicos ao `TransactionsModule` para `UpdateCardPurchase`, `DeleteCardPurchase`, `GetMonthlySummary`, `ListMonthlyEntries` (use cases já construídos internamente).
4. **Wiring**: `agents.Deps` recebe os módulos `categories/card/budgets/transactions` já construídos; ordem em `cmd/server` `categories → card → budgets → transactions → agents` (já vigente).

## Alternativas Consideradas

- **Tools chamando repositórios/SQL diretamente** — viola R-ADAPTER-001.2. Rejeitada.
- **Agente registrando despesa também em budgets** — causa dupla contagem (consumer já popula `budgets_expenses`). Rejeitada (D-13).
- **Reutilizar uma interface CategoriesReader global** — o repo define interfaces por consumidor (budgets e transactions têm as suas). Mantemos a convenção; agents define a sua. Rejeitada a global.

## Consequências

### Benefícios Esperados

- Zero dupla contagem; tools finas e testáveis; aderência ao padrão existente.

### Trade-offs e Custos

- Boilerplate de interfaces/adapters por módulo; mudança pública (mínima) no `TransactionsModule`.

### Riscos e Mitigações

- **Tool engordar com regra** → revisão R-AGENT-WF-001.2 + gates (sem SQL/branching).
- **Exposição quebrar encapsulamento** → expor apenas os UCs necessários, já públicos em padrão análogo (card/budgets).

## Plano de Implementação

1. Expor UCs faltantes no `TransactionsModule`.
2. Interfaces + adapters de binding (com testes).
3. Tools finas usando os bindings.
4. Ajustar `Deps`/wiring.

## Monitoramento e Validação

- `agents_write_total{operation,outcome}`; teste de integração provando lançamento→`budgets_expenses`→`GetMonthlySummary` sem duplicar.

## Impacto em Documentação e Operação

- Documentar a matriz tool→módulo→use case no runbook.

## Revisão Futura

- Revisar se novos módulos forem consumidos ou se surgir necessidade de transação cross-módulo (hoje proibida).
