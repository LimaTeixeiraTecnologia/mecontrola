# Tarefa 4.0: Introduzir camada agentiva offline e reconciliar suites complementares

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Adicionar a camada offline que prova invariantes críticos no boundary `llm.Provider`/`agent.Agent`, reaproveitando o runtime real do projeto e preservando `realllm` como smoke complementar. Quando houver mocks, o uso de `.mockery.yaml` do repositório é obrigatório, e os testes devem seguir o padrão `testify/suite` com cenários table-driven no formato aprovado.

<requirements>
- Cobrir RF-09, RF-10, RF-11 e RF-12.
- Provar onboarding com extração combinada de objetivo e valor sem provider real.
- Provar honestidade em falha de tool via `ToolOutcomeUsecaseError`, sem falso sucesso textual.
- Provar roteamento financeiro mínimo com sequência e argumentos mínimos no runtime real.
- Reconciliar as suites `realllm` complementares para que continuem úteis como smoke, mas não como evidência exclusiva.
- Usar `buildFinancialTools`/harness corrigido como base, respeitando a dependência em `3.0`.
- Usar `.mockery.yaml` quando a tarefa depender de mocks do runtime ou de interfaces do stack agentivo.
</requirements>

## Subtarefas

- [ ] 4.1 Criar helper de provider roteirizado ou harness equivalente no boundary `llm.Provider`.
- [ ] 4.2 Cobrir `BuildGoalStep` com extração estruturada e asserts de `StepStatus`, `GoalValueAsked` e `GoalValueCents`.
- [ ] 4.3 Cobrir `BuildMeControlaAgent` com sequência explícita de tool calls para C1, C4, C5 e um cenário principal de escrita.
- [ ] 4.4 Reconciliar as suites complementares `realllm` para manter o papel de smoke sem duplicar fonte de verdade.

## Detalhes de Implementação

Consultar `techspec.md`, especialmente:
- `## Arquitetura do Sistema` para o runtime real `internal/platform/agent`.
- `## Abordagem de Testes` para provider roteirizado, `agent.Result`, `ToolCalls`, `ToolOutcome` e oráculos determinísticos.
- `ADR-003` para a decisão de seam agentivo offline no boundary `llm.Provider`/`agent.Agent`.

## Critérios de Sucesso

- Existe camada offline reproduzível para onboarding combinado, honestidade em falha de tool e roteamento mínimo.
- Os testes usam o runtime real do agente, não um fake paralelo de workflow.
- A suite complementar `realllm` continua existente e coerente com o novo baseline.
- O padrão de teste segue `testify/suite` + cenários table-driven no estilo aprovado.
- Mocks, quando necessários, respeitam `.mockery.yaml`.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`.
     NÃO inclua aqui skills auto-carregadas em runtime: `agent-governance`, `execute-task`, `bugfix`,
     `review`, `refactor`, nem skills `*-implementation` (linguagem, inferida pelo diff).
     Use o conteúdo único `Nenhuma além das auto-carregadas (governance + linguagem).` se a tarefa
     não exigir skill processual extra. -->

- `mastra` — a tarefa altera agentes, workflows, scorers/evals, onboarding e o runtime agentivo real de `internal/agents`.

## Testes da Tarefa

- [ ] `go test -race -count=1 ./internal/agents/application/agents/...`
- [ ] `go test -race -count=1 ./internal/agents/application/workflows/...`
- [ ] `go test -race -count=1 ./internal/agents/application/scorers/...`
- [ ] `go test -tags=integration -count=1 ./internal/agents/application/agents/...` quando o ambiente estiver disponível

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/application/agents/mecontrola_agent_test.go`
- `internal/agents/application/agents/onboarding_goal_value_realllm_test.go`
- `internal/agents/application/workflows/onboarding_workflow.go`
- `internal/agents/application/workflows/onboarding_workflow_test.go`
- `internal/agents/application/scorers/mecontrola_tools_realllm_test.go`
- `internal/platform/agent/agent.go`
