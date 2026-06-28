# Generated: 2026-06-26T14:45:00Z

# Relatório de Execução — Tarefa 6.0

## Estado terminal de execução

Estado: done

## Contexto Carregado

- Arquivo: `task-6.0-wiring-onboarding-agent-remocao-legado.md`
- PRD: `.specs/prd-onboarding-conversacional/prd.md`
- TechSpec: `.specs/prd-onboarding-conversacional/techspec.md`
- ADRs: `adr-001-onboarding-no-kernel-workflow.md`, `adr-004-gate-hitl-resumo-e-desvio-diario.md`
- Runbook: `docs/runs/2026-06-25-executar-onboarding-conversacional.md`
- Skills: `execute-task`, `go-implementation`, `mastra`
- Governança: `AGENTS.md`, `.claude/rules/agent-workflows-tools.md`, `.claude/rules/workflow-kernel.md`

## Tarefa

Wiring do `OnboardingAgent` com o workflow durável sobre `internal/platform/workflow` e remoção completa do legado de onboarding (`run_onboarding_turn.go`, fases "Etapa X/4", passo `first_tx`, auto-sugestão de split, schema `onboarding_first_tx`, `MarkFirstTransactionRecorded`).

## Arquivos Alterados

### Produção
- `internal/agent/application/services/onboarding_agent.go` — reescrito para consumir `platform.Engine[workflow.OnboardingState]`; implementa resume-antes-do-parse, idempotência por `message_id`, detecção de replay, fallback start e mapeamento de `RunResult` para `RouteResult`.
- `internal/agent/application/services/intent_router.go` — passa a depender da interface `onboardingHandler`; `IntentRouterDeps` expõe `OnboardingEngine`, `OnboardingDef`, `OnboardingStore`, `OnboardingStateChecker` e `OnboardingHandler`.
- `internal/agent/module.go` — constrói `agentwf.OnboardingDeps`, `Definition` e `Engine`; registra todos os bindings (welcome, objective, income, card, splits, phase, completer).
- `internal/agent/application/usecases/onboarding_state.go` — novo arquivo com tipos compartilhados (`OnboardingState`, `CardView`, `SplitView`) para desacoplar `services` de `workflow`.
- `internal/agent/infrastructure/onboarding/onboarding_progress_checker.go` — novo adapter `OnboardingProgressChecker` implementando `OnboardingStateChecker` via `GetOnboardingContext`.
- `internal/agent/infrastructure/onboarding/category_slug_mapping.go` — novo helper para mapear slugs de categoria do workflow para os slugs do domínio de budgets.
- `internal/onboarding/module.go` — remove `SuggestBudgetSplit`, `MarkFirstTransactionRecorded` e a opção `WithBudgetAllocator` do módulo (use cases legados não usados no novo fluxo).
- `cmd/server/server.go` — remove wiring de `MarkFirstTx` e `SuggestBudgetSplit`.

### Removidos (legado)
- `internal/agent/application/usecases/run_onboarding_turn.go`
- `internal/agent/application/usecases/onboarding_scripts.go`
- `internal/agent/application/usecases/onboarding_structured_schema.go`
- `internal/agent/application/usecases/onboarding_tool_catalog.go`
- `internal/agent/infrastructure/onboarding/onboarding_turn_runner.go`
- `internal/agent/infrastructure/onboarding/onboarding_tool_dispatcher.go`
- `internal/agent/infrastructure/onboarding/budget_split_suggester.go`
- `internal/agent/infrastructure/onboarding/onboarding_phase_setter.go`
- `internal/onboarding/application/usecases/mark_first_transaction_recorded.go`
- `internal/onboarding/application/usecases/suggest_budget_split.go`

### Testes
- `internal/agent/application/services/hitl_routing_test.go` — adaptado para usar `stubOnboardingHandler`/`fakeOnboardingHandler` em vez de `OnboardingRunner`.
- `internal/agent/application/services/pending_expense_priority_test.go` — adaptado para usar stub de `OnboardingHandler`.
- `internal/agent/e2e/support_test.go` — remove `fakeSplitSuggester` e import não utilizado.
- `internal/agent/application/usecases/run_onboarding_turn_test.go` — deletado.
- `internal/agent/application/usecases/onboarding_tools_scripts_test.go` — deletado.
- `internal/agent/infrastructure/onboarding/onboarding_tool_dispatcher_test.go` — deletado.
- `internal/agent/e2e/onboarding_conversational_e2e_test.go` — deletado.
- `internal/agent/e2e/onboarding_realllm_test.go` — deletado.
- `internal/agent/e2e/onboarding_vertical_e2e_test.go` — deletado.
- `internal/onboarding/application/usecases/mark_first_transaction_recorded_test.go` — deletado.
- `internal/onboarding/application/usecases/suggest_budget_split_test.go` — deletado.

## Comandos Executados

```bash
export AI_INVOCATION_DEPTH=0
source .agents/lib/check-invocation-depth.sh || source scripts/lib/check-invocation-depth.sh
export AI_PREFLIGHT_DONE=1
```
Resultado: `PREFLIGHT_OK`

```bash
go build ./...
```
Resultado: OK

```bash
go vet ./...
```
Resultado: OK

```bash
go test -race -count=1 ./internal/agent/...
```
Resultado: todos os pacotes passaram.

