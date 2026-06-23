# Tarefa 8.0: [agent] RunOnboardingTurn refatorado + adapters de binding

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Refatorar `RunOnboardingTurn` em `internal/agent` para ser ponte pura: remover `buildAutoSplits`
(cálculo movido para budgets) e o acesso a `agent_sessions`; passar a chamar os bindings do onboarding
(`SuggestBudgetSplit`, `OnboardingHistoryGateway`, `MarkWelcomeSent`). Implementar os adapters finos
de binding. Off-topic é tratado por redirecionamento à etapa atual (system prompt/scripts).

<requirements>
- RF-13: chama `SuggestBudgetSplit` (onboarding→budgets) para o preview.
- RF-14/RF-15: ajuste preserva progresso (delegado aos usecases do onboarding).
- RF-20/RF-21: histórico via `OnboardingHistoryGateway` (onboarding_sessions); zero `agent_sessions`.
- RF-30: retomada a partir do estado persistido.
- RF-33: agentes isolados; RF-36: off-topic → redireciona.
- ADR-001, ADR-004, ADR-006.
</requirements>

## Subtarefas

- [ ] 8.1 Implementar adapters finos `OnboardingHistoryGateway` e `BudgetAllocator`/`BudgetSplitSuggester` (binding → usecases).
- [ ] 8.2 Substituir `buildAutoSplits(...)` por chamada ao binding `SuggestBudgetSplit`; remover a função do agente.
- [ ] 8.3 Substituir `loadOnbHistory`/`saveOnbTurn` (agent_sessions) por chamadas ao gateway de histórico; `emitWelcome` chama `MarkWelcomeSent`.
- [ ] 8.4 Off-topic: ajustar system prompt para resposta breve + redirecionamento à etapa atual.
- [ ] 8.5 Testes unitários (suite testify; bindings mockados; `emitWelcome` idempotente; sem cálculo de split no agente).

## Detalhes de Implementação

Ver techspec.md → "Visão Geral dos Componentes" (`internal/agent`), "Interfaces Chave" e ADR-006.
Tool/Workflow finos (R-AGENT-WF-001.2); LLM apenas no parse (R-AGENT-WF-001.4).

## Critérios de Sucesso

- `buildAutoSplits` removido de `internal/agent`; nenhum acesso a `agent_sessions` no fluxo de onboarding.
- O agente só orquestra LLM + chama bindings; sem regra/SQL de outro módulo.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`. -->

- `mastra` — refatora o fluxo de turno do onboarding no `internal/agent` (Workflow/Tool, Thread→Run, LLM só no parse).

go-implementation (linguagem, auto) e agent-governance (governança, auto) também se aplicam.

## Testes da Tarefa

- [ ] Testes unitários (suite testify; bindings mockados; idempotência welcome; ausência de cálculo de split)
- [ ] Testes de integração (T12 — fluxo de turno e2e)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Definition of Done (DoD)

- [ ] `buildAutoSplits` e uso de `agent_sessions` removidos do fluxo de onboarding no agente.
- [ ] Adapters de binding finos (sem SQL/regra); zero comentários no `.go`.
- [ ] `go build ./internal/agent/...` e `go test ./internal/agent/application/usecases/... -run Onboarding` passam.

## Critérios de Aceite (validações executáveis)

```bash
go build ./internal/agent/... && \
go test ./internal/agent/application/usecases/... -run Onboarding -count=1
grep -rn "buildAutoSplits\|agent_sessions" internal/agent --include="*.go" | grep -iv _test | grep -i onboard && echo FAIL || echo OK
```

## Arquivos Relevantes
- `internal/agent/application/usecases/run_onboarding_turn.go` (modificado)
- `internal/agent/infrastructure/.../onboarding_history_gateway.go` (novo)
- `internal/agent/infrastructure/.../budget_split_suggester.go` (novo)
- `internal/agent/module.go` (wiring dos adapters)
