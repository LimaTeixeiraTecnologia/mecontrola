# Tarefa 7.0: Scorers/Evals em `internal/platform/scorer`

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Criar `internal/platform/scorer`: primitivo de avaliação equivalente a `@mastra/core/evals`. Scorers code-based (determinísticos) e LLM-judged (com structured output via `llm`), anexáveis a um agente com sampling, executados assincronamente fora do caminho crítico, com resultados persistidos em `platform_scorer_results`.

<requirements>
- RF-40: `Scorer` com modalidades `codeBased` e `llmJudged` (tipo fechado `ScorerKind`).
- RF-41: anexável ao agente com `Sampling` configurável (tipo fechado `SamplingType`), sem alterar o caminho principal do agente.
- RF-42: scorer LLM-judged produz structured output validável; falha explícita se não conformar.
- RF-43: resultados (score, reason, metadata) persistidos em Postgres vinculados ao Run, chaves opacas.
- Sampling default: `1.0` em dev/conformidade, `0.1` em produção (overridável).
- Runner assíncrono cancelável, sem leak de goroutine; falha de scorer nunca afeta o agente.
</requirements>

## Subtarefas

- [ ] 7.1 Tipos fechados `ScorerKind` (`codeBased|llmJudged`) e `SamplingType` (`ratio|always|never`) com `String()`/`IsValid()`/`Parse*`.
- [ ] 7.2 `Scorer`/`ScoreResult`/`RunSample`/`Sampling`; scorers code-based (tool-call accuracy, completeness).
- [ ] 7.3 Scorer LLM-judged via `llm.Complete` + `StructuredContract` (judge configurável por modelo).
- [ ] 7.4 `ScorerRunner.Observe` assíncrono com `context` cancelável e shutdown cooperativo; aplica sampling.
- [ ] 7.5 Adapter Postgres `platform_scorer_results`; métricas `scorer_runs_total`/`scorer_duration_seconds` (labels `scorer_id`/`kind`/`outcome`).

## Detalhes de Implementação

Ver techspec.md "Interfaces Chave > Scorer", ADR-006, e o exemplo `weather-scorer.ts` (`/Users/jailtonjunior/Git/limateixeira-agents/agents/src/mastra/scorers/weather-scorer.ts`) como referência conceitual (toolCallAccuracy, completeness, translation LLM-judged). DTOs de input com `Validate()` (R-DTO-VALIDATE-001).

## Critérios de Sucesso

- Scorers code-based e LLM-judged produzem score+reason; structured output do judge validado (falha explícita em não-conformidade).
- Sampling respeitado (1.0/0.1) — testável; resultados persistidos e auditáveis.
- Runner não vaza goroutine (teste de shutdown); falha de scorer não propaga para o agente.
- Layering/gates: sem comentários em Go; sem labels de alta cardinalidade; sem import de domínio.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`. -->

- `go-implementation` — implementação Go obrigatória (CLAUDE.md): scorers, runner concorrente, persistência, testes.
- `mastra` — Scorers/Evals são primitivo do padrão Mastra portado (paridade @mastra/core/evals).

## Testes da Tarefa

- [ ] Testes unitários (testify/suite whitebox): sampling (ratio/always/never), code-based determinístico, LLM-judged decode conforme/não-conforme, cancelamento do runner.
- [ ] Testes de integração (`//go:build integration`, testcontainers Postgres): persistência de `platform_scorer_results` vinculada ao Run.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes
- `internal/platform/scorer/` (novo) — domain/application/infrastructure.
- `internal/platform/llm/` — `Complete`/structured output do judge.
- `migrations/000003_*` — tabela `platform_scorer_results`.
