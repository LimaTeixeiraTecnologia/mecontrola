# Tarefa 1.0: Domínio `budgets` — `ChangeTotal`/`ReplaceDistribution` + repositório `UpdatePlan`

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Adicionar ao agregado `Budget` os métodos de domínio que editam total e distribuição de forma válida em **Draft e Active**, e um novo método de repositório `UpdatePlan` que persiste plano (total + alocações) **sem alterar `state`/`activated_at`**. Base pura das 3 operações de edição (ADR-003).

<requirements>
- `Budget.ChangeTotal(newTotalCents int64, recomputed []Allocation, now time.Time) error`: valida `newTotalCents > 0` e soma BP = 10000; substitui total e alocações; aceita Draft e Active; atualiza `updatedAt`. Não altera `state`/`activatedAt`.
- `Budget.ReplaceDistribution(allocs []Allocation, now time.Time) error`: valida soma BP = 10000; substitui alocações preservando total; aceita Draft e Active (sem guarda `Active`); atualiza `updatedAt`.
- `BudgetRepository.UpdatePlan(ctx, b entities.Budget) error`: `UPDATE mecontrola.budgets SET total_cents=$1, updated_at=$2 WHERE id=$3 AND user_id=$4` (preserva `state`/`activated_at`) + upsert das alocações (reusar `upsertAllocation`); retorna `interfaces.ErrBudgetNotFound` se 0 linhas.
- Adicionar `UpdatePlan` à interface `BudgetRepository`, ao adapter postgres e ao mock do `.mockery.yml` (`task mocks`).
- Invariantes reusam as sentinelas existentes (`ErrBudgetTotalMustBePositive`, `ErrBudgetAllocationSumMustBe10000`).
- Zero comentários em `.go` de produção (R-ADAPTER-001.1).
</requirements>

## Subtarefas
- [ ] 1.1 Implementar `ChangeTotal` e `ReplaceDistribution` em `domain/entities/budget.go` (generalizando o guard de `RebalanceAllocations`, que permanece para compatibilidade se ainda usado).
- [ ] 1.2 Adicionar `UpdatePlan` à interface `application/interfaces/budget_repository.go`.
- [ ] 1.3 Implementar `UpdatePlan` no adapter `infrastructure/repositories/postgres/budget_repository.go` (reusar `upsertAllocation`; preservar `state`/`activated_at`).
- [ ] 1.4 Registrar `UpdatePlan` no `.mockery.yml` e rodar `task mocks`.
- [ ] 1.5 Testes de domínio (unit) e de repositório (integração Postgres).

## Detalhes de Implementação
Ver techspec.md seções "Interfaces Chave", "Modelos de Dados" e ADR-003. Moldes: `budget.go:116-150` (`RebalanceAllocations`/`Activate`), `budget_repository.go:96-132` (`Activate`), `upsertAllocation` (253-267).

## Critérios de Sucesso
- `ChangeTotal`/`ReplaceDistribution` aceitam Draft e Active e rejeitam soma ≠ 10000 e total ≤ 0.
- `UpdatePlan` atualiza `total_cents`/`basis_points`/`planned_cents` e **não** altera `state`/`activated_at` (verificado em teste de integração).
- `build`, `vet`, `test -race`, `lint` verdes no módulo `internal/budgets`.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`. -->

- `domain-modeling-production` — novos métodos de agregado com invariantes e state-as-type (DMMF), edição válida em Draft e Active.
- `postgresql-production-standards` — novo método de persistência `UpdatePlan` (UPDATE + upsert) preservando colunas de ciclo de vida.

## Testes da Tarefa
- [ ] Testes unitários (domínio: invariantes, Draft e Active, 0%, total<gasto permitido)
- [ ] Testes de integração (`UpdatePlan` preserva `state`/`activated_at`; soma `planned_cents` = `total_cents`)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/budgets/domain/entities/budget.go`
- `internal/budgets/application/interfaces/budget_repository.go`
- `internal/budgets/infrastructure/repositories/postgres/budget_repository.go`
- `.mockery.yml`