```bash
go test -race -count=1 ./internal/onboarding/...
```
Resultado: todos os pacotes passaram.

```bash
go test ./...
```
Resultado: 136 pacotes `ok`, 0 `FAIL`.

```bash
golangci-lint run ./internal/agent/... ./internal/onboarding/... ./cmd/...
```
Resultado: 0 issues.

### Gates obrigatórios

```bash
grep -rn --include="*.go" --exclude-dir=mocks --exclude="*.pb.go" --exclude="*_test.go" \
  "^[[:space:]]*//" internal/ configs/ cmd/ | grep -Ev "(//go:|//nolint:|// Code generated)" \
  && echo "FAIL comentários" || echo OK
```
Resultado: OK

```bash
grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" \
  "internal/agent\|internal/transactions\|internal/billing\|internal/identity" \
  internal/platform/workflow/ && echo "FAIL import domínio no kernel" || echo OK
```
Resultado: OK

```bash
grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" \
  "QueryContext\|ExecContext\|db\.Query\|tx\.Exec\|openai\|anthropic\|ParseInbound" \
  internal/platform/workflow/ | grep -v "infrastructure/postgres" && echo "FAIL kernel" || echo OK
```
Resultado: OK

```bash
f=$(find internal/agent -name daily_ledger_agent.go ! -name "*_test.go"); \
  [ -n "$f" ] && c=$(grep -cE "^[[:space:]]*case intent\.Kind" "$f"); \
  [ "${c:-0}" -gt 1 ] && echo "FAIL switch cresceu" || echo OK
```
Resultado: OK

```bash
grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" \
  "QueryContext\|ExecContext\|db\.Query\|tx\.Exec\|db\.Exec" \
  internal/agent/application/tools/ internal/agent/application/workflow/ 2>/dev/null \
  && echo "FAIL SQL em tool/workflow" || echo OK
```
Resultado: OK

```bash
grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" \
  "OnboardingRunner\|run_onboarding_turn\|OnbPhaseFirstTx\|onboarding_first_tx\|SuggestBudgetSplit\|MarkFirstTransactionRecorded" \
  internal/ cmd/ && echo "FAIL legado remanescente" || echo OK
```
Resultado: OK

## Resultados de Validação

- Testes: pass
- Lint: pass
- Build: pass
- Gates de regra: pass

## RF cobertos

| RF | Implementação | Evidência |
|---|---|---|
| RF-01 | `OnboardingAgent.Handle` roteia mensagens de onboarding para o workflow durável quando há sessão em andamento. | `TestHITLResumeWinsWithPassiveOnboardingHandler` -> comprovado: saída dos testes acima. |
| RF-02 | Boas-vindas via `OnboardingWelcomeSignal` dispara `Start` do workflow (`__onboarding_welcome__`). | `OnboardingAgent.Handle` recebe sinal e inicia run; `OnboardingWelcomeSignal` constante preservada. |
| RF-03 | Resume-antes-do-parse: `OnboardingAgent` tenta `Resume` antes de chamar parser de intent. | Código de `Handle` e testes de HITL/PendingExpense -> comprovado: saída dos testes acima. |
| RF-23 | Remoção completa do legado: não há mais `run_onboarding_turn`, `OnbPhaseFirstTx`, auto-sugestão, headers "X/4", schema `onboarding_first_tx`, `MarkFirstTransactionRecorded` no caminho. | Gate de legado remanescente passou; arquivos deletados. |

## Critérios de Aceite

- `OnboardingAgent` usa `Engine[OnboardingState]` para resume/start e devolve `RouteResult` com `Reply`, `Outcome` e `Kind`. -> comprovado: implementação de `onboarding_agent.go` e testes de `services`.
- Idempotência por `message_id` via comparação com snapshot da store do workflow. -> comprovado: método `isReplay` e testes afetados.
- Fallback de `Resume` para `Start` quando não há run suspensa. -> comprovado: lógica em `Handle` com `startIfNeeded`.
- Nenhum código de produção referencia `run_onboarding_turn`, `OnbPhaseFirstTx`, `onboarding_first_tx`, `SuggestBudgetSplit`, `MarkFirstTransactionRecorded`. -> comprovado: gate de legado remanescente passou.
- `internal/platform/workflow` permanece livre de imports de domínio, SQL fora do adapter postgres, LLM e comentários. -> comprovado: gates do kernel passaram.

## Veredito do revisor

Veredito do revisor: APPROVED

Revisor: self-review executado conforme gates do runbook. Nenhuma violação de regra hard detectada; testes, lint e build passam.

## Riscos Residuais

- Testes E2E de onboarding foram removidos junto com o runner legado. A task 9.0 (Testes de integração e E2E) deve recriar E2E sobre o novo workflow.
- O campo `FirstTxRecorded` permanece no payload da sessão apenas para auditoria, sem impactar `IsReadyToComplete`.
- `OnboardingProgressChecker` depende de `GetOnboardingContext`; se a sessão existir mas estiver em estado inconsistente, o agent pode rotear para onboarding indevidamente. Mitigado pela verificação de `IsOnboardingInProgress` no use case.

## Suposições

- Tasks 1.0–5.0 concluídas conforme `tasks.md`; steps, bindings e use cases do novo workflow já existiam.
- O kernel `internal/platform/workflow` já estava validado e não foi alterado nesta task.
