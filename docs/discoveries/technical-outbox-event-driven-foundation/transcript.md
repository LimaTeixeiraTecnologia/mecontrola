# Transcript do Discovery Técnico

## Contexto Inicial

### Origem
- Entrada: bundle `discoveries/brainstorm-event-driven-outbox-foundation/` aprovado pelo usuário (status `done`).
- Decisão estratégica já tomada: Alternativa 2 — Polling Two-Table (`outbox_events` + `outbox_deliveries`) com `time.Ticker` 250ms, `robfig/cron/v3` para housekeeping/reaper, `FOR UPDATE SKIP LOCKED`, retry 8× backoff exp+jitter (base 1s, cap 5min), DLQ por delivery, retenção 90d, SLO p95 < 1s, idempotência obrigatória.

### Resumo do estado atual do repositório (fontes verificadas)
1. **Monólito Go 1.26.3** com CLI Cobra (`cmd/server`, `cmd/worker`, `cmd/migrate`); `cmd/worker/worker.go` está idle e pronto para hospedar dispatcher + cron.
2. **Postgres via `pgx/v5 v5.9.2`** já em uso; suporta `FOR UPDATE SKIP LOCKED` e `LISTEN/NOTIFY` nativamente. Migrations via `golang-migrate v4.19.1` em `migrations/` (apenas `0001_init.up.sql` com tabela `health_probe` — sem tabelas de domínio criadas ainda).
3. **OpenTelemetry SDK 1.44** já no `go.mod` — base de métricas/tracing disponível em `internal/infrastructure/observability/` e `internal/telemetry/`.
4. **Eventbus in-process existente** em `internal/infrastructure/events/`: `Bus` tipado via generics com canais bufferizados (default 100), drop em buffer cheio, `Publish[E Event]`, `Subscribe[E Event]`, ADR-003 ("Eventbus tipado via generics + emissão pós-UoW.Commit"). Eventos têm contrato: `Name() EventName`, `OccurredAt()`, `AggregateID()`. Módulos validados: identity, conversation, agent, finance, notifications, telemetry.
5. **Testes de integração** com `testcontainers-go/modules/postgres` já em uso (`cmd_integration_test.go`, `events/bus_integration_test.go`).
6. **Governança** ativa em `AGENTS.md` / `CLAUDE.md` / `.agents/skills/` exigindo carregamento de `agent-governance` + `go-implementation` para qualquer mudança em `.go`.

### Implicação para o discovery
- O Outbox **não substitui** o `events.Bus`; complementa para o subset de eventos que exigem garantias (at-least-once, persistência, retry, DLQ). Decisão de integração precisa ser explicitada na Rodada 1.
- Schema `outbox_events`/`outbox_deliveries` precisa coexistir com a tipagem do `events.Event` atual (nome em formato `<modulo>.<acao>`, `EventID` ULID, `OccurredAt`, `AggregateID`).
- Existe ADR-003 já registrada — nova ADR (ADR-00X — Outbox Transacional) precisa coexistir e ser referenciada.

## Rodada 1 — Objetivo, escopo e criticidade

### P1.1 — Integração com o eventbus existente
Resposta: **Outbox como Publisher alternativo opt-in**.
Implicação: o repositório passa a ter 2 publishers documentados — `events.Bus` (fire-and-forget, drop em buffer cheio, ADR-003) para eventos voláteis e `outbox.Publisher` (at-least-once, persistente, retry+DLQ) para eventos críticos. Desenvolvedor escolhe explicitamente no momento do publish. Necessário ADR nova ("Outbox transacional como Publisher alternativo opt-in") que conviva com ADR-003 sem revogá-la, e uma seção de "quando usar cada um" no AGENTS.md.

### P1.2 — Recorte da primeira entrega
Resposta: **Infraestrutura genérica + handler dummy no `cmd/worker`**, sem acoplar a módulo de negócio.
Implicação: o MVP fecha o pipeline `publish → poll → deliver → retry → DLQ → housekeeping` sem evento de produto real; validação via teste de integração com testcontainers e smoke test em staging. Caso real entra em sprint subsequente (decisão de qual módulo fica em `## Itens em Aberto`).

### P1.3 — Criticidade do domínio
Resposta: **Fundação de plataforma — criticidade derivada por subscription**.
Implicação: SLO e alertas precisam ser parametrizáveis por `subscription_name`. Métricas/labels devem permitir cortar por subscription; runbook precisa documentar como classificar criticidade de cada handler novo; alertas usam threshold por subscription com defaults conservadores (DLQ>0 = warning para qualquer subscription, paging só quando declarado crítico).

### P1.4 — Restrição dominante de capacidade
Resposta: **Sem prazo rígido — qualidade > velocidade, 2 a 3 sprints com folga**.
Implicação: dossiê deve detalhar interfaces Go, schema SQL, ADRs, testes (unitários + integração com testcontainers + teste de concorrência multi-instância), benchmarks. Tasks futuras podem ser pequenas e sequenciadas. Sem corte de escopo para acelerar.

