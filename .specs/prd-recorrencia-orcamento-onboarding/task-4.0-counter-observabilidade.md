# Tarefa 4.0: Counter de outcome do step de recorrência

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Adicionar observabilidade de produto para o step de recorrência via counter Prometheus com label `outcome` fechado e cardinalidade controlada, espelhando o par existente `agents_onboarding_distribution_total` (`onboarding_workflow.go:1258-1283`) e `agents_onboarding_monthly_budget_total` (`:1086-1107`), e fazer o wiring no builder do workflow.

<requirements>
- RF-16.
- Ref: techspec "Monitoramento e Observabilidade" e "Visão Geral dos Componentes".
- Ref: ADR-004 (`.specs/prd-recorrencia-orcamento-onboarding/adr-004-gate-real-llm-zero-falso-sucesso.md`).
</requirements>

## Subtarefas

- [ ] 4.1 Declarar const `recurrenceOutcomeMetric = "agents_onboarding_recurrence_total"` e a factory `newRecurrenceOutcomeCounter(o11y) observability.Counter` (via `Counter(name, desc, "1")`).
- [ ] 4.2 Implementar `recordRecurrenceOutcome(ctx, rec observability.Counter, outcome string)` com guard nil e `rec.Add(ctx, 1, observability.String("outcome", outcome))` — label único `outcome` (valores vindos de `recurrenceOutcomeKind.String()`). PROIBIDO usar `months`/`user_id`/`competence` como label (R-TXN-004 / R-AGENT-WF-001.5).
- [ ] 4.3 Wiring em `BuildOnboardingWorkflow` (`:1637-1642`): declarar `var rec observability.Counter` no bloco `if o11y != nil` e atribuir `rec = newRecurrenceOutcomeCounter(o11y)`; passar `rec` a `BuildRecurrenceStep` na Sequence (`:1651`). Nota: a assinatura final de `BuildRecurrenceStep` é entregue pela tarefa 5.0; esta tarefa prepara o counter e o ponto de wiring (coordenar com 5.0, que é dependente).

## Detalhes de Implementação

Referenciar techspec "Monitoramento e Observabilidade" e "Visão Geral dos Componentes" e o ADR-004 desta pasta em vez de duplicar conteúdo. A assinatura de `BuildOnboardingWorkflow` e o call-site em `internal/agents/module.go:244` NÃO mudam (counter criado internamente). Zero comentários em Go de produção.

## Critérios de Sucesso

- Counter `agents_onboarding_recurrence_total` com label `outcome` fechado.
- Guard nil no `recordRecurrenceOutcome`.
- Nenhuma label de alta cardinalidade (`user_id`/`months`/`competence`).
- `go build` e `go vet` verdes.
- Gate de cardinalidade (grep por `user_id`/`months` como label em métrica) retorna vazio.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — métrica de Run/step com cardinalidade controlada (labels enum fechados), conforme R-AGENT-WF-001.5 / R-TXN-004

## Testes da Tarefa

- [ ] Testes unitários

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

O step com counter é exercitado na tarefa 5.0; aqui garantir compilação/wiring e ausência de label proibido.

## Arquivos Relevantes
- `internal/agents/application/workflows/onboarding_workflow.go`
- `.specs/prd-recorrencia-orcamento-onboarding/techspec.md`
- `.specs/prd-recorrencia-orcamento-onboarding/adr-004-gate-real-llm-zero-falso-sucesso.md`
