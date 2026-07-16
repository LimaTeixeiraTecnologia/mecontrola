# Tarefa 3.0: Estado com meses e resumo retrocompatível

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Estender `OnboardingState` com os campos aditivos de recorrência e refletir o período real no resumo final, com retrocompatibilidade para snapshots legados in-flight (RF-20). Campos aditivos desserializam para zero-value; o estado vive no snapshot JSON do kernel e a retomada aplica merge-patch no resume.

<requirements>
- RF-11, RF-13, RF-20
- Ref: techspec "Modelos de Dados" (campos aditivos) e "Fluxo de Dados"
- ADR-003 (`.specs/prd-recorrencia-orcamento-onboarding/adr-003-confirmacao-encadeada-retrocompat-estado.md`)
</requirements>

## Subtarefas

- [ ] 3.1 Adicionar a `OnboardingState` (`internal/agents/application/workflows/onboarding_workflow.go:310-325`) os campos `RecurrenceMonths int \`json:"recurrenceMonths"\`` e `RecurrenceConfirmation string \`json:"recurrenceConfirmation"\``, preservando `Recurrence bool`.
- [ ] 3.2 Alterar `recurrenceSummaryLine` (`:966-971`) para a assinatura `(recurrence bool, months int)` refletindo N; fallback legado RF-20: `recurrence==true && months<=0 → 12`; `recurrence==false → "desligada"`.
- [ ] 3.3 `conclusionSummaryMessage` (`:973-991`) passa `state.RecurrenceMonths` para `recurrenceSummaryLine`.
- [ ] 3.4 Atualizar os testes de resumo existentes que assertam a copy: `onboarding_workflow_test.go:3557` ("🔁 Recorrência: desligada") e `:3593` ("...ligada..."), incluindo caso novo de N específico (ex.: 3 meses) e caso legado (`Recurrence==true`, `RecurrenceMonths==0` → "12 meses").

## Detalhes de Implementação

Ver techspec "Modelos de Dados" (campos aditivos de `OnboardingState`) e "Fluxo de Dados", e ADR-003 (`.specs/prd-recorrencia-orcamento-onboarding/adr-003-confirmacao-encadeada-retrocompat-estado.md`) para o contrato de retrocompatibilidade dos snapshots in-flight.

A regra de leitura do fallback (legado `Recurrence==true` sem meses → 12) DEVE viver num único ponto: `recurrenceSummaryLine`. Nenhum outro call-site pode reimplementar o fallback. Zero comentários em Go de produção. Testes whitebox no `package workflows`, testify/suite.

## Critérios de Sucesso

- Campos aditivos `RecurrenceMonths` e `RecurrenceConfirmation` com tags JSON, preservando `Recurrence bool`.
- Resumo reflete N meses / "desligada" / legado-12 a partir de `recurrenceSummaryLine`.
- Testes de resumo verdes, incluindo o caso legado (`Recurrence==true`, `RecurrenceMonths==0` → "12 meses") e o caso de N específico.
- `go build`, `go vet` e `go test ./internal/agents/application/workflows/` verdes.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — estado do workflow no snapshot do kernel (campos aditivos) e retomada retrocompatível por merge-patch
- `domain-modeling-production` — modelagem de estado (quantidade de meses) representando ausência de recorrência de forma explícita

## Testes da Tarefa

- [ ] Testes unitários (resumo com N, desligada, legado-12)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/workflows/onboarding_workflow.go`
- `internal/agents/application/workflows/onboarding_workflow_test.go`
- `.specs/prd-recorrencia-orcamento-onboarding/techspec.md`
- `.specs/prd-recorrencia-orcamento-onboarding/adr-003-confirmacao-encadeada-retrocompat-estado.md`
