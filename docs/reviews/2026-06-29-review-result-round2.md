# Review — `internal/agents` (weather, paridade Mastra) vs `.specs/prd-agents-weather-mastra`

- Data: 2026-06-29
- Rodada: 2 (pós-bugfix)
- Skill: `review` (sem flexibilização)
- Veredito: **APPROVED**

## verdict

`APPROVED` — todos os 7 achados da rodada 1 foram remediados e verificados por evidência de código + execução real (unit + integração testcontainers pgvector com Docker). Zero achados remanescentes, zero falsos positivos, DoD 100%.

## review_cycles

Rodada 1 (review) → REJECTED (7 achados) → bugfix (F1–F7) → Rodada 2 (review) → APPROVED.

## Remediações verificadas

| # | RF/Task | Remediação | Evidência de verificação |
|---|---------|-----------|--------------------------|
| F1 | RF-30 | `gofmt -w scorers_test.go` | `git ls-files '*.go' \| xargs gofmt -l` → vazio (CI `lint:fmt:check` verde) |
| F2 | RF-16 | `ScorerRunner` + `ScoringHooks` (agent hooks) wired no `module.go`; persistência em `platform_scorer_results` vinculada ao Run; runID exposto no ctx (`agent.WithRunID`); shutdown cooperativo em server (defer) e worker (struct) | `module.go:118-120,165`; `scoring_hooks_test.go` (4 casos PASS); build+vet+test verdes |
| F3 | RF-19 / Task 7.0 | Conformance reescrita para consumir `internal/agents` real; port duplicado (`weather_tool.go`/`weather_workflow.go`/`weather_scorer.go`) deletado | `grep internal/agents conformance_test.go` → 6 imports; `ls test/conformance/weather/` → só `conformance_test.go`; `go test -tags integration ./test/conformance/weather/` → ok |
| F4 | RF-18/19 / Task 1.0,7.2 | Testes `//go:build integration` testcontainers pgvector: recall ANN escopado por resourceId (sem leak), replay não duplica embedding, Run audit round-trip | `TestAsyncChainScopedRecall` PASS, `TestReplayDoesNotDuplicateEmbedding` PASS, `TestRunAuditRoundTrip` PASS (execução real Docker) |
| F5 | RF-26 | Removidos ~19 campos `AgentConfig` órfãos, envs `AGENT_*` órfãs (`.env`/`.env.example`), validação obrigatória órfã `AGENT_ONBOARDING_LLM_MODEL`; `config_test.go` ajustado | `grep OnboardingModel\|ConvPrimaryModel\|CircuitFailures\|AGENT_ONBOARDING config.go` → vazio; `go test ./configs/...` verde |
| F6 | RF-18 | Idempotência por `ON CONFLICT (source_message_pk, model)` (dedup pela chave de conteúdo, mais forte que event_id), provada por teste de replay de integração. Sem nova migration (PRD veda nova migration sem ADR) | `TestReplayDoesNotDuplicateEmbedding` → `count(*)==1` |
| F7 | ADR-004 | Grupo `mecontrola.agent` de alertas Prometheus legados (4 métricas mortas + fallback órfão) removido | `prometheus-rules.yaml` sem refs a `agent_intent_parsed_total`/`agent_llm_*`/`agent_policy_*`/`agent_idempotency_*` |

## acceptance_matrix (RF-01..30) — todos `atendido`

- RF-01..04 módulo consumidor/DI/R0-R7/DMMF — atendido (gates grep verdes).
- RF-05..08 weather-agent/OpenRouter/sync+stream/tool+memória — atendido (conformance real).
- RF-09,10 tool get-weather + open-meteo — atendido.
- RF-11,12,13 weather-workflow agent-como-step — atendido (conformance real + suspend/resume).
- RF-14,15 scorers code-based + translation LLM-judged — atendido.
- **RF-16 scorers anexados/async/persistidos vinculados ao Run — atendido (F2)**.
- RF-17 memória Postgres chaves opacas — atendido.
- **RF-18 indexação assíncrona idempotente — atendido (F4: replay não duplica)**.
- **RF-19 recall demonstrável escopado por resourceId — atendido (F4: ANN scoped, sem leak)**.
- RF-20,21,22 WhatsApp E2E + Run auditável — atendido (F4 run round-trip + cadeia verificada na rodada 1).
- RF-23..27 cutover/eliminação/config — atendido (**F5** fecha config órfã).
- RF-28 O11Y cardinalidade controlada — atendido.
- **RF-29 CI determinístico + integração testcontainers — atendido (F4)**.
- **RF-30 build/gates/gofmt verdes — atendido (F1)**.

