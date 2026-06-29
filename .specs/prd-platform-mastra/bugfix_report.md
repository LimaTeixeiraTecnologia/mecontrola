# Relatorio de Bugfix

- Total de bugs no escopo: 7
- Corrigidos: 7
- Testes de regressao adicionados: 5
- Pendentes: nenhum no escopo acordado. FORA do escopo (decisão do usuário "só mecânicos agora"): B1 (remoção de internal/agent), B3 (wiring de indexação assíncrona), B6/B7/B8 (suite E2E/conformidade/persistência real) — permanecem bloqueando APPROVED.
- Estado final: done

## Bugs

- ID: B2
- Severidade: major
- Origem: review finding B2 (gate de merge `lint:fmt:check`); RF não-específico (gate transversal)
- Estado: fixed
- Causa raiz: 8 arquivos de produção + alguns testes não passavam em `gofmt`; o gate `lint:fmt:check` (`[ -z "$(gofmt -l .)" ]`) falharia no merge.
- Arquivos alterados: internal/platform/agent/{agent.go,errors.go,types.go,agent_test.go,runtime_test.go}, internal/platform/memory/infrastructure/postgres/thread_repository.go, internal/platform/llm/openrouter_test.go, internal/platform/scorer/{code_based.go,runner.go,types.go,scorer_test.go}, test/conformance/weather/weather_tool.go (gofmt -w em todo o escopo da plataforma + test/).
- Teste de regressao: gate objetivo — `git ls-files '*.go' | xargs gofmt -l` retorna vazio (CI-equivalente).
- Validacao: `gofmt -l` sobre arquivos rastreados → CLEAN; `.claude/worktrees/` é git-ignored (ausente no checkout de CI).

- ID: B5
- Severidade: major
- Origem: review finding B5; RF-04, RF-07
- Estado: fixed
- Causa raiz: `resultStream.drain()` enviava em canal bufferizado (64) com send bloqueante; `Result()` aguardava `done` mas nunca lia `out`. Consumidor que só quer o resultado estruturado (sem drenar `Deltas()`) travava e vazava a goroutine de drain quando o modelo emitia >64 chunks.
- Arquivos alterados: internal/platform/agent/stream.go (`Result()` agora drena `out` enquanto aguarda `done`/`ctx`, mantendo `drain()` sempre destravado; ao detectar `out` fechado aguarda `done` e retorna).
- Teste de regressao: internal/platform/agent/agent_test.go::TestStream_ResultWithoutDrainingDeltasDoesNotBlockOrLeak — emite 200 deltas, chama `Result()` SEM ler `Deltas()` com timeout de 2s; antes do fix retornaria `ctx.Err()` (deadlock), agora retorna conteúdo completo (200) e prova que `Deltas()` fechou (drain concluído, sem leak).
- Validacao: `go test ./internal/platform/agent/...` PASS.

- ID: B4
- Severidade: major
- Origem: review finding B4; RF-18, RF-23
- Causa raiz: `embedding_repository.Index` fazia INSERT puro; `platform_embeddings` não tinha UNIQUE em `(source_message_pk, model)`. Replay duplicava embeddings.
- Estado: fixed
- Arquivos alterados: migrations/000003_platform_mastra.up.sql (índice único parcial `platform_embeddings_source_model_uniq (source_message_pk, model) WHERE source_message_pk IS NOT NULL`); internal/platform/memory/infrastructure/postgres/embedding_repository.go (`ON CONFLICT (source_message_pk, model) WHERE source_message_pk IS NOT NULL DO NOTHING`). O down dropa a tabela inteira (índice incluso) — reversibilidade preservada.
- Teste de regressao: internal/platform/memory/infrastructure/postgres/embedding_repository_test.go::TestIndex_UsesOnConflictForIdempotency (mock DBTX; matcher exige `ON CONFLICT` + `DO NOTHING` na query).
- Validacao: `go test ./internal/platform/memory/infrastructure/postgres/...` PASS; `go test -tags=integration -run TestMigrationSuite/TestMigration000003PlatformMastra ./migrations/...` PASS (DDL do índice parcial aplica e é reversível em Postgres real).

- ID: M1
- Severidade: minor
- Origem: review finding M1; RF-29
- Causa raiz: `agentImpl.Execute` e `agentRuntime.Execute` emitiam o MESMO `agent_runs_total`/`agent_run_duration_seconds`; como o runtime chama `agent.Execute`, a execução padrão contava 2x.
- Estado: fixed
- Arquivos alterados: internal/platform/agent/agent.go (removidas as métricas `agent_runs_total`/`agent_run_duration_seconds` do `agentImpl`; runtime — dono do Run auditável — permanece o único emissor; `agentImpl` mantém `agent_stream_total` e `agent_tool_invocations_total`).
- Teste de regressao: internal/platform/agent/runtime_metrics_test.go::TestExecute_EmitsRunMetricExactlyOnce — runtime envolvendo um `agentImpl` REAL; assere exatamente 1 valor em `agent_runs_total` e 1 em `agent_run_duration_seconds` por execução (antes do fix: 2).
- Validacao: `go test ./internal/platform/agent/...` PASS.

