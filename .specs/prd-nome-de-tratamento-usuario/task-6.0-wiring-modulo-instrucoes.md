# Tarefa 6.0: Wiring do módulo, contrato de regressão e instruções do agente

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Ligar o fluxo treatment-name-edit no composition root (engine/registry/resumer/reaper/tool/write-set/index/dispatcher), incluir a tool no contrato de regressão pós-deploy, e ajustar as instruções do agente diário para (a) chamar `edit_treatment_name` na intenção de troca e (b) usar o nome de tratamento vigente da working memory de forma natural sem excesso.

<requirements>
- RF-05: uso natural do nome via instrução + working memory injetada.
- RF-06: roteamento por registry/tool, sem switch de intent.
- RF-08: fluxo de dois turnos habilitado pelo resume dispatcher.
- RF-10: isolamento — só working memory, nunca identity/cobrança.
- RF-15: rollout para todos sem feature flag = liberação direta.
</requirements>

## Subtarefas

- [x] 6.1 Em `internal/agents/module.go`: `treatmentNameEditEngine := workflow.NewEngine[workflows.TreatmentNameEditState](workflowStore, o11y)`; `treatmentNameEditDef := workflows.BuildTreatmentNameEditWorkflow(workingMem, onboardingAgent)`; registry por tipo + Register; `NewWorkflowResumer(TreatmentNameEditWorkflowID, registry, engine, workflows.TreatmentNameEditKey, workflows.ContinueTreatmentNameEdit)`; incluir o workflowID no `SuspendedRunIndex` e o resumer no `ResumeDispatcher`; reaper job `agents-treatment-name-edit-reaper`.
- [x] 6.2 Anexar a tool `BuildEditTreatmentNameTool(...)` aos `financialTools` do agente diário; adicionar `"edit_treatment_name"` ao `agent.WithWriteToolSet(...)`.
- [x] 6.3 Adicionar `"edit_treatment_name"` ao `internal/agents/application/postdeploy/regression_contract.go`.
- [x] 6.4 Em `internal/agents/application/agents/mecontrola_agent.go`, estender `mecontrolaAgentInstructions`: orientar a chamar `edit_treatment_name` (com `name` quando informado) na intenção de troca; e usar o nome de tratamento vigente (seção `## Nome de Tratamento` da working memory) de forma natural, sem repetição excessiva; sem tocar dados cadastrais (RF-10). Nenhuma feature flag (RF-15).
- [x] 6.5 Ajustar/rodar testes de wiring/instruções afetados (ex.: regression_contract_test).

## Detalhes de Implementação

Ver `techspec.md` (Arquitetura, Sequenciamento) e ADR-002. Blueprint de wiring do goal-edit em `internal/agents/module.go` (engine :209, def :215, registry :266-267, resumer :292-298, reaper :332-333, tool :433, write-set :249-254; SuspendedRunIndex :307-314, ResumeDispatcher :315-318). Ordem resume→onboarding→agent em `whatsapp_inbound_consumer.go:149-153`. Sem `switch case intent.Kind` (R-AGENT-WF-001.1).

## Critérios de Sucesso

Fluxo roteável fim-a-fim (start via tool + resume via dispatcher); RF-10 garantido por só usar working memory; sem flag (RF-15); zero comentários; testes de wiring verdes.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — wiring de engine/registry/resumer/reaper/write-set/index/dispatcher e instruções do agente.

## Testes da Tarefa

- [ ] Testes unitários (regression_contract e wiring afetados)
- [ ] Testes de integração (fim-a-fim coberto na Tarefa 7.0)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `internal/agents/module.go` (mod)
- `internal/agents/application/agents/mecontrola_agent.go` (mod)
- `internal/agents/application/postdeploy/regression_contract.go` (mod) + testes
- Consome Tarefas 3.0/4.0/5.0
