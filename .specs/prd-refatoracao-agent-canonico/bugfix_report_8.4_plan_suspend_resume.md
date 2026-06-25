# Relatorio de Bugfix — Task 8.0 AC 8.4 (plan suspend/resume)

- Total de bugs no escopo: 4
- Corrigidos: 4
- Testes de regressao adicionados: 8 (6 unit no PlanExecutor + 2 integration no decision repo)
- Pendentes: nenhum
- Estado final: done

## Bugs

- ID: BUG-1
- Severidade: critical
- Origem: Task 8.0 AC 8.4 / ADR-004 (suspende o plano inteiro; resume do cursor)
- Estado: fixed
- Causa raiz: o loop de passos em `plan_executor.go` tratava qualquer outcome diferente de
  `OutcomeUsecaseError` como sucesso. Um passo destrutivo abre um confirm side-run e retorna
  `OutcomeClarify` (o prompt de confirmacao); o loop anexava o prompt como reply e avancava o cursor,
  continuando os passos restantes. O plano nunca suspendia e nao havia caminho de resume-from-cursor.
- Arquivos alterados:
  - `internal/agent/application/workflow/plan_executor.go` (suspend no step func; `Resume`; `planCorrelationKey` sem message_id para ser encontravel no resume; `planResultToToolResult`)
  - `internal/agent/application/services/daily_ledger_agent.go` (`continuePendingPlan` em `tryResumeInbound` em ordem deterministica; branch de resume destrutivo em `executePlanStep`)
- Teste de regressao: `TestDestructiveStepSuspendsWholePlanAndResumesFromCursor`
- Validacao: passa; snapshot fica `RunStatusSuspended` no turno 1 e `RunStatusSucceeded` apos resume; ordem `delete_suspend → delete_resume:sim → read`.

- ID: BUG-2
- Severidade: major
- Origem: Task 8.0 / ADR-004 ("condicao de parada = funcao pura, sem LLM")
- Estado: fixed
- Causa raiz: a condicao de parada nao era pura sobre o conjunto fechado `ToolOutcome`; apenas
  `OutcomeUsecaseError` e erro de dispatch eram terminais. `OutcomePolicyBlocked`/`OutcomeAuthzDenied`/
  `OutcomeMissingResolver` no meio do plano avancavam como sucesso.
- Arquivos alterados: `internal/agent/application/workflow/plan_outcome.go` (novo) — `planStepDispositionFor(ToolOutcome) planStepDisposition` pura: `advance` (`Routed`/`Replay`/`Fallback`), `suspend` (`Clarify`), `short-circuit` (`UsecaseError`/`PolicyBlocked`/`AuthzDenied`/`MissingResolver`/`ParseError`/`ReplyFailed`/`EmptyText`).
- Teste de regressao: `TestStopConditionIsPureOverClosedOutcomeSet`, `TestPolicyBlockedMidPlanShortCircuits`, `TestAuthzDeniedMidPlanShortCircuits`
- Validacao: passa; sem IO/LLM na funcao de decisao.

- ID: BUG-3
- Severidade: major
- Origem: Task 8.0 (Testes da Tarefa — suspend→resume; short-circuit mid-plan)
- Estado: fixed
- Causa raiz: ausencia de cobertura para suspend/resume e short-circuit de policy/authz no meio do plano.
- Arquivos alterados: `internal/agent/application/workflow/plan_suspend_resume_test.go` (novo, testify/suite)
- Teste de regressao: suite `PlanSuspendResumeSuite` (suspend+resume-from-cursor, replay nao-duplica e avanca, policy/authz short-circuit, resume sem plano suspenso = not handled, stop-condition pura)
- Validacao: 6 casos passam.

- ID: BUG-4
- Severidade: minor
- Origem: Task 8.6 / ADR-004 (migration 000021 step_index)
- Estado: fixed
- Causa raiz: faltava cobertura de integracao para o indice unico `(user_id,channel,message_id,step_index)`.
- Arquivos alterados: `internal/agent/infrastructure/repositories/postgres/agent_decision_repository_integration_test.go` (`//go:build integration`)
- Teste de regressao: `TestSameMessageDifferentStepIndexCoexist` (mesma mensagem, step_index 0 e 1 coexistem e sao encontraveis e atualizaveis de forma independente), `TestSameMessageSameStepIndexConflicts` (duplicata de mesmo step_index → `ErrAgentDecisionConflict`)
- Validacao: `go vet -tags integration` compila a suite; execucao exige banco (testcontainers).