- ID: M3
- Severidade: minor
- Origem: review finding M3; RF-33
- Causa raiz: `Run.Outcome` exposto como `string` livre na fronteira em vez do tipo fechado `ToolOutcome`.
- Estado: fixed
- Arquivos alterados: internal/platform/agent/ports.go (`Outcome ToolOutcome`); runtime.go (`closeRun` recebe `ToolOutcome`; mapeamento por caso: missingResolver/usecaseError/routed); infrastructure/postgres/run_store.go (mapeamento `ToolOutcome`↔TEXT via `outcomeText`, parse no Load; `errors.Is(sql.ErrNoRows)`).
- Teste de regressao: internal/platform/agent/runtime_metrics_test.go (mesma) — assere `run.Outcome == ToolOutcomeRouted` (valor tipado) no Run persistido em sucesso. Compilação garante ausência de string livre na fronteira.
- Validacao: `go build ./...` OK; GATE-5 (tipos fechados) PASS; `go test ./internal/platform/agent/...` PASS.

- ID: M4
- Severidade: minor
- Origem: review finding M4; RF-43
- Causa raiz: erro de `ResultStore.Insert` descartado com `_ =` → perda silenciosa de auditoria.
- Estado: fixed
- Arquivos alterados: internal/platform/scorer/runner.go (captura o erro de Insert; emite log `scorer.runner.persist.failed` + métrica `scorer_runs_total{outcome="persist_error"}`).
- Teste de regressao: internal/platform/scorer/scorer_test.go::TestObserve_PersistError_IsObservedNotSwallowed — store retorna erro; assere entrada de log `scorer.runner.persist.failed`.
- Validacao: `go test ./internal/platform/scorer/...` PASS.

- ID: L1
- Severidade: minor
- Origem: review finding L1; RF-09 (techspec: "NewTool valida contra schema")
- Causa raiz: `tool.Invoke` só fazia `json.Unmarshal`; o JSON Schema declarado em `in` nunca validava `required`/`enum`/etc.
- Estado: fixed
- Arquivos alterados: internal/platform/tool/tool.go (validação lazy via `santhosh-tekuri/jsonschema/v6` — compila o schema uma vez por tool com `sync.Once`; valida `argsJSON` antes do `execFn`; schema vazio ou não-compilável → pula validação para não quebrar tools sem schema). go.mod: dependência promovida a direta.
- Teste de regressao: internal/platform/tool/tool_test.go::TestNewTool_Invoke_SchemaValidation — payload `{}` viola `required:["city"]` → erro "schema validation"; payload válido passa.
- Validacao: `go test ./internal/platform/tool/...` PASS; tools existentes com schema `{"type":"object"}` permanecem verdes.

## Comandos Executados
- `go build ./...` -> EXIT 0
- `go vet ./internal/platform/... ./test/conformance/...` -> EXIT 0
- `git ls-files '*.go' | xargs gofmt -l` -> vazio (CLEAN)
- `go test -count=1 ./internal/platform/... ./test/conformance/...` -> PASS (sem FAIL)
- `task gates:platform` -> EXIT 0 (GATE-1..5 PASS)
- `go test -tags=integration -run TestMigrationSuite/TestMigration000003PlatformMastra ./migrations/...` -> PASS
- `go mod tidy` -> jsonschema/v6 promovido a dependência direta

## Riscos Residuais
- Fora do escopo acordado (decisão "só mecânicos"), continuam bloqueando APPROVED: B1 (internal/agent vivo + wired em cmd/server e cmd/worker, com a migration 000003 dropando as tabelas agent_* que ele consulta — regressão de runtime), B3 (indexação assíncrona de embeddings não conectada via outbox/worker → recall vazio em produção), B6/B7/B8 (consumidor weather não exercita memória/recall/scorer LLM end-to-end; sem teste de persistência Postgres real; sem variante RUN_REAL_LLM). Esses não são bugs mecânicos e exigem decisão/implementação à parte.
- B4: a execução real do `ON CONFLICT` não é coberta por teste de integração do repositório de embeddings (apenas asserção de query unit + validação do DDL do índice parcial em Postgres real). A inferência de índice parcial é sintaxe Postgres padrão e o índice agora existe.
- M3: `run_store` continua sem teste de integração que inspecione a linha persistida (residual pré-existente); o conteúdo tipado é coberto por teste unit via fake store.