## Rodada 2 — Arquitetura, dados, volumetria e custo

### P2.1 — Layout de pacote
Resposta: **`internal/infrastructure/outbox/`** paralelo a `internal/infrastructure/events/`.
Implicação: novo pacote autocontido com arquivos `publisher.go`, `dispatcher.go`, `storage.go`, `storage_pgx.go`, `registry.go`, `handler.go`, `subscription.go`, `housekeeping.go`, `dlq.go`, `config.go`, `metrics.go`, `errors.go`, `doc.go`. Bootstrap acontece em `internal/infrastructure/runtime/bootstrap.go` (no mode `worker`).

### P2.2 — Abstração de Storage
Resposta: **Interface `Storage` mínima + 1 implementação pgx + mock para unit tests**.
Implicação: contrato pequeno e estável (`InsertEvent`, `InsertDeliveries`, `ClaimReady`, `MarkProcessed`, `MarkFailed`, `MarkDLQ`, `ReleaseStuck`, `PurgeOlderThan`), gerado mock via `mockery v2.53.6` já no projeto. Testes unitários do `Dispatcher` (regras de retry, contagem de attempts, transição para DLQ) rodam sem Postgres; testes de integração com testcontainers cobrem o caminho real.

### P2.3 — Teste de concorrência multi-instância
Resposta: **Testcontainers + N goroutines no mesmo processo simulando réplicas**.
Implicação: teste de integração obrigatório no MVP — populariza N×1000 deliveries pendentes, dispara 3–5 dispatchers concorrentes (cada um com seu pool de conexões), aguarda drenar, assertar `count(processed)=N×1000` e `nenhum delivery com attempts>1 por causa de double-processing` (separar falhas legítimas de retries). Cobertura de SKIP LOCKED virificada empiricamente, não só pela teoria.

### P2.4 — Postura de custo
Resposta: **Equilibrado — 2 a 3 sprints engenharia, zero infra nova**.
Implicação: orçamento estimado 80–120h de engenharia + ~10h DBA/SRE para review e validação. Sem custo de infra (Postgres existente; OTel já no projeto; sem broker). Guardrail de custo de infra: monitorar CPU/IO do Postgres antes/depois (revisar polling se aumento >15%) — registrar como guardrail explícito.

## Rodada 3 — Segurança, confiabilidade e operação

### P3.1 — Baseline de segurança para o payload
Resposta: **Baseline padrão** — payload em texto claro no DB; política documentada de "não incluir segredos no payload"; redaction em logs por allowlist de campos.
Implicação: regra obrigatória no contrato Event/Outbox: payload não carrega senhas/tokens/PII bruta; documentar em AGENTS.md e no godoc de `Publisher`. Logs do dispatcher emitem apenas `event_id`, `event_type`, `subscription_name`, `attempt`, `correlation_id`, `error_class` — nunca payload bruto. LGPD: dado pessoal incluído voluntariamente em payload está sujeito às regras gerais de retenção do produto; 90d do housekeeping fecha o ciclo.

### P3.2 — Estratégia de degradação
Resposta: **Feature flag global do dispatcher via config Viper**.
Implicação: chave `outbox.dispatcher.enabled` (bool, default `true`) lida no bootstrap; quando `false`, o `cmd/worker` registra a goroutine do dispatcher mas ela não inicia o loop de polling. Publish continua funcionando — eventos seguem sendo gravados em `outbox_events`/`outbox_deliveries`, apenas não são entregues. Quando o flag voltar a `true`, dispatcher acorda e drena a fila acumulada. Runbook documenta procedimento de desligar/religar.

### P3.3 — Profundidade de observabilidade
Resposta: **Completa** — métricas (counters + histograms) + traces propagando via headers + logs estruturados + dashboard sugerido na techspec.
Implicação: catálogo de métricas OTel implementado desde o MVP (`outbox.events.published`, `outbox.deliveries.pending`, `outbox.deliveries.processed`, `outbox.deliveries.failed`, `outbox.deliveries.dlq`, `outbox.delivery.latency_ms`, `outbox.poll.duration_ms`, `outbox.poll.batch_size`). Tracing: campo `headers.traceparent` propagado do Publisher ao Handler usando `propagation.TraceContext` do OTel. Logs via `slog` com `slog.Group("outbox", ...)`. Dashboard inicial proposto na techspec com queries Prometheus equivalentes.

### P3.4 — Estratégia de rollout
Resposta: **Feature flag off por padrão no primeiro deploy + ativar em segundo deploy após smoke test**.
Implicação: deploy 1 = código novo + migration `0002_outbox.up.sql` aplicada + flag `outbox.dispatcher.enabled=false`. Smoke test em staging valida publish sem dispatcher rodando (verifica que eventos chegam em `outbox_events`/`outbox_deliveries`). Deploy 2 = flag para `true` em horário de baixa carga; observar métricas por 1h antes de fechar incidente. Rollback = flag para `false` (não exige nova migration); rollback total = `0002_outbox.down.sql` apenas se o esquema causar problema.
