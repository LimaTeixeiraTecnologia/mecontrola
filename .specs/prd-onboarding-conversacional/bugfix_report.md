# Relatorio de Bugfix

- Total de bugs no escopo: 1
- Corrigidos: 1
- Testes de regressao adicionados: 3
- Pendentes: []
- Estado final: done

## Bugs

- ID: BUG-002
- Severidade: major
- Origem: BUG-002 issue (bugfix_input.json)
- Estado: fixed
- Causa raiz: `OnboardingState` mantinha copias dos dados canonicos (Objective, IncomeCents, Cards) que pertencem exclusivamente a `onboarding_sessions`, criando risco de drift entre o snapshot do workflow kernel e a sessao duravel.
- Arquivos alterados:
  - `internal/agent/application/workflow/onboarding_state.go`
  - `internal/agent/application/workflow/onboarding_workflow.go`
  - `internal/agent/application/workflow/onboarding_steps_summary.go`
  - `internal/agent/application/workflow/onboarding_steps_values.go`
  - `internal/agent/application/workflow/onboarding_steps_budget.go`
  - `internal/agent/application/workflow/onboarding_steps_cards.go`
  - `internal/agent/application/workflow/onboarding_steps_objective.go`
  - `internal/agent/application/workflow/onboarding_steps_test.go`
  - `internal/agent/application/workflow/onboarding_workflow_test.go`
  - `internal/agent/infrastructure/onboarding/onboarding_bindings.go`
  - `internal/agent/module.go`
  - `internal/agent/application/services/onboarding_workflow_integration_test.go`
  - `internal/agent/infrastructure/messaging/database/consumers/onboarding_completed_integration_test.go`
  - `internal/onboarding/e2e/feature_onboarding_conversacional_steps_test.go`
  - `internal/onboarding/e2e/support_runtime_test.go`
- Teste de regressao:
  - `TestSummaryStep_UsesContextLoaderForCanonicalData`
  - `TestSummaryStep_ContextLoaderError_Fails`
  - `TestValuesStep_UsesContextLoaderIncome`
- Validacao:
  - `go build ./...` -> OK
  - `go vet ./...` -> OK
  - `go test -race -count=1 ./internal/agent/application/workflow/... ./internal/agent/application/services/... ./internal/agent/infrastructure/onboarding/... ./internal/onboarding/...` -> OK
  - `golangci-lint run ./internal/agent/application/workflow ./internal/agent/infrastructure/onboarding ./internal/agent` -> OK

## Comandos Executados

- `python3 .agents/skills/bugfix/scripts/validate-bug-input.py --input .specs/prd-onboarding-conversacional/bugfix_input.json` -> SUCCESS: 16 bugs validados
- `go build ./...` -> OK
- `go vet ./...` -> OK
- `go test -race -count=1 ./internal/agent/application/workflow/... ./internal/agent/application/services/... ./internal/agent/infrastructure/onboarding/... ./internal/onboarding/...` -> OK
- `golangci-lint run ./internal/agent/application/workflow ./internal/agent/infrastructure/onboarding ./internal/agent` -> OK

## Riscos Residuais

- Testes de integracao e e2e (com build tags `integration`/`e2e`) foram ajustados para incluir `ContextLoader`, mas nao foram executados por dependerem de containers Postgres; o build com tags passou.
- O step `values` passou a depender do `ContextLoader` para obter `IncomeCents`; se o loader falhar, o run falha. Isso e aceitavel porque a sessao deve existir quando o onboarding esta em progresso.
