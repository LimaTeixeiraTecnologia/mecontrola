# Relatorio de Bugfix

- Total de bugs no escopo: 1
- Corrigidos: 1
- Testes de regressao adicionados: 3
- Pendentes: nenhum
- Estado final: done

## Bugs

- ID: OCD-BUG-01
- Severidade: major
- Origem: RF-07 (finding de review — paridade do gate anti-falso-sucesso entre workflows de escrita)
- Estado: fixed
- Causa raiz: O workflow `budget-manage` persiste um orcamento novo no caminho `create_retroactive` (`planner.CreateBudget` + `ActivateBudget`) apos confirmacao positiva, mas descartava o `BudgetRef` retornado (`_, createErr := ...`) e nao possuia guarda anti-falso-sucesso nem metrica equivalente. Os demais workflows de escrita (`transaction-write`, `card-manage`) ja emitem `agents_<workflow>_false_success_total` e falham o passo (`StepStatusFailed` + mensagem de falha) quando a confirmacao positiva nao resulta em recurso duravel. Com `CreateBudget` retornando `BudgetRef{ID:""}` e `nil`, o `budget-manage` respondia sucesso ("🎉 Orçamento ... criado e ativado com sucesso!") sem recurso duravel e sem incremento de metrica — invisivel ao alerta `FinancialWriteFalseSuccess` (familia `agents_.+_false_success_total`), violando o objetivo absoluto do PRD ("Zero falso-sucesso ... e equivalentes por workflow permanece em zero").
- Arquivos alterados:
  - `internal/agents/application/workflows/budget_manage_decisions.go` — nova decisao pura `DecideBudgetManagePostWrite(resourceID string)` (retorna `StepStatusFailed` + `ErrBudgetManageAcceptedWithoutResource` quando o id do recurso e vazio), reutilizando o erro compartilhado `ErrWriteAcceptedWithoutResource`.
  - `internal/agents/application/workflows/budget_manage_workflow.go` — metrica `agents_budget_manage_false_success_total` (labels fechados `workflow`/`step`), variante `BuildBudgetManageWorkflowWithObservability`, propagacao de `*budgetManageMetrics` ate `executeBudgetManageCreateRetroactive`, guarda apos create+activate com `messages.WriteFailure()` e `recordBudgetManageFalseSuccessIfNeeded`. O construtor legado `BuildBudgetManageWorkflow` delega com `o11y` nil (metrica nil-safe; a guarda permanece ativa).
  - `internal/agents/module.go` — wiring passa a usar `BuildBudgetManageWorkflowWithObservability(..., deps.O11y)`.
- Teste de regressao:
  - `budget_manage_decisions_test.go::TestDecideBudgetManagePostWrite` — id vazio/branco → `StepStatusFailed` + erro; id valido → `StepStatusCompleted`.
  - `budget_manage_workflow_test.go::TestCreateRetroactiveConfirmFalseSuccessOnEmptyRef` — reproduz o bug: `CreateBudget` retorna `BudgetRef{ID:""}`, `nil` → passo falha, `errors.Is(err, ErrBudgetManageAcceptedWithoutResource)`, status != `BudgetManageCompleted`, sem mensagem de sucesso.
  - `budget_manage_workflow_test.go::TestCreateRetroactiveConfirmPersistsAndSucceeds` — happy path preservado: id valido → `StepStatusCompleted` + mensagem de sucesso.
- Validacao: `go build ./internal/agents/...` OK; `go test ./internal/agents/... ./internal/transactions/... ./internal/budgets/...` OK (39 pacotes, 0 FAIL); `go vet ./internal/agents/...` OK; gate zero-comentarios OK nos arquivos de producao editados.

## Comandos Executados

- `go build ./internal/agents/... ./internal/transactions/... ./internal/budgets/...` -> exit 0
- `go test ./internal/agents/... ./internal/transactions/... ./internal/budgets/...` -> 39 pacotes ok, 0 FAIL
- `go vet ./internal/agents/...` -> exit 0
- `grep -rn "^[[:space:]]*//" <arquivos-editados> | grep -Ev "(//go:|//nolint:|// Code generated)"` -> vazio (zero comentarios)
- `grep '"agents_[a-z_]*_false_success_total"'` -> 3 metricas (transaction_write, card_manage, budget_manage), todas casam `agents_.+_false_success_total`

## Riscos Residuais

- Caminhos de EDICAO (`card-manage` edit, `budget-manage` edit_total/edit_distribution) nao emitem metrica de falso-sucesso: para edicao o recurso ja existe (id nao-nulo) e os usecases propagam erro diretamente (nil error = uow commit efetivado), portanto o falso-sucesso classe RF-07 ("recurso persistido") nao e representavel ali; um update nil-error/zero-rows seria uma preocupacao de completude de idempotencia, fora do escopo do gate RF-07 e do padrao estabelecido. Nao bloqueante.
</content>
</invoke>
