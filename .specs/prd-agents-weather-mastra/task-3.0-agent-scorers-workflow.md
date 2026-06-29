# Tarefa 3.0: `weather-agent` + scorers + `weather-workflow` (agent-como-step)

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Definir o `weather-agent` (instructions, model OpenRouter, tool, memória), os scorers (code-based + LLM-judged com structured output) e o `weather-workflow` (`fetch-weather` → `plan-activities`, com agent-como-step via streaming), promovendo `test/conformance/weather`.

<requirements>
- RF-05: weather-agent registrado/resolvido, instructions equivalentes ao Mastra.
- RF-06: OpenRouter como canal LLM; modelo configurável.
- RF-07: execução síncrona e streaming sob o mesmo contrato.
- RF-08: tool + memória vinculadas.
- RF-11/12/13: weather-workflow `{city}`→`{activities}`, estado `S` tipado, agent-como-step (plan-activities via `agent.Stream`).
- RF-14: scorers code-based `tool-call-accuracy` + `completeness`.
- RF-15: scorer LLM-judged `translation` com structured output validável (falha explícita).
- RF-16: sampling configurável; assíncrono fora do caminho crítico; resultados persistidos.
</requirements>

## Subtarefas

- [ ] 3.1 `agent.go`: `buildWeatherAgent(provider, tool, o11y)` com instructions e `WithTools`; registrar no `AgentRegistry`.
- [ ] 3.2 `scorers.go`: `tool-call-accuracy` (`NewToolCallAccuracyScorer`), `completeness` (`NewCompletenessScorer`), `translation` (`NewLLMJudgedScorer` + structured contract); montar `[]ScorerEntry` com sampling.
- [ ] 3.3 `workflow.go`: `Definition[WeatherState]` com steps `fetch-weather` (forecast) e `plan-activities` (agent-como-step via `agent.Stream`, runtime context); encadeamento Sequence.
- [ ] 3.4 Validar fim-de-stream/structured output (contrato `Result(ctx)` que drena `Deltas()`).

## Detalhes de Implementação

Ver techspec.md §"Modelos de Dados" (WeatherState), §"Mapeamento RF→Decisão→Teste", ADR-003. Reaproveitar `test/conformance/weather/{weather_workflow.go,weather_scorer.go}`.

## Critérios de Sucesso

- Agent responde (sync + stream) usando a tool; workflow produz `{activities}` a partir de `{city}`.
- Scorer translation falha explicitamente quando o contrato não é satisfeito.
- Teste de streaming com >64 deltas sem drenar `Deltas()` não trava (fix B5).
- Zero comentários; tipos fechados na fronteira; gofmt limpo.

## Skills Necessárias

<!-- MANDATÓRIO -->

- `mastra` — Agent, Workflow (agent-como-step), Scorers e o ciclo de execução seguem o modelo Mastra mapeado ao `internal/platform`.

## Testes da Tarefa

- [ ] Testes unitários (agent sync/stream com provider fake; scorers code-based; translation judge; workflow steps)
- [ ] Testes de integração (workflow durável no kernel; opcional nesta tarefa)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/agents/{agent.go,scorers.go,workflow.go}`; base: `test/conformance/weather/{weather_workflow.go,weather_scorer.go}`.
