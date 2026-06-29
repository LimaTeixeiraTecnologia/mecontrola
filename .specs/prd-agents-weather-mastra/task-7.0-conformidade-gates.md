# Tarefa 7.0: Conformidade + gates (testcontainers, RUN_REAL_LLM, gate de ausência de internal/agent)

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Fechar o enforcement: promover a conformidade weather para exercitar o `internal/agents` real, integração testcontainers (Postgres+pgvector), variante E2E real atrás de `RUN_REAL_LLM`, e gates de governança no CI (incl. ausência de `internal/agent`), com gofmt limpo.

<requirements>
- RF-28: O11Y reutilizada; métricas de cardinalidade controlada (labels enums fechados).
- RF-29: CI padrão determinístico (provider fake + testcontainers pgvector + migrations); variante real atrás de `RUN_REAL_LLM`, fora do gate de merge.
- RF-30: build verde, gates de governança verdes (import sem `internal/agent`, zero comentários nas camadas novas, cardinalidade, tipos fechados), gofmt limpo (`lint:fmt:check`).
- ADR-005.
</requirements>

## Subtarefas

- [ ] 7.1 Promover `test/conformance/weather` para exercitar `internal/agents` real (agent sync+stream, tool, workflow agent-como-step, memória, scorers, runtime context).
- [ ] 7.2 Integração `//go:build integration` (testcontainers `pgvector/pgvector:pg16`, migrations 000001..000003): memória/recall/runs/scorers/indexação idempotente.
- [ ] 7.3 Variante E2E real sob `RUN_REAL_LLM=1` (OpenRouter+Postgres reais), fora do gate de merge.
- [ ] 7.4 Gate de ausência de `internal/agent` em `taskfiles/gates.yml` + CI; manter gates de governança (kernel sem domínio/LLM, zero comentários, cardinalidade, tipos fechados) e `lint:fmt:check`.

## Detalhes de Implementação

Ver techspec.md §"Abordagem de Testes", §"Conformidade com Padrões" (gates), ADR-005. Reaproveitar a infra de `taskfiles/gates.yml`/`ci.yml`.

## Critérios de Sucesso

- CI determinístico verde (unit + integração testcontainers); variante real verde sob demanda.
- Gates verdes: import sem `internal/agent`, zero comentários, cardinalidade, tipos fechados, gofmt.
- Conformidade weather exercita o módulo real (não mocks de memória).

## Skills Necessárias

<!-- MANDATÓRIO -->

- `taskfile-production` — configurar/operacionalizar os gates e jobs de teste/integração no Taskfile e CI.
- `mastra` — a suite de conformidade exercita agent/workflow/tool/scorer/memory conforme o modelo Mastra.

## Testes da Tarefa

- [ ] Testes unitários (conformidade determinística verde)
- [ ] Testes de integração (testcontainers pgvector; recall real; replay idempotente)

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `test/conformance/weather/*` (promovido), `taskfiles/gates.yml`, `taskfiles/ci.yml`, `.github/workflows/ci-cd.yml`.
