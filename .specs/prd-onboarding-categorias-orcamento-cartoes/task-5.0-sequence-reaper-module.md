# Tarefa 5.0: Montagem da `Sequence` + reaper de onboarding em `module.go`

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Montar a `Sequence` do onboarding na nova ordem, atualizar os `stepXxxID` e wirar um reaper dedicado de
runs suspensos do onboarding em `module.go` (D-12), garantindo durabilidade, erros tipados sem falso
sucesso e provider inalterado.

<requirements>
- RF-38: preservar workflow durável com suspensão/retomada por merge-patch; sem branching solto em handler/consumer.
- RF-39: falhas de IO retornam erro tipado no step correspondente.
- RF-40: falha técnica não afirma conclusão; rastreável por workflow, step, status e erro sanitizado.
- RF-41: extração usa o provider OpenRouter existente; sem outro provider nem fallback chain.
- D-12: reaper de onboarding com TTL 7 dias (ADR-005).
</requirements>

## Subtarefas

- [ ] 5.1 Atualizar os `stepXxxID` (`step-welcome`, `step-goal`, `step-monthly-budget`, `step-budget-review`, `step-activation`, `step-recurrence`, `step-cards`, `step-conclusion`).
- [ ] 5.2 Reescrever `BuildOnboardingWorkflow` com a nova `Sequence`: welcome → goal → monthly_budget → budget_review → activation → recurrence → cards → conclusion; aplicar `wrap` (mensagens) nos steps que suspendem; `activation` sem `wrap` (não suspende).
- [ ] 5.3 Adicionar `BuildOnboardingReaper(store, o11y)` + consts `OnboardingStaleAfter = 7 * 24 * time.Hour`, `OnboardingReaperBatch = 100` (molde de `NewStaleSuspendedReaper`).
- [ ] 5.4 Em `internal/agents/module.go`: `onboardingReaper := workflows.BuildOnboardingReaper(workflowStore, deps.O11y)`; `onboardingReaperJob := jobhandlers.NewConfirmReaperJob("agents-onboarding-reaper", onboardingReaper, "")`; adicionar `onboardingReaperJob` ao slice `Jobs: []worker.Job{...}`.
- [ ] 5.5 Confirmar assinatura de `BuildOnboardingWorkflow` (chamada em `module.go:231`) preservada; validar que erros de step propagam `StepStatusFailed`.

## Detalhes de Implementação

Ver `techspec.md` "Fluxo de Dados", "Monitoramento e Observabilidade" e ADR-005. O reaper segue o
padrão dos existentes (confirm/cardCreate/budgetCreation): job via `NewConfirmReaperJob`, adicionado
ao slice `Jobs`, executado por cron `*/5`.

## Critérios de Sucesso

- `go build ./... && go vet ./...` verdes no projeto.
- `go test ./internal/agents/... -count=1 -race` verde.
- A `Sequence` está na ordem correta; o reaper de onboarding aparece no slice `Jobs` de `module.go`.
- Erro forçado de IO em um step produz `RunStatusFailed` com `LastError` preenchido e nenhuma mensagem
  de conclusão (RF-40).

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — montagem do workflow durável no consumidor `internal/agents` e wiring do reaper/job no `module.go` sobre o substrato de plataforma.

## Testes da Tarefa

- [ ] Testes unitários
- [ ] Testes de integração

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/workflows/onboarding_workflow.go` — `stepXxxID`, `BuildOnboardingWorkflow`, `BuildOnboardingReaper`.
- `internal/agents/module.go` — wiring do `onboardingReaperJob` no slice `Jobs`.
- `internal/platform/workflow/reaper.go` — `NewStaleSuspendedReaper` (referência).
