# Tarefa 3.0: budgets: EditBudgetTotal e reconciliação de edição

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Adicionar ao módulo `internal/budgets` a edição do total do orçamento ativo (RF-17) e fechar a reconciliação event-driven de edição de lançamento (RF-15) para que o resumo por categoria/geral (RF-25) reflita a mudança. Conforme ADR-004: consumir `transactions.transaction.updated.v1` delegando ao `UpsertExpense` existente.

<requirements>
- RF-17: editar o total do orçamento ativo com allocations reescaladas.
- RF-15: reconciliação de edição de lançamento (valor e categoria) refletida na projeção `budgets_expenses`.
- RF-25: resumo por categoria reflete a edição.
- ADR-004: enriquecer `TransactionUpdated` com subcategoria (via dependência task 1.0) e consumir `updated` no budgets como adapter fino.
- Dependência: task 1.0 (evento `TransactionUpdated` enriquecido com `SubcategoryID`).
</requirements>

## Subtarefas

- [ ] 3.1 Mutador `Budget.ChangeTotal(newTotalCents int64, allocations []Allocation, now time.Time) error` em `internal/budgets/domain/entities/budget.go` com guardas: `IsActive`, `newTotal > 0`, soma dos basis points das allocations `== 10000`.
- [ ] 3.2 Usecase `EditBudgetTotal` em `internal/budgets/application/usecases/edit_budget_total.go` espelhando `edit_category_percentage.go`: DTO `EditBudgetTotalInput{UserID, Competence string; TotalCents int64}` com `Validate()` (reusar `ErrInputInvalidUserID`/`ErrInputInvalidCompetence`/`ErrInputInvalidTotalCents`), command `NewEditBudgetTotalCommand`, fluxo `uow.Do`: `GetByUserCompetence` → guard `IsActive` → `services.AllocationDistributor{}.Distribute(novoTotal, basisPointsAtuais)` → `Budget.ChangeTotal` → persistir via `budgets.Activate`. Sem evento, sem métrica (apenas span + `logFailure`).
- [ ] 3.3 Porta `BudgetPlanner.EditBudgetTotal(ctx, userID, competence, totalCents)` em `internal/agents/application/interfaces/budget_planner.go` + impl em `internal/agents/infrastructure/binding/budget_planner_adapter.go` + UC exposto em `internal/budgets/module.go` + wiring em `internal/agents/module.go`.
- [ ] 3.4 Consumer `transaction_updated_consumer.go` em `internal/budgets/infrastructure/messaging/database/consumers/` para `transactions.transaction.updated.v1` delegando ao `UpsertExpense` (resolve `RootSlug` pela subcategoria; move valor/competência via `existing.Edit`); skip quando sem subcategoria (mirror do consumer de created); idempotência por `event_id`; registrar em `internal/budgets/module.go`.
- [ ] 3.5 Testes unitários (suite canônica) + integração (reconciliação move valor entre categorias raiz; `GetMonthlySummary` reflete).

## Detalhes de Implementação

Ver `techspec.md` (RF-17, RF-15, RF-25) e `adr-004-edit-reconciliation.md` desta pasta — **referenciar em vez de duplicar**.

Pontos-chave do ADR-004:
- Reconciliação aditiva e event-driven; budgets já consome `created.v1` e `deleted.v1`, faltando `updated.v1`.
- Consumer é adapter fino (R-ADAPTER-001): delega ao `UpsertExpense`, sem SQL nem branching de domínio; tolera ausência de subcategoria (skip como no created).
- Idempotência por `event_id` conforme padrão de outbox do repositório.

Guardas do mutador e distribuição de allocations seguem o padrão DMMF do módulo (estado-como-tipo, invariante de soma). `EditBudgetTotal` espelha estruturalmente `edit_category_percentage.go`.

## Critérios de Sucesso

- Novo total persiste com allocations reescaladas somando exatamente o total.
- Editar categoria/valor de um lançamento reflete no `GetMonthlySummary` (resumo por categoria e geral).
- Consumer é adapter fino: delega ao `UpsertExpense`, sem SQL direto nem branching de domínio.
- `EditBudgetTotal` rejeita orçamento inativo/inexistente e total inválido.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `domain-modeling-production` — mutador `ChangeTotal` e invariantes de soma como estado-como-tipo.
- `design-patterns-mandatory` — gate de desenho do usecase `EditBudgetTotal` e do consumer de reconciliação.
- `postgresql-production-standards` — persistência do plano e consumer de reconciliação em postgres.

## Testes da Tarefa

- [ ] Testes unitários (`ChangeTotal`; `EditBudgetTotal` sucesso/inativo/inexistente/infra)
- [ ] Testes de integração (reconciliação move valor entre categorias raiz; `GetMonthlySummary` reflete)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/budgets/domain/entities/budget.go`
- `internal/budgets/application/usecases/edit_budget_total.go`
- `internal/budgets/application/usecases/edit_category_percentage.go` (referência de espelhamento)
- `internal/budgets/infrastructure/messaging/database/consumers/transaction_updated_consumer.go`
- `internal/budgets/module.go`
- `internal/agents/application/interfaces/budget_planner.go`
- `internal/agents/infrastructure/binding/budget_planner_adapter.go`
- `internal/agents/module.go`
