# Tarefa 5.0: Workflow `budget-edit` — step/execute + continuer + reaper

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Implementar o passo do workflow (switch `Operation × Awaiting`, com prefill → confirmação), a execução por operação com **no-false-success**, o continuer (resume por merge-patch + Run auditável) e o reaper dedicado.

<requirements>
- `budget_edit_workflow.go`: `BuildBudgetEditWorkflow`, `BudgetEditWorkflowID = "budget-edit"`, `BudgetEditKey(resourceID) = resourceID + ":budget-edit"` (sem competência — ADR-004), `buildBudgetEditStep` com switch `(Operation, Awaiting)`, helpers `budgetEditSuspend/Complete/Fail/Expire`.
- Passo `AwaitingEditValue` por operação: `EditTotal` coleta total (LLM extrai; ≤0 → reprompt); `AdjustCategory` coleta categoria+% (categoria irreconhecível → clarifica; % 0–100, 0% permitido); `Redistribute` coleta distribuição (3 modos; soma inválida → reprompt). Prefill: se o valor já veio válido no estado inicial, iniciar em `AwaitingEditConfirm`.
- Passo `AwaitingEditConfirm`: resumo do estado novo destacando o que muda (RF-24/R8); `sim` → `executeBudgetEdit`; `não`/cancelar → cancela sem efeito; ambíguo → reprompt único, 2ª vez cancela; expira/replay conforme `Decide*`.
- `executeBudgetEdit` (branch por `Operation`): chama `planner.EditBudgetTotal` | `EditCategoryPercentage` | `RedistributeBudget`. Erro não-de-domínio → `StepStatusFailed` + erro + mensagem específica de indisponibilidade (RF-34/RF-35, sem falso sucesso). `ErrBudgetNotFound` no confirm → completa com mensagem apropriada.
- `budget_edit_continuer.go`: `BudgetEditContinuer` com `Continue(ctx, resourceID, text, messageID)`; patch merge `{"resumeText","incomingMessageId"}`; `engine.Resume`; Run auditável (`thread_id`,`run_id`,`workflow`,`status`,`error`); tratar `RunStatusFailed` com `ResponseText` (entrega msg, fecha Run failed).
- `BuildBudgetEditReaper` (`StaleAfter = 35min`).
- LLM só no step de extração (call-site sancionada); sem LLM no kernel; zero comentários.
</requirements>

## Subtarefas
- [ ] 5.1 `budget_edit_workflow.go` (definição, key, step switch, prefill, prompts).
- [ ] 5.2 `executeBudgetEdit` com branch por operação + no-false-success.
- [ ] 5.3 `budget_edit_continuer.go` (resume + Run auditável).
- [ ] 5.4 `BuildBudgetEditReaper`.
- [ ] 5.5 Testes suite (transições) + integração (workflow ponta a ponta, Draft e Active, replay, expiração, planner indisponível → StepStatusFailed).

## Detalhes de Implementação
Ver techspec.md "Design de Implementação"/"Fluxo de Dados" + ADR-004/ADR-005. Moldes: `budget_creation_workflow.go:21-308`, `budget_creation_continuer.go:19-166`. Correção: key = `resourceID` idêntica em tool e continuer.

## Critérios de Sucesso
- 3 operações aplicam corretamente após confirmação; prefill pula coleta quando valor válido.
- Falha de escrita → `StepStatusFailed` sem recurso e sem falso sucesso; expiração/replay/cancelamento corretos.
- Run auditável emitido; `build`/`vet`/`test -race`/`lint` verdes.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`. -->

- `mastra` — step/continuer/reaper do workflow durável sobre `internal/platform/{workflow,agent}`, LLM só na call-site sancionada, Run auditável.
- `domain-modeling-production` — branch de execução por operação (tipo fechado) e contrato de resume/estado de espera sem string livre.

## Testes da Tarefa
- [ ] Testes unitários (suite: transições, prefill, no-false-success)
- [ ] Testes de integração (`//go:build integration`, testcontainers: 3 operações Draft/Active, replay, expiração, planner indisponível)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/workflows/budget_edit_workflow.go` (novo)
- `internal/agents/application/usecases/budget_edit_continuer.go` (novo)