## Mecanismo (suspend do plano inteiro + resume-from-cursor)

- O `PlanExecutor` roda como `Definition[PlanState]` de step unico (`plan_execute`) com loop interno
  sobre `state.Cursor`. Durabilidade condicional preservada: `Durable=true` so se ha `IsWrite()`.
- `correlationKey` do plano passou de `plan:user:channel:message_id` para `plan:user:channel`, para ser
  encontravel no turno de resume (em que o message_id muda). O message_id original fica em
  `PlanState.MessageID` para auditoria/idempotencia por passo.
- Turno 1: ao atingir um passo destrutivo, o dispatcher (`executePlanStep`→`dispatchWrite`→
  `dispatchWriteDestructive`) abre o confirm side-run (`destructive_confirm`, key `user:channel`) que
  suspende e retorna `OutcomeClarify`. O step func mapeia `Clarify`→`StepStatusSuspended` com o prompt,
  sem avancar o cursor. O engine persiste o snapshot do plano (`RunStatusSuspended`, `PlanState.Cursor`
  no estado — R-WF-KERNEL-001.7). Reply = prompt.
- Turno 2 ("sim"): `tryResumeInbound` em ordem deterministica
  `continuePendingExpenseConfirmation → continuePendingPlan → continuePendingApproval`.
  `continuePendingPlan` resume o plano (merge-patch `{"resume_text":"sim"}`). O step func re-entra no
  `Cursor`; como `ResumeText` esta setado para o cursor suspenso, o dispatcher chama
  `continuePendingApproval` para retomar o confirm side-run (e nao `Start` outro). A operacao
  destrutiva efetiva, retorna `OutcomeRouted`, o cursor avanca e os passos de leitura restantes rodam.
  O plano completa (`RunStatusSucceeded`); housekeeping do kernel purga.
- Composicao: o suspend/resume interno do confirm gate compoe com o suspend do plano — todo o plano
  suspende; ao confirmar, o plano retoma do cursor e segue. Funciona para qualquer passo destrutivo
  (delete-last, edit-last, delete-card, by-ref) por mapa `intentToOperationKind`, sem `case` novo.

## Comandos Executados

- `go build ./internal/agent/...` -> BUILD OK
- `go test ./internal/agent/application/workflow/... ./internal/agent/application/services/...` -> ok (todos)
- `go test ./internal/agent/... -count=1 -race` -> ok (inclui e2e)
- `go vet ./internal/agent/...` -> VET OK
- `go vet -tags integration ./internal/agent/infrastructure/repositories/postgres/...` -> compila
- `gofmt -l <arquivos tocados>` -> vazio (limpo)
- switch-growth gate em `daily_ledger_agent.go` -> cases=0 (<=1, OK)
- zero-comments gate nos `.go` de producao tocados -> OK (zero comentarios)
- LLM-in-plan check -> OK (nenhum simbolo LLM; `uuid.Parse` falso-positivo)
- kernel `internal/platform/workflow` -> intocado

## Riscos Residuais

- O resume do confirm side-run usa key fixa `user:channel`. Se o usuario tiver, simultaneamente, um
  plano suspenso aguardando confirmacao destrutiva E disparar outra operacao destrutiva fora de plano
  no mesmo canal, ha colisao logica de um unico gate por (user,channel) — comportamento intencional do
  modelo single-pending por canal (mesma premissa do HITL existente). Mitigado por TTL/reprompt do
  confirm gate.
- A execucao real do teste de integracao 000021 requer banco (testcontainers); aqui validada apenas a
  compilacao com `-tags integration`. Execucao plena ocorre na esteira de integracao.
- `continuePendingExpenseConfirmation` roda antes de `continuePendingPlan`; ambos usam engines/IDs de
  workflow distintos, sem colisao de snapshot. Ordem deterministica mantida.
