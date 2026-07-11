# Registro de Decisão Arquitetural (ADR)

## Metadados
- **Título:** `UpdatePlan` no repositório + `ChangeTotal`/`ReplaceDistribution` no agregado — editar Draft e Active sem re-stamp de `activated_at`
- **Data:** 2026-07-10
- **Status:** Aceita
- **Decisores:** Plataforma / autor da techspec
- **Relacionados:** PRD (RF-11..RF-16, RF-19..RF-23, D-04, E1, E3); techspec.md; `internal/budgets/domain/entities/budget.go`

## Contexto
Editar um orçamento existente exige persistir novo `total_cents` e/ou nova distribuição **sem** alterar o ciclo de vida do agregado. Hoje `EditCategoryPercentage` persiste via `budgetRepository.Activate` (`infrastructure/repositories/postgres/budget_repository.go:96-132`), que executa `UPDATE ... SET state=$1, total_cents=$2, activated_at=$3` — ou seja, **re-stampa `activated_at` a cada edição** e força `state=Active`. Isso: (a) corrompe `activated_at` em edições de orçamento já ativo; (b) impede editar um Draft sem ativá-lo (D-04 exige editar Draft mantendo Draft). Além disso `Budget.RebalanceAllocations` (`budget.go:116-130`) tem guarda `state == Active`, bloqueando Draft (G-04); e não há operação de domínio para trocar o total preservando basis points (G-01).

## Decisão
1. Adicionar `BudgetRepository.UpdatePlan(ctx, b entities.Budget) error` — `UPDATE mecontrola.budgets SET total_cents=$1, updated_at=$2 WHERE id=$3 AND user_id=$4` (preserva `state` e `activated_at`) + upsert das alocações (reusa `upsertAllocation`); retorna `ErrBudgetNotFound` se 0 linhas.
2. Adicionar dois métodos de domínio no agregado, válidos para **Draft e Active**:
   - `ChangeTotal(newTotalCents int64, recomputed []Allocation, now time.Time) error` — valida `newTotalCents > 0` e soma BP = 10000; substitui total e alocações (planejado recomputado preservando basis points).
   - `ReplaceDistribution(allocs []Allocation, now time.Time) error` — valida soma BP = 10000; substitui alocações preservando total; **sem** guarda `Active` (generaliza `RebalanceAllocations`).
3. Migrar `EditCategoryPercentage` para persistir via `UpdatePlan` (em vez de `Activate`) e usar `ReplaceDistribution`, removendo a guarda `IsActive` — habilitando Draft e eliminando o re-stamp de `activated_at`.
4. Os novos use cases `EditBudgetTotal` e `RedistributeBudget` seguem o padrão UoW (`uow.Do` + `factory.BudgetRepository(tx)`), reutilizando `AllocationDistributor.Distribute` para o planejado.

## Alternativas Consideradas
- **Reutilizar `Activate` para edição:** zero código novo de repositório, mas re-stampa `activated_at` e ativaria Draft. Rejeitada (corrompe ciclo de vida; viola D-04).
- **Método único `Update` que também aceita mudança de estado:** amplo demais; risco de ativar por engano. Rejeitada em favor de `UpdatePlan` estrito (só plano).
- **Manter guarda Active e recriar Draft:** forçaria fluxo de recriação para editar rascunho; diverge da UX pedida. Rejeitada.

## Consequências
### Benefícios Esperados
- Edição não altera `state`/`activated_at` (corrige defeito latente).
- Draft e Active editáveis pelas 3 operações (fecha G-01, G-04).
- Regra de negócio permanece em `Decide*`/serviços puros; repositório só persiste.

### Trade-offs e Custos
- Novo método de repositório + 2 métodos de agregado + migração do use case existente (com teste de regressão).

### Riscos e Mitigações
- R-1 (re-stamp): teste de integração assertando `activated_at` inalterado após edição de Ativo.
- R-3 (afrouxar invariante em Active ao remover guarda): manter invariantes total>0/soma=10000 em ambos os métodos; testes cobrindo Active e Draft; E1 (total < gasto) permitido por design (orçamento é planejamento) e sinalizado, sem violar invariante.

## Plano de Implementação
1. `UpdatePlan` na interface + adapter postgres (+ mock no `.mockery.yml`, `task mocks`).
2. `ChangeTotal`/`ReplaceDistribution` no agregado + testes de domínio.
3. `EditBudgetTotal`/`RedistributeBudget` use cases + DTOs `Validate()` + commands.
4. Migrar `EditCategoryPercentage` para `UpdatePlan`+`ReplaceDistribution`+Draft; ajustar testes.
5. Exportar UCs no `internal/budgets/module.go`.

## Monitoramento e Validação
- Teste de integração: após cada operação, banco reflete `total_cents`/`basis_points`/`planned_cents` novos, `state`/`activated_at` inalterados.
- Soma de `planned_cents` = `total_cents` (half-even), invariante conferida.

## Impacto em Documentação e Operação
- Runbook de budgets: registrar que edição usa `UpdatePlan` (não `Activate`).

## Revisão Futura
- Reavaliar se surgir necessidade de auditar histórico de edições (fora de escopo atual).
