# Tarefa 1.0: Decisão pura DecideRecurrence e tipos-estado fechados

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Implementar o núcleo de decisão puro do step de recorrência do onboarding — tipos-estado fechados (DMMF state-as-type) e a função pura `DecideRecurrence`, com testes unitários determinísticos, espelhando os padrões já existentes em `internal/agents/application/workflows/onboarding_workflow.go` (`distributionIntentKind:184-222`, `DecideMonthlyBudgetCents:342`, `DecideGoalValueCents:335`). Nenhum IO. Base para o RF-17 gate (b).

<requirements>
RF-01, RF-02, RF-03, RF-04, RF-06, RF-07, RF-08, RF-17 (parte unit).
Ref: `.specs/prd-recorrencia-orcamento-onboarding/techspec.md` seção "Modelos de Dados" e ADR-001 (`.specs/prd-recorrencia-orcamento-onboarding/adr-001-decisao-recorrencia-pura-tipos-fechados.md`).
</requirements>

## Subtarefas

- [ ] 1.1 Declarar `recurrenceIntentKind` (`recurrenceIntentNegative` | `recurrenceIntentPositive` | `recurrenceIntentUnclear`) com `errInvalid`, `String()`, `IsValid()` e `ParseRecurrenceIntentKind(string)`.
- [ ] 1.2 Declarar `recurrenceOutcomeKind` (`recurrenceOutcomeNone` | `recurrenceOutcomeDefault` | `recurrenceOutcomeSpecific` | `recurrenceOutcomeInvalid` | `recurrenceOutcomeAmbiguous`) com `String()` retornando exatamente os rótulos de métrica do RF-16 (`no_recurrence`, `default_12`, `specific_months`, `invalid_reprompt`, `ambiguous_reprompt`) e `IsValid()`.
- [ ] 1.3 Declarar struct `recurrenceDecision{Outcome recurrenceOutcomeKind; Months int}` e as const `recurrenceDefaultMonths = 12`, `recurrenceMinMonths = 1`, `recurrenceMaxMonths = 12`.
- [ ] 1.4 Implementar `DecideRecurrence(intent, hasMonths, months) recurrenceDecision` puro (prioridade RF-06: quantidade válida 1-12 vence intenção → `Specific`; fora de 1-12 → `Invalid`; sem meses: positiva → `Default(12)`, negativa → `None`, unclear/não-parseável → `Ambiguous`).
- [ ] 1.5 Testes unitários table-driven de `DecideRecurrence` (negativa, positiva-12, específica 1/3/12, inválida 0/13/24, unclear, precedência negativa+meses-válidos → `Specific`, positiva+meses-inválidos → `Invalid`) e dos enums (round-trip, zero-value, `String()` asserta os 5 rótulos exatos).

## Detalhes de Implementação

Ver `.specs/prd-recorrencia-orcamento-onboarding/techspec.md` seção "Modelos de Dados" (bloco Go de `DecideRecurrence` e enums) e ADR-001 (`.specs/prd-recorrencia-orcamento-onboarding/adr-001-decisao-recorrencia-pura-tipos-fechados.md`) — **referenciar, não duplicar**.

- Local (produção): `internal/agents/application/workflows/onboarding_workflow.go`.
- Local (teste): `internal/agents/application/workflows/onboarding_workflow_test.go` (whitebox `package workflows`, testify/suite).
- Zero comentários em Go de produção (R-ADAPTER-001.1).

## Critérios de Sucesso

- `DecideRecurrence` puro e determinístico, coberto por testes unitários sem mock.
- `recurrenceOutcomeKind.String()` cobre os 5 rótulos do RF-16 (`no_recurrence`, `default_12`, `specific_months`, `invalid_reprompt`, `ambiguous_reprompt`).
- `go test ./internal/agents/application/workflows/ -run '<testes da 1.0>'` verde.
- `go build ./...` e `go vet ./...` verdes.
- Zero comentários em Go de produção.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `domain-modeling-production` — decisão pura Decide* e tipos-estado fechados (state-as-type / DMMF) para intenção e outcome de recorrência
- `design-patterns-mandatory` — gate de padrões confirma "não aplicar padrão" GoF (função pura + enums + dispatch), evitando overengineering

## Testes da Tarefa

- [x] Testes unitários
- [ ] Testes de integração

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/workflows/onboarding_workflow.go`
- `internal/agents/application/workflows/onboarding_workflow_test.go`
- `.specs/prd-recorrencia-orcamento-onboarding/techspec.md`
- `.specs/prd-recorrencia-orcamento-onboarding/adr-001-decisao-recorrencia-pura-tipos-fechados.md`
