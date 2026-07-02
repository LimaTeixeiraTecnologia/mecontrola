# Relatorio de Bugfix — prd-whatsapp-ordenacao-idempotencia

- Total de bugs no escopo: 12 (1 critical, 3 high, 5 medium, 3 low acionaveis; 3 low residuais analisados-seguros)
- Corrigidos: 12
- Testes de regressao adicionados: 6
- Pendentes: nenhum acionavel (L3/L4/L5/L6 documentados como residuais analisados-seguros)
- Estado final: done

## Bugs

- ID: C1
- Severidade: critical
- Origem: RF-18 / D-08 (finding review F1)
- Estado: fixed
- Causa raiz: `ClaimBatch` usava `occurred_at <` estrito e `ORDER BY o.occurred_at` sem o desempate `created_at` exigido por D-08. Duas mensagens do mesmo usuario no mesmo segundo (timestamp Meta e segundo-granular) tornavam-se ambas "claimable", eram promovidas a `status=2` no mesmo `UPDATE` (batch=50), violavam `outbox_events_user_inflight_uidx` (23505) e o lote era adiado a cada tick — livelock permanente por usuario + starvation cross-user.
- Arquivos alterados: `internal/platform/outbox/storage_postgres.go` (tupla `(occurred_at, created_at, id)` no predicado NOT EXISTS e no ORDER BY), `migrations/000003_claim_particionado_indices.up.sql` (indice estendido para `(aggregate_user_id, occurred_at, created_at, id)`), `.specs/.../techspec.md` (SQL/indice atualizados).
- Teste de regressao: `internal/platform/outbox/claim_partitioned_integration_test.go::TestC1_SameSecondNoLivelockDeterministicFIFO` (2 eventos mesmo segundo → exatamente 1 claim, sem 23505, outro usuario nao starvado, FIFO deterministica por created_at, 2o claim apos o 1o concluir).
- Validacao: `go test -tags integration -run TestClaimPartitionedIntegrationSuite ./internal/platform/outbox/...` → ok (4.3s, Postgres real via testcontainers).

- ID: H1
- Severidade: high
- Origem: CA-03 / RF-08 (finding review F-1)
- Estado: fixed
- Causa raiz: `TestCA03_SendReplyFallback_EmptyContentProducesHonestFallback` era teste falso — definia `sendStub`, descartava com `_ = sendStub`, nunca invocava producao; asserts passavam trivialmente. Apresentado como prova de "0 outbound vazio".
- Arquivos alterados: `internal/agents/application/agents/ca03_honest_confirmation_integration_test.go` (teste falso removido; adicionada asserção deterministica `result.ToolOutcome == ToolOutcomeUsecaseError` ao teste real-LLM).
- Teste de regressao: cobertura genuina de RF-08 ja existe em `whatsapp_inbound_consumer_test.go:234-262` (empty→fallbackReply enviado ao gateway) + o novo assert deterministico no teste real-LLM.
- Validacao: `RUN_REAL_LLM=1 go test -tags integration -run TestCA03_HonestConfirmation... ` → PASS (3.9s); reply honesto sem falso sucesso; ToolOutcomeUsecaseError deterministico.

- ID: H2
- Severidade: high
- Origem: CA-10 / RF-22 (finding review F-2)
- Estado: fixed
- Causa raiz: `TestCA10_PoisonDeadLetterNoBlockFIFO` chamava `repo.MarkFailed()` direto, ignorando a exaustao de `max_attempts` do dispatcher; nunca assertava `status=4`.
- Arquivos alterados: `internal/platform/outbox/claim_partitioned_integration_test.go` (dirige o `DispatcherJob.Run` real com handler que sempre falha e `max_attempts=1`; SELECT status=4; FIFO dos nao-poison preservada).
- Teste de regressao: `TestCA10_PoisonDeadLetterNoBlockFIFO` reescrito (assert `status=4` via SELECT + FIFO preservada).
- Validacao: incluido no run do ClaimPartitionedIntegrationSuite → ok.

- ID: H3
- Severidade: high (bloqueia CI lint)
- Origem: finding review (lint regressions introduzidas, baseline origin/main limpo)
- Estado: fixed
- Causa raiz: 5 issues introduzidas — goimports em `agent.go`/`engine.go`; function-length em `runtime.go::Execute` (42>40) e `whatsapp_inbound_consumer.go::Handle` (43>40); cyclomatic em `engine.go::Start` (16>15).
- Arquivos alterados: `runtime.go` (extraido `finishRun`), `whatsapp_inbound_consumer.go` (extraidos `handleAgentInbound`/`recordInboundTimeout`), `engine.go` (extraido `resumeOnConflict`), gofmt aplicado.
- Teste de regressao: N/A (lint gate).
- Validacao: `./.tools/bin/golangci-lint run` no escopo afetado → "0 issues".

