# Review — `internal/agents` (weather, paridade Mastra) vs `.specs/prd-agents-weather-mastra`

- Data: 2026-06-29
- Rodada: 1
- Skill: `review` (sem flexibilização)
- Veredito: **REJECTED**

## verdict

`REJECTED` — há achados `high` que violam critérios de aceite explícitos (RF-16, RF-19, RF-26, RF-30) e critérios de sucesso de tarefas (1.0, 3.0, 7.0). Não é admissível `APPROVED` nem `APPROVED_WITH_REMARKS` para esta execução.

## files_reviewed

- `.specs/prd-agents-weather-mastra/{prd.md,techspec.md,tasks.md,adr-001..005,task-1.0,task-3.0,task-7.0}`
- `internal/agents/module.go`
- `internal/agents/application/agents/agent.go`
- `internal/agents/application/scorers/scorers.go` (+ `scorers_test.go`)
- `internal/agents/application/tools/tool.go`, `application/workflows/workflow.go`, `application/usecases/handle_inbound.go`
- `internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer.go`
- `internal/platform/memory/publishing_message_store.go`, `infrastructure/indexer/handler.go`
- `internal/platform/agent/runtime.go`, `internal/platform/outbox/dispatcher.go`
- `internal/platform/scorer/{scorer.go,runner.go}`
- `cmd/server/server.go`, `cmd/server/whatsapp_wiring.go`, `cmd/worker/worker.go`
- `configs/config.go`
- `test/conformance/weather/conformance_test.go`
- `migrations/000003_platform_mastra.up.sql`

## refs_loaded

`AGENTS.md`, `.claude/rules/{workflow-kernel,agent-workflows-tools,go-adapters,go-testing,input-dto-validate,governance,transactions-workflows}.md`

## validations_run

- `go build ./...` → verde (exit 0)
- `go vet ./internal/agents/... ./internal/platform/memory/...` → verde
- `go test ./internal/agents/... ./internal/platform/memory/...` → verde (unit)
- `gofmt -l .` → **reporta `internal/agents/application/scorers/scorers_test.go`** (gate `lint:fmt:check` FALHA)
- Gates techspec: `grep internal/agent` (≠ platform) vazio ✓; `test -d internal/agent` ausente ✓; zero comentários em `internal/agents/` ✓
- Docker disponível (testcontainers possível), porém os testes de integração mandatados não existem.

## acceptance_matrix

| Item | Status | Evidência |
|------|--------|-----------|
| RF-01..04 (módulo consumidor, DI, R0-R7, DMMF) | atendido | `internal/agents/module.go`; gate grep vazio |
| RF-05..08 (weather-agent, OpenRouter, sync/stream, tool+memória) | atendido | `agent.go`; runtime |
| RF-09,10 (tool get-weather + open-meteo) | atendido | `tools/tool.go`, `infrastructure/weather/*` |
| RF-11,12,13 (weather-workflow agent-como-step) | atendido (unit) | `workflows/workflow.go`, testes |
| RF-14 (scorers code-based) | atendido (código existe) | `scorers.go` |
| RF-15 (scorer translation LLM-judged) | atendido (código existe) | `scorers.go:25` |
| **RF-16 (scorers anexados ao agente, assíncrono, persistidos vinculados ao Run)** | **bloqueado** | `BuildWeatherScorers` nunca é chamado fora do teste; sem `ScorerRunner` no `module.go`; agente não anexa scorers |
| RF-17 (memória Postgres por chaves opacas) | atendido | `module.go`, memory postgres |
| **RF-18 (indexação assíncrona idempotente por event_id)** | **parcial/sem evidência** | decorator+handler wired em produção; **sem dedup por event_id no consumer** (só `ON CONFLICT`); **sem teste de replay** |
| **RF-19 (semantic recall demonstrável)** | **bloqueado** | nenhum teste de integração Append→evento→worker→`platform_embeddings`→Recall ANN; conformance mocka `MessageStore` |
| RF-20,21,22 (WhatsApp E2E + Run auditável) | atendido | cadeia inbound→outbox→consumer→runtime→gateway; `platform_runs` insert/update |
| RF-23 (`internal/agent` removido 100%) | atendido | gate grep/`test -d` vazios |
| RF-24 (onboarding conversacional desligado) | atendido | dispatcher rota única; `MatchActivationCommand`→`OutcomeNoRoute` |
| RF-25 (cmd/server,worker religados) | atendido | `agents.NewModule` em server/worker |
| **RF-26 (config migrada, sem variáveis órfãs)** | **bloqueado** | ~17 campos `AgentConfig` órfãos; 23 envs `AGENT_*` órfãs em `.env`/`.env.example`; validação obrigatória órfã exige `AGENT_ONBOARDING_LLM_MODEL` (`config.go:1179`) não consumido |
| RF-27 (migration 000003 mantida) | atendido | `migrations/000003_*` dropa 7 tabelas `agent_*` |
| RF-28 (O11Y cardinalidade controlada) | atendido | labels enums; sem `resource_id`/`thread_id` como label |
| **RF-29 (CI determinístico + integração testcontainers)** | **bloqueado** | inexistentes testes `//go:build integration` para memória/recall/runs/scorers/indexação |
| **RF-30 (gofmt limpo, gate `lint:fmt:check`)** | **bloqueado** | `scorers_test.go` não-gofmt → gate vermelho |
| Task 1.0 — replay event_id não duplica (integração) | bloqueado | teste ausente |
| Task 1.0 — recall ANN escopado por resourceId (integração) | bloqueado | teste ausente |
| Task 3.0/RF-16 — resultados de scorer persistidos | bloqueado | scorers não wired |
| Task 7.0 — "Conformidade weather exercita o módulo real (não mocks)" | bloqueado | conformance usa port duplicado `test/conformance/weather/*`, não `internal/agents` |
| Task 7.0 — integração testcontainers (7.2) | bloqueado | ausente |

