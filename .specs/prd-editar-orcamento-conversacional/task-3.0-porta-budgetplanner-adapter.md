# Tarefa 3.0: Porta `BudgetPlanner` + adapter (agents)

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Estender a porta `BudgetPlanner` (consumida por `internal/agents`) e seu adapter para expor as operações de edição, delegando aos use cases de `internal/budgets`. Adapter fino, sem regra de negócio (R-ADAPTER-001, R-AGENT-WF-001.2).

<requirements>
- Adicionar à porta `internal/agents/application/interfaces/budget_planner.go`:
  - `EditBudgetTotal(ctx, userID uuid.UUID, competence string, newTotalCents int64) error`
  - `RedistributeBudget(ctx, in DraftBudget) error`
- `EditCategoryPercentage` permanece na porta; adapter passa a aceitar Draft (via use case migrado em 2.0).
- Implementar os novos métodos no `infrastructure/binding/budget_planner_adapter.go` mapeando para `EditBudgetTotalUC`/`RedistributeBudgetUC` (via `Deps.BudgetsModule`); mapear erros de fronteira (`ErrBudgetNotFound`) para as sentinelas de `agents/application/interfaces/errors.go`.
- `RedistributeBudget` reusa `DraftBudget`/`AllocationDraft` (o campo `TotalCents` é ignorado pelo use case, que preserva o total atual) — documentar via nomeação clara no adapter; sem lógica de decisão no adapter.
- Atualizar o construtor `NewBudgetPlannerAdapter` e o mock da porta (`.mockery.yml`, `task mocks`).
- Zero comentários; wrapping de erro `fmt.Errorf("ctx: %w", err)`.
</requirements>

## Subtarefas
- [ ] 3.1 Adicionar métodos à interface `BudgetPlanner`.
- [ ] 3.2 Implementar no adapter, injetando os novos use cases via `Deps.BudgetsModule`.
- [ ] 3.3 Mapear `ErrBudgetNotFound` para a sentinela do consumidor.
- [ ] 3.4 Atualizar mock + `task mocks`.
- [ ] 3.5 Testes suite do adapter.

## Detalhes de Implementação
Ver techspec.md "Interfaces Chave". Moldes: `budget_planner.go:9-18`, `budget_planner_adapter.go:17-83` (CreateBudget), `interfaces/errors.go`, `interfaces/types.go:77-110` (`DraftBudget`/`AllocationDraft`/`BudgetSummary`).

## Critérios de Sucesso
- Porta expõe as 2 novas operações; adapter delega sem regra de negócio.
- Erros de fronteira mapeados; `errors.Is` funciona no consumidor.
- `build`/`vet`/`test -race`/`lint` verdes em `internal/agents`.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`. -->

- `mastra` — porta/adapter fino entre o consumidor de agente e o módulo de domínio, seguindo o substrato de plataforma (binding → usecase).

## Testes da Tarefa
- [ ] Testes unitários (suite do adapter; sucesso e mapeamento de erro)
- [ ] Testes de integração (opcional — coberto por 2.0/5.0)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/interfaces/budget_planner.go`
- `internal/agents/infrastructure/binding/budget_planner_adapter.go`
- `internal/agents/application/interfaces/errors.go`
- `.mockery.yml`