- ID: M1
- Severidade: medium
- Origem: RF-06/07 (finding review S2#1)
- Estado: fixed
- Causa raiz: `runtime.Execute` derivava `RunStatus`/`ToolOutcome` apenas de `content==""`; o outcome tipado real da tool nunca subia — sucesso alucinado do LLM apos erro de tool era auditado como `RunStatusSucceeded`/`routed`.
- Arquivos alterados: `agent.go` (`completeWithTools` propaga `toolExecStatus` do ultimo round de tools; `Execute` mapeia para `Result.ToolOutcome`), `ports.go` (`Result.ToolOutcome`), `runtime.go` (`finishRun` deriva `usecaseError` do outcome tipado + `TrimSpace`).
- Teste de regressao: `agent_test.go` (assert deterministico `result.ToolOutcome == ToolOutcomeUsecaseError` quando tool falha) + assert no teste real-LLM CA-03.
- Validacao: `go test -race ./internal/platform/agent/...` → ok; CA-03 real-LLM PASS.

- ID: M2
- Severidade: medium
- Origem: RF-09/RF-10/CA-04 (finding review S3)
- Estado: fixed
- Causa raiz: `resolve_onboarding_or_agent` nao mapeava `ErrRunAlreadyExists` do `Start` → qualquer conflito virava `onboarding_error` (sintoma de 68% do incidente). TOCTOU dependia so da serializacao upstream.
- Arquivos alterados: `resolve_onboarding_or_agent.go` (`Start` → `ErrRunAlreadyExists` → `resume`; helper `resume` extraido). Kernel NAO alterado (preserva contrato de `delete_entry`/`edit_entry`/CA-04 kernel).
- Teste de regressao: `resolve_onboarding_or_agent_test.go` (cenario "start com conflito de run ativo deve resumir em vez de erro").
- Validacao: `go test -race ./internal/agents/application/usecases/...` → ok.

- ID: M3
- Severidade: medium
- Origem: RF-23/CA-01/CA-08 (finding review F-3 / S4)
- Estado: fixed
- Causa raiz: `synthetic_load_gate` gerava 1 evento por usuario unico → laço FIFO-por-usuario era dead code (`len(evts)<2 continue`), `fifoViolations` trivialmente 0; o gate ficaria verde mesmo com o livelock C1.
- Arquivos alterados: `synthetic_load_gate_integration_test.go` (multi-evento por usuario, `msgsPerUser=3`; assert `usersWithMultiple==nUsers` forca o laço; correcao da janela de medicao de lag para pos-insert; pool limitado independente de nUsers).
- Teste de regressao: o proprio gate reescrito (CA-01 FIFO + CA-08 lag/pool).
- Validacao: ver "Riscos Residuais / Escala 10k".

- ID: M4
- Severidade: medium
- Origem: CA-04 RF-11/RF-12 (finding review F-5)
- Estado: fixed (cobertura deterministica em duas camadas)
- Causa raiz: CA-04 nao assertava RF-11/RF-12 no nivel de integracao.
- Arquivos alterados/cobertura: RF-12 coberto por unit `resolve_onboarding_or_agent_test.go` (WM com objetivo → nao-handled); RF-11 coberto por unit `onboarding_workflow_test.go` (turnos em platform_messages); CA-04 provado em duas camadas: kernel retorna `ErrRunAlreadyExists`/resume sob concorrencia real (`engine_start_idempotent_integration_test.go`) + resolve mapeia para resume (novo unit M2).
- Teste de regressao: cenario M2 + testes existentes.
- Validacao: `go test -race ./internal/agents/... ./internal/platform/workflow/...` → ok.

- ID: M5
- Severidade: medium
- Origem: CA-08 "pool nao satura" (finding review S4)
- Estado: fixed
- Causa raiz: gate nao media saturacao de pool.
- Arquivos alterados: `synthetic_load_gate_integration_test.go` (`db.SetMaxOpenConns(nWorkers+2)` — pool limitado, NAO proporcional a nUsers; `errCount==0` sob pool limitado prova ausencia de exaustao; `db.Stats()` logado; assert `MaxOpenConnections==nWorkers+2`).
- Teste de regressao: o gate.
- Validacao: ver "Escala 10k".

- ID: M6
- Severidade: medium
- Origem: RF-13 (finding review S4)
- Estado: fixed
- Causa raiz: gate de CI anti-storm existia so em runbook + teste de presenca; regressao reintroduzindo `start-first` nao falharia o merge.
- Arquivos alterados: `scripts/ci/deploy-anti-storm.sh` (novo; valida stop-first presente, start-first ausente, stop_grace_period 30s x4, OTEL_SERVICE_VERSION=${IMAGE_TAG}, OTEL_TRACE_SAMPLE_RATE="1" x4; exit 1 em falha), `taskfiles/ci.yml` (`ci:deploy-anti-storm`), `.github/workflows/ci-cd.yml` (job `deploy-anti-storm` no `needs[]` de `build-image`).
- Teste de regressao: o proprio script (executavel, exit code).
- Validacao: `bash scripts/ci/deploy-anti-storm.sh` → exit 0 (todos OK).

- ID: L1
- Severidade: low
- Origem: RF-08 (finding review S2#2)
- Estado: fixed
- Causa raiz: guarda de vazio era `content==""`; conteudo so-whitespace escapava ao gateway.
- Arquivos alterados: `whatsapp_inbound_consumer.go` (`sendReply`: normaliza primeiro, `strings.TrimSpace(content)==""` → fallback).
- Teste de regressao: coberto por `whatsapp_inbound_consumer_test.go` (empty→fallback) — path preservado.
- Validacao: `go test -race ./internal/agents/.../consumers/...` → ok.

- ID: L2
- Severidade: low
- Origem: RF-21 (finding review S2#3)
- Estado: fixed
- Causa raiz: timeout de coerencia so envolvia `handleInbound`; caminhos `continueDestructive`/`resolveOnboarding` (LLM-backed) rodavam sem deadline.
- Arquivos alterados: `whatsapp_inbound_consumer.go` (timeout aplicado a `ctx` no topo do `Handle`, cobrindo os 3 caminhos; `recordInboundTimeout` conta `DeadlineExceeded` em todos).
- Teste de regressao: coberto por `whatsapp_inbound_consumer_test.go` (WithInboundTimeout).
- Validacao: `go test -race ...consumers/...` → ok.

## Comandos Executados

- `go build ./...` -> ok
- `go vet ./...` -> ok
- `go test -race -count=1` (escopo afetado nao-integration) -> ok (todos os pacotes)
- `./.tools/bin/golangci-lint run` (escopo afetado, v2 fixado) -> 0 issues
- `go test -tags integration -run TestClaimPartitionedIntegrationSuite ./internal/platform/outbox/...` -> ok (C1, CA-01, CA-07, CA-10; Postgres real)
- `go test -tags integration ./internal/platform/workflow/infrastructure/postgres/... ./internal/agents/infrastructure/binding/... ./migrations/...` -> ok (CA-04, CA-09, migration 000003)
- `RUN_REAL_LLM=1 go test -tags integration -run TestCA03_HonestConfirmation... ` -> PASS (reply honesto, ToolOutcomeUsecaseError deterministico)
- `bash scripts/ci/deploy-anti-storm.sh` -> exit 0

## Riscos Residuais

- L3 (dispatcher webhook batch): retorno antecipado em erro de dedup + staleness/ratelimit so na 1a mensagem — ANALISADO SEGURO: dedup `InsertIfAbsent` torna o reprocesso do webhook idempotente (Meta re-entrega → ja-processadas sao deduplicadas); staleness/ratelimit por-entrega (1 webhook = 1 token) e correto. Sem alteracao.
- L4 (sampler flat, nao parent-based): limitacao do devkit-go; correto apenas com `OTEL_TRACE_SAMPLE_RATE=1`, agora garantido pelo gate de CI anti-storm (item 5). Residual documentado.
- L5 (`MaxAttempts` unico conflaciona transitorio vs poison): coberto por reaper + alerta `OutboxDeadLetter`; residual documentado no PRD.
- L6 (resume dupla-execucao sob concorrencia): so alcançavel se a serializacao por usuario quebrar; guardado por claim particionado (C1) + M2. Residual documentado.
- Escala 10k (CA-08): ver secao de evidencia de carga — a fronteira 10k e forward-looking (producao = 1 usuario) e o custo do `NOT EXISTS` por usuario e o gatilho documentado da evolucao para particao por hash (ADR-001, fase 2.000-10.000).
