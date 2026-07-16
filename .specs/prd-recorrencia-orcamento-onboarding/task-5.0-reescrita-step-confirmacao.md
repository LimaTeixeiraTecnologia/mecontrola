# Tarefa 5.0: Reescrita do BuildRecurrenceStep e confirmação encadeada

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Reescrever o `BuildRecurrenceStep` para: extração única por turno (um `agent.Execute` com
`recurrenceDecisionSchema`) → `DecideRecurrence` → dispatch por outcome; aplicar
`CreateRecurrence(ctx, userUUID, competence, decision.Months)` apenas em `Default`/`Specific`
(0 falso-sucesso); reperguntar em `Invalid`/`Ambiguous` sem aplicar; setar a confirmação encadeada
e refleti-la no cards step, espelhando o padrão `GoalConfirmation` → `BuildMonthlyBudgetStep`
(`onboarding_workflow.go:1113-1118`). Consome as tarefas 1.0-4.0.

<requirements>
- RF-01, RF-02, RF-03, RF-07, RF-08, RF-09, RF-10, RF-12, RF-18, RF-19
- Referência: techspec.md, seções "Fluxo de Dados" e "Design de Implementação"; ADR-002, ADR-003, ADR-004
</requirements>

## Subtarefas

- [ ] 5.1 Nova assinatura `BuildRecurrenceStep(a agent.Agent, budgets interfaces.BudgetPlanner, rec observability.Counter)`.
- [ ] 5.2 Primeira entrada (`ResumeText == ""`) suspende com o novo `conclusionRecurrencePrompt` e `PhaseRecurrence`.
- [ ] 5.3 Resposta: um `agent.Execute` com `recurrenceDecisionSchema` → `recurrenceExtract` → `ParseRecurrenceIntentKind` → `DecideRecurrence`.
- [ ] 5.4 Dispatch por outcome:
  - `Invalid` → `record(invalid_reprompt)` + `suspend(recurrenceInvalidReprompt)` sem `CreateRecurrence`;
  - `Ambiguous` → `record(ambiguous_reprompt)` + `suspend(conclusionRecurrencePrompt)` sem `CreateRecurrence`;
  - `None` → `state.RecurrenceConfirmation = recurrenceConfirmationNone` + `record(no_recurrence)` + `completeStep`;
  - `Default`/`Specific` → resolver competence (`America/Sao_Paulo` + `competenceLocation` fallback UTC, padrão `onboarding_workflow.go:1546-1547`), `CreateRecurrence(N)`, `state.Recurrence = true`, `state.RecurrenceMonths = N`, `state.RecurrenceConfirmation = recurrenceConfirmationFor(N)`, `record(outcome.String())`, `completeStep`; erro em `CreateRecurrence` → `failStep`.
- [ ] 5.5 RF-09: `Invalid`/`Ambiguous` reperguntam indefinidamente (sem contador; sem avançar fase). RF-19: nenhum caminho condicional de feature flag.
- [ ] 5.6 Confirmação encadeada em `BuildCardsStep` (`onboarding_workflow.go:1149-1161`): prefixar `state.RecurrenceConfirmation` no prompt inicial e zerar o campo (espelhar `:1113-1118`).
- [ ] 5.7 Wiring: passar `rec` em `BuildOnboardingWorkflow` (`onboarding_workflow.go:1651`); manter a assinatura de `BuildOnboardingWorkflow` e o call-site `module.go:244` inalterados.
- [ ] 5.8 Testes unitários do step (mock agent `RawJSON` novo schema; mock `BudgetPlanner` com contagem): primeira entrada; negativa (`CreateRecurrence` NÃO chamado); positiva → `CreateRecurrence(12)`; específica 3 → `CreateRecurrence(3)`; específica 1 → confirmação singular; inválida 13/0 → suspend sem `CreateRecurrence`; ambígua → suspend sem `CreateRecurrence`; erro `CreateRecurrence` → `failStep`; cards step prefixa `RecurrenceConfirmation`.
- [ ] 5.9 Atualizar testes existentes impactados: prompt inicial `onboarding_workflow_test.go:2938`; mock 12 `:2959`; cenário ambíguo `:2994-3016` (passa a esperar `StepStatusSuspended`); assinatura `:3048`; surface map `TestM02_NoRendaTermInAnyOnboardingSurface:3869/3876` (incluir novos prompts/reprompts/confirmações).

## Detalhes de Implementação

Referenciar techspec.md, seção "Fluxo de Dados". Manter UMA chamada `agent.Execute` por turno
(preserva a contagem `.Once()` do full-flow). Não tocar `summary_confirm`/`recurrenceSchema`.
Zero comentários em Go de produção. Aderência a R-AGENT-WF-001 (step fino, LLM só na call-site
sancionada, roteamento por mapa de decisão sem switch de domínio). Ver ADR-002, ADR-003 e ADR-004
para as decisões de extração única, dispatch por outcome e confirmação encadeada.

## Critérios de Sucesso

- 0 falso-sucesso (`CreateRecurrence` só em `Default`/`Specific`).
- Confirmação imediata setada e prefixada no cards step.
- RF-09: repergunta sem cap em `Invalid`/`Ambiguous`.
- Sem feature flag (RF-19).
- Testes unitários do step verdes.
- Testes existentes atualizados verdes.
- `go build`, `go vet` e `go test ./internal/agents/application/workflows/` verdes.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — reescrita de step de workflow (adapter fino → binding/usecase), LLM só na call-site sancionada, estados de fronteira fechados e confirmação encadeada, conforme R-AGENT-WF-001

## Testes da Tarefa

- [x] Testes unitários
- [ ] Testes de integração

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/workflows/onboarding_workflow.go`
- `internal/agents/application/workflows/onboarding_workflow_test.go`
- `.specs/prd-recorrencia-orcamento-onboarding/techspec.md`
- `.specs/prd-recorrencia-orcamento-onboarding/adr-002-*.md`, `adr-003-*.md`, `adr-004-*.md`
