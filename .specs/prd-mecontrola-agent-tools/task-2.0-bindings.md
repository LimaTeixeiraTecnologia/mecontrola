# Tarefa 2.0: Binding adapters + wiring dos use cases nos módulos

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Implementar os binding adapters finos que ligam os contratos da Tarefa 1.0 aos use cases reais dos módulos, e fazer o wiring desses use cases em `internal/agents/module.go`. Cada método é um adapter fino (abre span, mapeia args→DTO, `Execute`, mapeia retorno→tipo agent-owned, faz wrapping de erro), sem regra de negócio. Ver techspec seções "Design de Implementação → Interfaces Chave", "Pontos de Integração" e "Sequenciamento de Desenvolvimento → Ordem de Build" (passo 2). Depende da Tarefa 1.0.

<requirements>
- RF-02: nenhuma capacidade mapeada sem use case real por trás.
- RF-05: distinção bound vs não-bound resolvida no wiring (só injetar use cases existentes).
- RF-19: cada método delega a um único use case, sem branching de domínio.
- Estender adapters `card_manager_adapter.go`, `transactions_ledger_adapter.go`, `budget_planner_adapter.go` com campos/construtores/métodos novos; criar `recurrence_manager_adapter.go`.
- Idioma verificado por método: span `agents.binding.<x>.<op>`, mapear args→DTO de input, `Execute`, mapear retorno→tipo agent-owned, `fmt.Errorf("agents/binding/<x>: <ação>: %w", err)`.
- R-ADAPTER-001.2: zero regra de negócio/SQL/branching de domínio nos adapters.
- R-ADAPTER-001.1: zero comentários em Go de produção.
- Wiring em `module.go` dos use cases reais (card: `GetCard`, `CountCards`, `BestPurchaseDay`, `UpdateCard`; transactions: `GetCardInvoice`, `SearchTransactions`, `GetTransaction`, `GetCardPurchase`, `ListCardPurchases`, `Create/Update/Delete/List RecurringTemplate`; budgets: `SuggestAllocation`).
</requirements>

## Subtarefas

- [ ] 2.1 Estender os 3 adapters (`card_manager_adapter.go`, `transactions_ledger_adapter.go`, `budget_planner_adapter.go`) com os campos, construtores e métodos novos seguindo o idioma verificado.
- [ ] 2.2 Criar `recurrence_manager_adapter.go` implementando `RecurrenceManager` sobre os use cases de recurring template.
- [ ] 2.3 Wiring em `internal/agents/module.go`: injetar os use cases reais nos adapters e construir a nova `RecurrenceManager`.

## Detalhes de Implementação

Ver techspec.md seção "Design de Implementação → Interfaces Chave" (idioma do adapter: `card_manager_adapter.go:42-70` como referência do padrão span→map→Execute→map→wrap), seção "Pontos de Integração" (consumo dos módulos exclusivamente via binding→usecase, sem SQL direto nem tx compartilhada) e "Sequenciamento de Desenvolvimento → Ordem de Build" passo 2. Os use cases consumidos são os já existentes listados em "Arquivos Relevantes e Dependentes" (`internal/card/...`, `internal/transactions/...`, `internal/budgets/application/usecases/suggest_allocation.go`) — não criar capacidade de domínio nova. `update_card` sem alteração de dia de vencimento executa direto via `CardManager.UpdateCard`; o gate condicional é decidido na camada de tool/workflow (Tarefas seguintes), não aqui.

## Critérios de Sucesso

- Adapters compilam e implementam integralmente as interfaces da Tarefa 1.0.
- Testes unitários de mapeamento args↔DTO por adapter passam com use case mockado.
- Nenhuma capacidade mapeada sem use case real (RF-02).
- Distinção bound vs não-bound resolvida no wiring (RF-05).
- Zero regra de negócio/SQL/branching nos adapters (R-ADAPTER-001.2); zero comentários (R-ADAPTER-001.1).

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — port do agente sobre o substrato internal/platform; criar/estender tools, interfaces de consumidor e bindings segue o molde internal/agents.

## Testes da Tarefa

- [ ] Testes unitários — mapeamento args↔DTO por adapter com use case mockado (mockery), cobrindo sucesso e propagação de erro com wrapping.
- [ ] Testes de integração — N/A nesta tarefa (wiring end-to-end coberto nas Tarefas 6.0/7.0).

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/infrastructure/binding/card_manager_adapter.go`
- `internal/agents/infrastructure/binding/transactions_ledger_adapter.go`
- `internal/agents/infrastructure/binding/budget_planner_adapter.go`
- `internal/agents/infrastructure/binding/recurrence_manager_adapter.go` (novo)
- `internal/agents/module.go`
