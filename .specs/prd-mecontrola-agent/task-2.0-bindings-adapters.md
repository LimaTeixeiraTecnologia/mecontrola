# Tarefa 2.0: Interfaces consumer-side + adapters de binding (categories/card/budgets/transactions)

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Definir as interfaces consumer-side em `internal/agents/application/interfaces` (`TransactionsLedger`, `BudgetPlanner`, `CardManager`, `CategoriesReader`) e implementar os adapters em `internal/agents/infrastructure/binding` que chamam os use cases reais dos módulos de domínio. Espelha o padrão existente (`budgets/.../categories_reader_adapter.go`, `transactions/.../card_lookup_adapter.go`).

<requirements>
- ADR-003: fronteira de ferramentas + binding consumer-side (interface no consumidor, adapter no consumidor).
- Escrita só via `transactions`; `budgets` só planejamento; categorias via dicionário.
- Cobre: RF-31, RF-32, RF-33, RF-34, RF-35, RF-36.
</requirements>

## Subtarefas

- [ ] 2.1 Interface `TransactionsLedger` + adapter chamando `CreateTransaction`, `CreateCardPurchase`, `UpdateTransaction`, `DeleteTransaction`, `UpdateCardPurchase`, `DeleteCardPurchase`, `ListMonthlyEntries`, `GetMonthlySummary`.
- [ ] 2.2 Interface `BudgetPlanner` + adapter (`CreateBudget`, `ActivateBudget`, `CreateRecurrence`, `EditCategoryPercentage`, `GetMonthlySummary`, `ListAlerts`). **Nunca** expor `UpsertExpense` (anti-dupla-contagem, D-13).
- [ ] 2.3 Interface `CardManager` + adapter (`CreateCard`, `ListCards`, `SoftDeleteCard`, `HasOpenInstallments`).
- [ ] 2.4 Interface `CategoriesReader` + adapter (`SearchDictionary`, `ResolveRootsBySlug`).
- [ ] 2.5 Tipos de fronteira tipados (`EntryRef`, `MonthlyEntry`, `BudgetSummary`, `CategoryCandidate`, etc.) e mapeamento DTO↔use case.

## Detalhes de Implementação

Ver techspec.md → "Interfaces Chave" e ADR-003. Adapters são finos (R-ADAPTER-001.2): mapeiam DTO, invocam UC, mapeiam saída, fazem wrapping `%w`; sem regra/SQL/branching de domínio.

## Critérios de Sucesso

- Interfaces definidas no consumidor (R6); adapters em `internal/agents/infrastructure/binding`.
- Zero comentários em `.go` de produção; sem SQL direto (R-ADAPTER-001); `Validate()` em DTOs de input após span quando aplicável (R-DTO-VALIDATE-001).
- DMMF: tipos de fronteira fechados onde houver estado; sem `Result[T,E]`/currying/DSL.
- `BudgetPlanner` não expõe escrita de despesa (fronteira D-13 preservada).
- Build/gofmt verdes; testes unitários dos adapters.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — define as portas de consumo dos módulos pelo agente (binding consumer-side), base do roteamento Tool→binding→usecase.

## Testes da Tarefa

- [ ] Testes unitários (testify/suite whitebox, IIFE por mock dos UCs): mapeamento DTO↔UC, wrapping de erro, sucesso e erro de binding.
- [ ] Testes de integração: adapter chama o UC real e mapeia a resposta (quando proporcional ao risco).

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/interfaces/` (novas interfaces)
- `internal/agents/infrastructure/binding/` (novos adapters)
- Referência de padrão: `internal/budgets/infrastructure/repositories/postgres/categories_reader_adapter.go`, `internal/transactions/infrastructure/http/client/card_lookup_adapter.go`
- techspec.md (Interfaces Chave), ADR-003