## findings (severidade canônica)

### F1 [high] RF-30 — gofmt gate vermelho
- `internal/agents/application/scorers/scorers_test.go` não está gofmt-formatado → `[ -z "$(gofmt -l .)" ]` falha → gate `lint:fmt:check` (RF-30) quebra o CI.
- fix: `gofmt -w internal/agents/application/scorers/scorers_test.go`.

### F2 [high] RF-16 — scorers não anexados/executados/persistidos
- `BuildWeatherScorers` (`scorers.go:29`) só é chamado por `scorers_test.go`. `module.go` não instancia `scorer.NewScorerRunner` (existe em `internal/platform/scorer/runner.go`), não anexa scorers ao agente, e não persiste `platform_scorer_results` no caminho de inbound. RF-16 exige scorers anexados ao agente, com sampling, executados assíncronos, persistidos vinculados ao Run.
- fix: wire `ScorerRunner` + entries no `module.go`/runtime; persistir resultados por Run; teste.

### F3 [high] RF-19 / Task 7.0 — conformance não promovida ao módulo real
- `test/conformance/weather/conformance_test.go` exercita o port local duplicado (`NewWeatherTool`/`NewWeatherWorkflow`/`NewWeatherScorerEntries` em `test/conformance/weather/*.go`) e mocka `MessageStore`/`ThreadGateway`/`RunStore`. Nenhum teste importa `internal/agents` para conformance. Viola task 7.0 ("exercita o módulo real, não mocks de memória") e ADR-005 (promoção). Resultado: o código de produção `internal/agents` (agent/tool/workflow/scorers) é coberto só por seus unit tests; a "prova viva" valida código duplicado.
- fix: promover a suite para `internal/agents` real e remover/rapar o port duplicado.

### F4 [high] RF-18/19 / Task 1.0 & 7.2 — testes de integração mandatados ausentes
- Não existe teste `//go:build integration` para: (a) Append→evento→worker→`platform_embeddings`→Recall ANN escopado por `resourceId`; (b) replay de `event_id` não duplica; (c) `platform_runs`/`platform_scorer_results` round-trip. Tasks 1.0 e 7.2 e o techspec §Abordagem de Testes os listam como "Adotados". RF-19 exige recall "demonstrável".
- fix: adicionar integração testcontainers `pgvector/pgvector:pg16` (migrations 000001..000003) cobrindo recall ANN, replay idempotente e runs.

### F5 [high] RF-26 — config órfã + validação obrigatória órfã
- Apenas 6 campos `AgentConfig` consumidos (PrimaryModel, EmbedModel, OpenRouterAPIKey, OpenRouterBaseURL, MaxTokens, Temperature). ~17 campos órfãos (OnboardingModel/OnboardingMaxTokens/Conv*/Parse*/Policy*/Circuit*/HTTPReferer/XTitle/RequestTimeout/MaxInputChars/ProseMaxTokens). 23 envs `AGENT_*` órfãs em `.env`/`.env.example`. Pior: `config.go:1179` exige `AGENT_ONBOARDING_LLM_MODEL` quando há config de WhatsApp/onboarding presente — branch que dispara em produção (WhatsApp creds setadas) e bloqueia startup por uma env que nada consome. RF-26 exige "sem variáveis órfãs".
- fix: remover campos/envs/validações órfãs; manter só o config consumido pelo módulo; ou passar HTTPReferer/XTitle/RequestTimeout ao provider se desejado.

### F6 [medium] RF-18 — dedup por event_id não implementado no consumer
- ADR-002 e task 1.0 exigem idempotência dupla: dedup por `event_id` no consumer **e** `ON CONFLICT`. `indexer/handler.go` não dedup por `event_id`; depende só do `ON CONFLICT (source_message_pk, model)`. Funcionalmente o replay não duplica (outbox marca processed + ON CONFLICT), mas o contrato de dedup por event_id não está implementado e não há teste.
- fix: dedup por `event_id` (ou justificar formalmente que `ON CONFLICT` + outbox basta) + teste de replay.

### F7 [low] ADR-004 — alertas Prometheus legados órfãos
- `deployment/monitoring/prometheus-rules.yaml:99-148` mantém alertas para métricas do `internal/agent` removido (`agent_intent_parsed_total`, `agent_llm_provider_errors_total`, `agent_policy_blocks_total`, `agent_idempotency_replay_total`) que nunca mais disparam.
- fix: remover o grupo de alertas legado.

## residual_risks

- Cobertura E2E real (`RUN_REAL_LLM`) existe apenas no port duplicado; o módulo real não tem variante E2E real.
- Sem integração testcontainers, a corretude de recall ANN e idempotência de indexação é não-verificada (só presumida).

## review_cycles

Rodada 1 (review): REJECTED. Próximo passo: `bugfix` sobre F1–F7, seguido de nova rodada de review sobre o delta.

## Falsos positivos descartados (verificados)

- "Decorator/handler não wired (OutboxPublisher nil)": **falso positivo** — em produção `identityModule.OutboxPublisher` é não-nil (`internal/identity/module.go:156`; passado em `cmd/server:223`/`cmd/worker:231`); o `if deps.OutboxPublisher != nil` é guarda defensiva, não defeito. A cadeia de indexação ESTÁ conectada em produção.
- WhatsApp inbound→outbound, Run auditável, cardinalidade de métricas: verificados e corretos.
</content>
</invoke>