## validations_run (rodada 2)

- `go build ./...` → exit 0
- `go vet ./...` e `go vet -tags integration ./internal/platform/memory/... ./internal/platform/agent/... ./test/conformance/weather/...` → exit 0
- `go test ./...` (unit, todos os módulos) → PASS
- `go test -tags integration` (memory, agent/postgres, scorer, conformance, whatsapp, onboarding) → PASS
- `git ls-files '*.go' | xargs gofmt -l` → vazio
- Gates: `grep internal/agent` (≠ platform) vazio; `test -d internal/agent` ausente; zero comentários em `internal/agents/` e nos novos arquivos de plataforma
- Spawn de subagentes especializados: indexação/canal/cutover (rodada 1); config/config_test/prometheus/conformance/integração (rodada 2)

## findings

Nenhum.

## residual_risks

- Variante E2E real (`RUN_REAL_LLM`) permanece sob flag/nightly, fora do gate de merge (decisão ADR-005, não é gap).
- Sampling de scorers usa `AlwaysSample` (herdado da task 3.0; configurável; RF-16 satisfeito no núcleo attached/async/persistido).

## Alteração mínima de plataforma (justificada)

`internal/platform/agent`: novo `runid_context.go` (helper genérico `WithRunID`/`RunIDFromContext`, sem domínio) + injeção do runID no ctx em `runtime.Execute`. Estritamente necessário para vincular `platform_scorer_results` ao Run (RF-16), dentro do escopo permitido pelo PRD ("alteração … estritamente necessária para consumir").

## Adendo — Validação com OpenRouter real (RUN_REAL_LLM)

Solicitado: validar com OpenRouter real para 100% pronto. A validação real **encontrou um defeito de produção que todos os testes mockados escondiam**:

### F8 [critical] RF-05/07/20 — agente retornava conteúdo VAZIO em tool call
- `agent.Execute` fazia uma única chamada `Complete`: quando o modelo emitia um `tool_call`, a tool executava mas **não havia segunda rodada** para sintetizar o resultado em resposta textual → `Result.Content` vazio. Em produção (WhatsApp, caminho primário ADR-003), "clima em São Paulo?" → tool call → **nenhuma resposta enviada** (o consumer retorna nil com Content vazio).
- Detectado por `TestWeatherAgent_Execute_Sync` (real LLM): `Should NOT be empty, but was`.
- **Correção**: loop de tool-use em `agent.Execute` (`completeWithTools`) — após invocar tools, anexa a mensagem assistant com `tool_calls` + mensagens `role=tool` (`tool_call_id`, resultado) e re-chama `Complete` até resposta final (guarda `maxToolRounds`); extensão aditiva de `llm.Message` (ToolCalls/ToolCallID/Name) + mapeamento no openrouter (`toWireToolCalls`, `chatMessage`). Hooks propagados também ao streaming.
- **Regressão travada em CI**: testes determinísticos `TestExecute_ToolLoop_FeedsResultsBackToModel` e `TestExecute_ToolLoop_MaxRoundsEmptyContentErrors` (provider mockado), pois o caminho real fica fora do gate de merge.

### Validação real (OpenRouter, RUN_REAL_LLM=1)
- `internal/platform/llm` real (Complete + Embed): **ok**.
- Conformance weather completa contra OpenRouter: **ok** — 30/30 subtests (agent sync, stream, workflow agent-como-step, translation judge), 0 fail.
- Build, `golangci-lint run ./...` = 0, gofmt, unit suite, integração de escopo: verdes.

Blast radius: apenas o weather agent usa `WithTools`; agentes sem tools retornam na rodada 0 (comportamento idêntico ao anterior).
