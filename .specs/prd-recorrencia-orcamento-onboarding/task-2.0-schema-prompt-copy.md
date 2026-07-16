# Tarefa 2.0: Schema dedicado, prompt e copy no Tom de Voz

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Criar o schema de saída estruturada dedicado ao step de recorrência (SEM tocar no `recurrenceSchema` compartilhado com `summary_confirm`) e reescrever o prompt do sistema para extrair intenção + quantidade de meses (numérica, por extenso ou `unclear`), além das constantes de copy no Tom de Voz oficial (pergunta que sinaliza 1-12, repergunta de inválido, confirmações). Espelha o par já existente `distributionIntentSystemPrompt` (`internal/agents/application/workflows/onboarding_workflow.go:789-796`) e o schema `distributionIntentSchema` (`:677-685`).

<requirements>
- RF-04 — extração de intenção de recorrência
- RF-05 — extração de quantidade de meses
- RF-07 — copy de repergunta de quantidade inválida
- RF-08 — copy de repergunta
- RF-14 — pergunta sinaliza opções "sim" (12 meses), número 1-12, ou "não"
- RF-15 — copy de confirmação no Tom de Voz
Ref: techspec seção "Modelos de Dados" (`recurrenceDecisionSchema` / `recurrenceExtract`) e "Visão Geral dos Componentes"; ADR-002 (`.specs/prd-recorrencia-orcamento-onboarding/adr-002-extracao-unica-intent-meses.md`).
</requirements>

## Subtarefas

- [ ] 2.1 Declarar `recurrenceExtract{Intent string; HasMonths bool; Months int}` e `recurrenceDecisionSchema` (intent enum `negative|positive|unclear`, `hasMonths` bool, `months` integer, `additionalProperties: false`, todos os campos `required`).
- [ ] 2.2 Reescrever `recurrenceSystemPrompt` (`onboarding_workflow.go:821`): extrair `intent`, `hasMonths`, `months`; converter por extenso `um..doze`; definir `unclear` quando não houver intenção reconhecível; instruir que a quantidade não deve ser coagida — a decisão de prioridade/limites é da função pura `DecideRecurrence`, não do LLM.
- [ ] 2.3 Novo `conclusionRecurrencePrompt` (`onboarding_workflow.go:772`) no Tom de Voz sinalizando as opções: "sim" (12 meses), número de 1 a 12 meses, ou "não" (RF-14).
- [ ] 2.4 `recurrenceInvalidReprompt` (RF-07) informando o intervalo 1-12 e pedindo quantidade válida.
- [ ] 2.5 Constantes de confirmação: `recurrenceConfirmationNone`, `recurrenceConfirmationDefault` e helpers `recurrenceConfirmationFor(months int) string` e `monthsLabel(n int) string` (singular "1 mês" / plural "N meses").
- [ ] 2.6 GARANTIR que `recurrenceSchema` / `yesNoExtract` / `summaryConfirmSystemPrompt` permaneçam intocados (usados por `summary_confirm` em `onboarding_workflow.go:1480`).

## Detalhes de Implementação

Ver techspec seção "Modelos de Dados" (contrato de `recurrenceDecisionSchema` / `recurrenceExtract`) e "Visão Geral dos Componentes"; ADR-002 (`.specs/prd-recorrencia-orcamento-onboarding/adr-002-extracao-unica-intent-meses.md`) para a decisão de extração única de intenção + meses.

A copy DEVE aderir ao Tom de Voz verificável pelos scorers: asterisco simples (nunca negrito duplo `**`) e emoji oficial, conforme `internal/agents/application/scorers/behavioral_scorers.go:309-346`. Zero comentários em Go de produção (R-ADAPTER-001.1).

## Critérios de Sucesso

- `recurrenceDecisionSchema` estrito criado (enum fechado, `additionalProperties: false`, todos `required`).
- `recurrenceSchema` inalterado — `summary_confirm` continua desserializando `yesNoExtract`.
- Copy no Tom de Voz (asterisco simples + emoji oficial), verificável pelos scorers.
- `go build` e `go vet` verdes.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — saída estruturada (`llm.Schema`) e prompt do agente na call-site sancionada do step, sem tocar no schema compartilhado do `summary_confirm`.

## Testes da Tarefa

- [ ] Testes unitários — validar shape do schema/const via testes de superfície e o helper `monthsLabel` (singular/plural).
- [ ] Testes de integração — não aplicável nesta tarefa.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/workflows/onboarding_workflow.go`
- `internal/agents/application/workflows/onboarding_workflow_test.go`
- `.specs/prd-recorrencia-orcamento-onboarding/techspec.md`
- `.specs/prd-recorrencia-orcamento-onboarding/adr-002-extracao-unica-intent-meses.md`
