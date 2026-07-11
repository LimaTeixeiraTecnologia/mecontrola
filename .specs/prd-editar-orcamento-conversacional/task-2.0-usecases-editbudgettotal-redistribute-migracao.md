# Tarefa 2.0: Use cases de edição em `budgets` + migração `EditCategoryPercentage` + exports

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Criar os use cases `EditBudgetTotal` e `RedistributeBudget` (com DTOs `Validate()` e commands), migrar `EditCategoryPercentage` para persistir via `UpdatePlan` e aceitar Draft, e exportar os novos use cases no `internal/budgets/module.go`.

<requirements>
- `EditBudgetTotal.Execute(ctx, input)`: valida input; resolve budget por competência; recomputa planejado preservando BP via `AllocationDistributor.Distribute(newTotal, BP_atuais)`; aplica `Budget.ChangeTotal`; persiste via `repo.UpdatePlan`. Aceita Draft e Active. Permite total < gasto (E1) sem bloquear.
- `RedistributeBudget.Execute(ctx, input)`: valida input (5 categorias, soma 10000 via command/VO); recomputa planejado via `Distribute(total_atual, BP)`; aplica `Budget.ReplaceDistribution`; persiste via `repo.UpdatePlan`. Aceita Draft e Active.
- Migrar `EditCategoryPercentage` (persist em `edit_category_percentage.go:71-118`): persistir via `repo.UpdatePlan` (não `repo.Activate`); usar `Budget.ReplaceDistribution`. Remover a guarda `Active` nos **dois** sítios: (a) `if !budget.IsActive() { return ErrBudgetNotActive }` em `edit_category_percentage.go:78-80`; (b) a dependência de `Budget.RebalanceAllocations` (guard Active em `budget.go:116-119`), substituída por `ReplaceDistribution` (sem guard). Podar a classificação morta `entities.ErrBudgetNotActive` em `edit_category_percentage.go:55-61`. Preservar `DecideEditCategoryPercentage` + rebalanceamento proporcional; permitir 0% (E3/RF-19).
- Input DTOs em `application/dtos/input/` com `Validate()` (R-DTO-VALIDATE-001, `errors.Join`, campos nomeados).
- Commands (smart constructors) em `domain/commands/`.
- Exportar `EditBudgetTotalUC`, `RedistributeBudgetUC` na struct `BudgetsModule` + `buildUseCases()` + `Build()`.
- Padrão UoW (`uow.Do` + `factory.BudgetRepository(tx)`); testes testify/suite table-driven (R-TESTING-001).
</requirements>

## Subtarefas
- [ ] 2.1 Commands + DTOs `Validate()` para editar total e redistribuir.
- [ ] 2.2 Use case `EditBudgetTotal` (preserva BP).
- [ ] 2.3 Use case `RedistributeBudget` (preserva total).
- [ ] 2.4 Migrar `EditCategoryPercentage` → `UpdatePlan` + `ReplaceDistribution` + Draft (remover guard `IsActive` em `:78-80` e a dependência de `RebalanceAllocations`; podar classificação `ErrBudgetNotActive` em `:55-61`).
- [ ] 2.5 Exportar UCs em `module.go`.
- [ ] 2.6 Testes suite + integração (assert `activated_at` inalterado após edição de Ativo).

## Detalhes de Implementação
Ver techspec.md "Design de Implementação" + ADR-003. Moldes: `edit_category_percentage.go` (persist), `create_budget.go`/`activate_budget.go` (UoW), `allocation_distributor.go:20-75`, `category_percentage_workflow.go:21-74`, `module.go` (exports 40-61, buildUseCases 249-256).

## Critérios de Sucesso
- As 3 operações persistem plano correto (BP/planejado) em Draft e Active sem alterar `state`/`activated_at`.
- `EditCategoryPercentage` não re-stampa `activated_at` (teste de regressão).
- `build`/`vet`/`test -race`/`lint` verdes em `internal/budgets`.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`. -->

- `domain-modeling-production` — smart constructors, DTOs com `Validate()`, `Decide*` puros e composição de invariantes de domínio nas 3 operações.

## Testes da Tarefa
- [ ] Testes unitários (suite table-driven por use case; sucesso, validação, erro de infra; Draft e Active; 0%; total<gasto)
- [ ] Testes de integração (persistência + `activated_at` inalterado)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/budgets/application/usecases/edit_budget_total.go` (novo), `redistribute_budget.go` (novo), `edit_category_percentage.go` (migrar)
- `internal/budgets/application/dtos/input/edit_budget_total.go`, `redistribute_budget.go` (novos)
- `internal/budgets/domain/commands/edit_budget_total.go`, `redistribute_budget.go` (novos)
- `internal/budgets/module.go`
