# Tarefa 8.0: Consumidor de referência `test/conformance/weather` + suite de conformidade

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Portar o exemplo weather do `limateixeira-agents` para Go em `test/conformance/weather` (fora de `internal/platform` — é consumidor, não plataforma) e montar a suite de conformidade que prova production-ready: unit determinístico, integração (testcontainers Postgres+pgvector) e E2E atrás de `RUN_REAL_LLM` (OpenRouter real + Postgres real). Exercita todas as capacidades nucleares end-to-end.

<requirements>
- RF-44: consumidor de referência weather (agent+tool+workflow+scorer) exercitando todas as capacidades nucleares.
- RF-45: suite de conformidade E2E com OpenRouter real e Postgres real atrás de flag `RUN_REAL_LLM`; CI padrão determinístico sem rede LLM.
- RF-46: testes de persistência contra Postgres real (testcontainers) com migrations e pgvector.
- Sem a flag: modo determinístico (provider fake + testcontainers); passos que exigem LLM real fazem `t.Skip`.
</requirements>

## Subtarefas

- [ ] 8.1 Portar `weatherTool` (open-meteo geocoding+forecast) via `tool.NewTool[I,O]`.
- [ ] 8.2 Portar `weatherAgent` (instructions, model OpenRouter, tools, memory, scorers) via `internal/platform/agent`.
- [ ] 8.3 Portar `weatherWorkflow` (`fetchWeather` → `planActivities` usando `agent.Stream` como step) via kernel + agent-como-step.
- [ ] 8.4 Portar scorers (tool-call accuracy, completeness, translation LLM-judged) via `internal/platform/scorer`.
- [ ] 8.5 Suite de conformidade: cenários por capacidade (sync, stream, structured, tool, workflow multi-step, suspend/resume idempotente, memória thread/working/recall, runtime context, scorers), com gate por `RUN_REAL_LLM`.

## Detalhes de Implementação

Ver techspec.md "Testes E2E", "Mapeamento Requisito → Decisão → Teste", ADR-008. Fontes de referência (conceituais, portar — não copiar literalmente): `/Users/jailtonjunior/Git/limateixeira-agents/agents/src/mastra/{agents/weather-agent.ts,tools/weather-tool.ts,workflows/weather-workflow.ts,scorers/weather-scorer.ts}`.

## Critérios de Sucesso

- Modo determinístico verde no CI padrão (provider fake + testcontainers), cobrindo todas as capacidades nucleares.
- Variante `RUN_REAL_LLM=1` verde sob demanda (OpenRouter real + Postgres real).
- Cada RF do mapeamento da techspec tem cenário E2E correspondente executável.
- Persistência real exercitada (threads, runs, embeddings/recall HNSW, scorer_results).

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`. -->

- `go-implementation` — implementação Go obrigatória (CLAUDE.md): consumidor de referência e suite de testes.
- `mastra` — o consumidor é um agent+tool+workflow+scorer Mastra portado (skill canônica).

## Testes da Tarefa

- [ ] Testes E2E de conformidade (determinístico e variante `RUN_REAL_LLM`): todas as capacidades nucleares end-to-end.
- [ ] Testes de integração (`//go:build integration`, testcontainers `pgvector/pgvector:pg16`): persistência e recall reais.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `test/conformance/weather/` (novo) — consumidor de referência + suite.
- `internal/platform/{agent,tool,memory,scorer,llm,workflow}/` — plataforma exercitada.
