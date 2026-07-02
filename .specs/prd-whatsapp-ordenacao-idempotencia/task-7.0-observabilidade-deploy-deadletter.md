# Tarefa 7.0: Observabilidade do caminho crítico + deploy seguro + dead-letter

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Tornar o caminho `webhook → agente → LLM → envio` observável fim-a-fim (sampler parent-based +
`traceparent` obrigatório no `metadata` jsonb do outbox), expor o conflito de Start como outcome, medir
lag/idempotência/dead-letter, corrigir `OTEL_SERVICE_VERSION` e configurar deploy anti-storm com
dead-letter/backoff dos eventos inbound (RF-13, RF-14, RF-15, RF-16, RF-22; ADR-004).

<requirements>
- RF-14: spans do caminho crítico (`whatsapp.handler.inbound`, `agent.runtime.execute`, `llm.complete`) correlacionáveis por `run_id`/`thread_id`; NÃO criar spans novos onde já existem.
- Sampler **parent-based com raiz AlwaysOn no caminho inbound** (hoje `OTEL_TRACE_SAMPLE_RATE=0.1` derruba 90%); configurar no provider OTel de `cmd/server`/`cmd/worker`.
- D-10 (obrigatório): propagar `traceparent` (W3C) no `metadata` (JSONB) do `outbox_events` na publicação e restaurá-lo no consumer, costurando o hop assíncrono server→worker num único trace. Producer/consumer permanecem adapters finos (R-ADAPTER-001).
- RF-15: contador `workflow_version_conflict_total` (existente) + outcome novo `resumed_on_conflict` (label de outcome em `workflow_runs_total`/`workflow_resume_total`) para o Start idempotente-resume; cardinalidade controlada — sem `user_id`/`correlation_key`/`category_id` (R-TXN-004, R-WF-KERNEL-001.4).
- RF-13: deploy `order: stop-first` + `max_parallelism: 1` + `stop_grace_period: 30s` (≥ shutdown ≈15s) + gate de CI anti-storm que serializa/consolida releases; PROIBIDO `order: start-first`. Reaper `STUCK_AFTER=5m` permanece como rede de segurança.
- RF-16: `OTEL_SERVICE_VERSION=${IMAGE_TAG}` nos 4 serviços (server-1/2, worker-1/2) no `compose.swarm.yml`.
- RF-22: `max_attempts`/backoff dos eventos inbound dimensionados para dead-letter (`status=4`) rápido (≈1 turno), preservando FIFO estrito; alerta em `status=4 > 0`.
- Métricas de ordenação/idempotência: lag `occurred_at → published_at` (p95, alerta > 30s), duplicidade de escrita (=0), outbound vazio (=0), reivindicações adiadas por "usuário em voo", eventos em dead-letter, timeouts de LLM/tool. Sem labels de alta cardinalidade.
</requirements>

## Subtarefas

- [ ] 7.1 Configurar sampler parent-based (raiz AlwaysOn no inbound) no provider OTel de `cmd/server`/`cmd/worker`.
- [ ] 7.2 Propagar `traceparent` no `metadata` jsonb do outbox (producer) e restaurar no consumer (adapters finos).
- [ ] 7.3 `engine.go`: adicionar outcome `resumed_on_conflict` (cardinalidade controlada) ao Start idempotente-resume.
- [ ] 7.4 `compose.swarm.yml`: `OTEL_SERVICE_VERSION=${IMAGE_TAG}` + `stop_grace_period: 30s` + `order: stop-first` + `max_parallelism: 1` nos 4 serviços.
- [ ] 7.5 Dimensionar `max_attempts`/backoff dos eventos inbound para dead-letter (`status=4`) em ~1 turno; alerta `status=4 > 0`.
- [ ] 7.6 Novas métricas de lag/duplicidade/outbound-vazio/dead-letter/timeouts (sem `user_id`/`correlation_key`).
- [ ] 7.7 Gate de CI anti-storm que serializa/consolida releases (documentado em runbook) + painéis Grafana.

## Detalhes de Implementação

Ver ADR-004 §Decisão (itens 1–5) e §Plano de Implementação, techspec §Monitoramento e Observabilidade e
§Pontos de Integração (Docker Swarm). Depende de 2.0 (claim para métricas de lag/adiamento) e de 6.0 (o
outcome `resumed_on_conflict` nasce do Start idempotente-resume). A verificação end-to-end (traces
visíveis, versão == binário) é a CA-06 na tarefa 9.0.

## Critérios de Sucesso

- Trace do inbound visível fim-a-fim (parent-based + `traceparent` no metadata costurando o hop).
- `resumed_on_conflict` observável; `OTEL_SERVICE_VERSION` == tag do binário.
- Deploy `stop-first` + `stop_grace_period: 30s` + gate anti-storm configurados; PROIBIDO `start-first`.
- Dead-letter (`status=4`) em ~1 turno com alerta; FIFO preservado.
- Métricas de lag/duplicidade/outbound-vazio sem labels de alta cardinalidade.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`. -->

- `otel-grafana-dashboards` — cria/atualiza os painéis Grafana (lag p95, `onboarding_error`, `resumed_on_conflict`, outbound vazio, dead-letter) sobre a stack otel-lgtm para serviços instrumentados com OpenTelemetry.

## Testes da Tarefa

- [ ] Testes unitários
- [ ] Testes de integração

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

Unitários: propagação/restauração do `traceparent` no metadata; contagem de outcome
`resumed_on_conflict`; guarda de cardinalidade das métricas. Ensaio de deploy (CA-05) e traces
end-to-end (CA-06) são validados na tarefa 9.0.

## Rollback

Reverter para taxa fixa de amostragem e remover `traceparent` do metadata (ADR-004 §Riscos); reverter o
`compose.swarm.yml` restaura os valores de deploy anteriores. Métricas novas são aditivas (remoção sem
impacto funcional).

## Done-when

- Sampler parent-based ativo; `traceparent` propagado e restaurado.
- `compose.swarm.yml` com `OTEL_SERVICE_VERSION=${IMAGE_TAG}` + `stop_grace_period: 30s` + `stop-first`.
- Gate de CI anti-storm documentado; alerta `status=4 > 0` definido.
- Gate executável de cardinalidade (R-TXN-004 / R-WF-KERNEL-001.4 — deve retornar vazio):
  ```bash
  grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" \
    '"user_id"\|"correlation_key"\|"category_id"' \
    internal/platform/workflow/ internal/platform/outbox/ \
    && echo "FAIL: label de alta cardinalidade em métrica" && exit 1 || true
  ```
- Validação proporcional (Go em cmd/ + engine + outbox): `go build ./...`, `go vet ./cmd/... ./internal/platform/...`, `go test -race -count=1 ./internal/platform/workflow/... ./internal/platform/outbox/...`.

## Arquivos Relevantes
- `cmd/server/server.go`, `cmd/worker/worker.go` (config do provider OTel/sampler)
- `internal/platform/outbox/dispatcher.go`, `internal/platform/outbox/storage_postgres.go` (traceparent no metadata, dead-letter/backoff)
- `internal/agents/infrastructure/messaging/database/consumers/whatsapp_inbound_consumer.go` (restaurar traceparent)
- `internal/platform/workflow/engine.go` (outcome `resumed_on_conflict`, métricas)
- `deployment/compose/compose.swarm.yml` (`stop_grace_period`, `OTEL_SERVICE_VERSION`, deploy)
- Runbook de deploy anti-storm + dashboards Grafana (otel-lgtm)
