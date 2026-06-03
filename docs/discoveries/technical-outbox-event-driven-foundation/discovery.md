# DOSSIÊ DE DISCOVERY TÉCNICO

## Título
MeControla — Fundação Transactional Outbox para mensageria interna at-least-once

## Resumo Executivo
Contexto:
O monólito Go MeControla já possui um eventbus in-process (`internal/infrastructure/events/Bus`, ADR-003) que entrega eventos via canais com política de drop em buffer cheio — apropriado para sinalizações voláteis, inadequado quando um side-effect crítico precisa sobreviver a falhas e ser entregue após o commit transacional do agregado. Sem uma fundação canônica de Outbox, novas features tendem a implementar side-effects ad-hoc (goroutine fire-and-forget, HTTP inline na transação) que acumulam dívida e expõem o produto a perda silenciosa de eventos.

Recomendação:
Implementar um Outbox transacional com schema two-table em PostgreSQL (`outbox_events` + `outbox_deliveries`), exposto via um novo `outbox.Publisher` opt-in (coexistindo com o `events.Bus` atual), com Dispatcher rodando como goroutine no `cmd/worker` já idle, polling de 250ms via `time.Ticker` e `FOR UPDATE SKIP LOCKED`, retry 8× com backoff exponencial e jitter (base 1s, cap 5min), DLQ por delivery, housekeeping diário de 90d e reaper minuto-a-minuto via `github.com/robfig/cron/v3`. Observabilidade completa via OpenTelemetry desde o MVP. Feature flag global Viper controla ligar/desligar o dispatcher sem afetar publish.

Status de viabilidade:
viável

## Necessidade e Objetivos
Problema atual:
Não existe substrato canônico para entrega assíncrona com garantia at-least-once no MeControla. O `events.Bus` atual descarta eventos quando o canal do subscriber está cheio e não sobrevive a crash do processo. Qualquer feature que dependa de side-effect crítico (notificação, projeção, integração, retry) precisa hoje inventar sua própria solução, o que provocará drift de padrão e perda real de eventos em produção quando o caminho ad-hoc falhar após o commit do agregado.

Objetivos de negócio:
- Habilitar features assíncronas confiáveis (notificações, projeções, integrações futuras) sem bloquear o roadmap por dependência de infraestrutura ausente.
- Reduzir risco operacional de perda silenciosa de eventos críticos, eliminando atalhos ad-hoc por feature.
- Manter custo total de operação baixo: zero infraestrutura nova, sem broker externo, sem novo binário.

Objetivos técnicos:
- Garantir entrega at-least-once de eventos publicados na mesma transação do agregado (atomicidade preservada em SQL).
- Suportar 1×N handlers in-process por evento, com retries e DLQ independentes por handler (granularidade por `subscription_name`).
- Coordenar processamento entre múltiplas réplicas do `cmd/worker` sem broker e sem leader election externo, via `FOR UPDATE SKIP LOCKED`.
- Atender SLO de latência de entrega p95 < 1s para subscriptions internas.
- Entregar em produção um pipeline completo (`publish → poll → deliver → retry → DLQ → housekeeping`) com handler dummy comprovando o ciclo end-to-end.
- Coexistir com o `events.Bus` (ADR-003) sem revogá-lo, oferecendo um Publisher opt-in para os casos que exigem garantia.

## Materiais de Apoio
- Bundle de brainstorming aprovado: `discoveries/brainstorm-event-driven-outbox-foundation/` (decision-brief, scorecard, assumptions, transcript) — fonte da decisão arquitetural Alt 2 (Polling Two-Table).
- Prompt enriquecido original: `docs/prompts/event-driven-outbox-brainstorming.md`.
- Eventbus existente: `internal/infrastructure/events/bus.go`, `event.go`, `bus_integration_test.go` — contrato `Event` (Name, OccurredAt, AggregateID), `EventID` (ULID), `EventName` (`<modulo>.<acao>`), módulos válidos.
- ADR-003 referenciada no godoc de `events`: "Eventbus tipado via generics + emissão pós-UoW.Commit".
- Entrypoint do worker: `cmd/worker/worker.go` (idle, aguarda jobs).
- Bootstrap atual: `internal/infrastructure/runtime/{bootstrap.go,app.go,mode.go}`.
- Migrations: `migrations/embed.go`, `migrations/0001_init.up.sql`/`down.sql` (apenas `health_probe` no estado atual).
- Stack confirmada via `go.mod`: Go 1.26.3, `pgx/v5 v5.9.2`, `golang-migrate v4.19.1`, `go.opentelemetry.io/otel v1.44.0`, `testcontainers-go/modules/postgres v0.42.0`, `mockery/v2 v2.53.6`, `spf13/cobra`, `spf13/viper`.
- Governança: `AGENTS.md`, `CLAUDE.md`, `.claude/rules/governance.md`, skills `agent-governance` e `go-implementation` carregadas obrigatoriamente para qualquer alteração `.go`.

## Escopo
Inclui:
- Novo pacote `internal/infrastructure/outbox/` com `publisher.go`, `dispatcher.go`, `storage.go`, `storage_pgx.go`, `registry.go`, `handler.go`, `subscription.go`, `housekeeping.go`, `dlq.go`, `config.go`, `metrics.go`, `errors.go`, `doc.go`.
- Migration `migrations/0002_outbox.up.sql`/`down.sql` criando `outbox_events`, `outbox_deliveries`, índices e constraints.
- Interface `outbox.Storage` mínima (`InsertEvent`, `InsertDeliveries`, `ClaimReady`, `MarkProcessed`, `MarkFailed`, `MarkDLQ`, `ReleaseStuck`, `PurgeOlderThan`, `Stats`) com implementação `storage_pgx.go` e mock gerado por `mockery v2`.
- `outbox.Publisher` como Publisher opt-in que insere `outbox_events` + N `outbox_deliveries` na mesma transação do agregado (resolução de handlers via Registry resolvido em build time).
- `outbox.Dispatcher` rodando como goroutine no `cmd/worker`, com `time.Ticker` configurável (default 250ms), batch configurável (default 100), retry exponencial com jitter (base 1s, cap 5min, 8 tentativas).
- `outbox.Registry` estático para mapear `event_type` → `[]Subscription` com validação no bootstrap.
- Jobs `robfig/cron/v3`: `housekeeping @daily` (apaga linhas processed e dead_letter com >90d), `reaper @every 1m` (libera linhas claimed com claimed_at < now()-5min).
- Feature flag global `outbox.dispatcher.enabled` em config Viper, com default `true`.
- Instrumentação OpenTelemetry completa (counters, histograms) + tracing propagado via campo `headers.traceparent` + logs `slog` estruturados.
- Dashboard sugerido (queries Prometheus equivalentes) e runbook inicial na techspec.
- Handler dummy exemplo + teste end-to-end em testcontainers comprovando ciclo completo.
- Testes obrigatórios: unitários do Dispatcher com mock de Storage; integração com testcontainers Postgres; teste de concorrência multi-instância (3–5 dispatchers no mesmo Postgres assertam zero double-processing).
- ADR nova ("Outbox transacional como Publisher opt-in") coexistindo com ADR-003.
- Atualizações em `AGENTS.md` documentando quando usar `events.Bus` vs. `outbox.Publisher` e a regra obrigatória de idempotência de handler.

Exclui:
- Broker externo (RabbitMQ, Kafka, NATS, SNS/SQS) no MVP — substrato deixado pronto para evolução V2 mantendo contrato de Publisher desacoplado.
- Integrações HTTP/webhooks externos como handler — MVP apenas handlers in-process.
- CDC/Debezium/leitura via WAL.
- LISTEN/NOTIFY — evolução condicionada a métrica de p99 degradar acima de 2s consistentemente.
- Ordenação global FIFO entre `event_types` distintos; ordem apenas por `aggregate_id` dentro de um batch via `ORDER BY id`.
- Schema registry (Avro/Protobuf); payloads JSONB opacos com `event_version` controlado pelo producer.
- Particionamento por advisory lock (Alternativa 4 do scorecard) — over-engineering vs. escopo aprovado.
- Configurabilidade de retry policy por subscription no MVP — política global no início; vira evolução futura.
- Substituição ou deprecação do `events.Bus` (ADR-003 preservada).
- Acoplamento a módulo de negócio na primeira entrega — handler dummy apenas; caso real fica em sprint seguinte.

## Premissas e Restrições
Premissas:
- O `cmd/worker` permanecerá responsável por hospedar workers internos do produto e tolera 2 novas goroutines (Dispatcher + Cron) sem impacto em cargas atuais (atualmente idle).
- Postgres em produção tem capacidade ociosa suficiente para receber polling de ~4 qps por instância em fila vazia e tráfego de escrita +1 linha por handler ativo na transação de publish, sem degradar p95 das APIs transacionais — validar com `pg_stat_statements` pós-deploy.
- Handlers serão escritos sob a regra obrigatória de idempotência por `event_id`; código-revisor garantirá via PR template.
- Retenção de 90d atende às exigências regulatórias atuais do produto — pendente validação com Legal/Compliance antes do deploy em ambientes regulados.
- Volumetria do primeiro ano permanece na faixa média (10–100 ev/s, ~1M deliveries/dia); cenários com >500 ev/s acionam re-discovery para broker externo ou particionamento.
- A coexistência de `events.Bus` (volátil) e `outbox.Publisher` (persistente) será documentada e divulgada para o time evitar uso incorreto.

Restrições:
- Backend persistente fixo: PostgreSQL no mesmo schema/DB do agregado (atomicidade transacional é não-negociável).
- Sem dependência de infraestrutura nova (sem broker, sem coordenador externo, sem cache distribuído).
- Scheduler periódico obrigatoriamente `github.com/robfig/cron/v3` (versão estável v3 atual).
- Dispatcher em goroutine do `cmd/worker` existente — sem novo binário.
- Coordenação multi-instância apenas via `SELECT ... FOR UPDATE SKIP LOCKED`.
- Política de retry global: 8 tentativas, backoff exponencial com jitter (base 1s, cap 5min); transição automática para `dead_letter` após esgotar.
- SLO formal: p95 < 1s; p99 ocasional 1–2s aceitável.
- Idempotência de handler é regra obrigatória de implementação.
- Sem prazo rígido — qualidade prevalece sobre velocidade; entrega em 2–3 sprints com folga.

## Viabilidade Técnica
Status:
viável

Justificativa:
Todas as primitivas exigidas estão presentes e exercitadas no projeto: PostgreSQL via `pgx/v5` (suporta nativamente `FOR UPDATE SKIP LOCKED`), `golang-migrate` para schema, `testcontainers-go/modules/postgres` para teste de integração com banco real, OpenTelemetry SDK 1.44 para métricas/tracing, `cmd/worker` idle pronto para hospedar goroutines, `mockery v2` para gerar mocks da interface `Storage`. O eventbus existente serve como base para naming e contratos de evento (`<modulo>.<acao>`, ULID), e a nova ADR coexistirá com a ADR-003 sem revogá-la. Não há dependência externa nova nem mudança de stack. Volumetria alvo (10–100 ev/s) está confortavelmente dentro do que polling com SKIP LOCKED entrega em Postgres bem-indexado.

Bloqueadores:
- Nenhum bloqueador técnico identificado.
- Validação com Legal/Compliance sobre retenção 90d é prerrequisito de deploy em ambientes regulados, não de implementação.

## Arquitetura Atual
- Monólito Go com CLI Cobra (`cmd/server`, `cmd/worker`, `cmd/migrate`); `cmd/worker/worker.go` está idle aguardando jobs.
- Bootstrap centralizado em `internal/infrastructure/runtime/bootstrap.go` com modes `server` e `worker` definidos em `mode.go`.
- Eventbus in-process em `internal/infrastructure/events/`: `Bus` tipado via generics, canais bufferizados (default 100), drop em buffer cheio com warn em log, `Publish[E Event]`/`Subscribe[E Event]`/`Close(ctx)`. ADR-003 registra "emissão pós-UoW.Commit". Sem persistência, sem retry, sem DLQ.
- Persistência via `pgx/v5`; migrations carregadas via `migrations/embed.go`; apenas `0001_init.up.sql` aplicado (tabela `health_probe` para health-check).
- Observabilidade: pacote `internal/infrastructure/observability/` e `internal/telemetry/` com adapters OTel — usados pelos módulos atuais para métricas e traces.
- Módulos de negócio em estágio inicial: `internal/{identity,conversation,agent,finance,notifications,telemetry}/` sem tabelas próprias migradas ainda.
- Testes de integração com testcontainers já estabelecidos: `cmd_integration_test.go`, `internal/infrastructure/events/bus_integration_test.go`.

## Arquitetura Proposta
Componentes:
- `outbox.Event` — struct serializável com `ID` (ULID), `Type` (alias de `events.EventName`), `Version uint16`, `AggregateType`, `AggregateID`, `PartitionKey`, `Payload []byte` (JSON serializado), `Headers map[string]string` (inclui `traceparent`, `correlation_id`, `causation_id`), `OccurredAt time.Time`.
- `outbox.Subscription` — `{ Name string; EventType events.EventName; Handler Handler }`. Resolvida em build time via `Registry.Register`.
- `outbox.Handler` — `func(ctx context.Context, evt Event) error`. Erro = retry; `errors.Is(err, outbox.ErrPermanent)` = transição imediata para DLQ.
- `outbox.Registry` — mapeia `event_type → []Subscription`; método `SubscriptionsFor(eventType)` consultado pelo Publisher para criar deliveries.
- `outbox.Publisher` — interface `Publish(ctx, tx pgx.Tx, evt Event) error`. Recebe a `pgx.Tx` da transação do agregado, insere `outbox_events` + N `outbox_deliveries` (uma por subscription registrada para `evt.Type`) na mesma transação.
- `outbox.Storage` — interface com `InsertEvent`, `InsertDeliveries`, `ClaimReady(ctx, batchSize, instanceID) ([]Claim, error)`, `MarkProcessed`, `MarkFailed`, `MarkDLQ`, `ReleaseStuck(ctx, olderThan)`, `PurgeOlderThan(ctx, olderThan)`, `Stats(ctx) (Stats, error)`. Implementação concreta `storage_pgx.go`.
- `outbox.Dispatcher` — loop principal: `time.Ticker` configurável (default 250ms) → `Storage.ClaimReady(batch)` em transação curta → executa `Handler` para cada Claim com timeout configurável → `MarkProcessed` ou calcula próximo retry e `MarkFailed`/`MarkDLQ`. Conta tentativas, aplica backoff exponencial com jitter.
- `outbox.Housekeeping` — wrapper para `robfig/cron/v3` registrando dois jobs: `@daily` chama `Storage.PurgeOlderThan(90d)`; `@every 1m` chama `Storage.ReleaseStuck(5m)`.
- `outbox.Metrics` — fachada OTel com `events_published_total`, `deliveries_pending`, `deliveries_processed_total`, `deliveries_failed_total`, `deliveries_dlq_total`, `delivery_latency_ms`, `poll_duration_ms`, `poll_batch_size`.
- `outbox.Config` — bind Viper para `outbox.dispatcher.enabled`, `outbox.dispatcher.tick_interval`, `outbox.dispatcher.batch_size`, `outbox.dispatcher.handler_timeout`, `outbox.retry.max_attempts`, `outbox.retry.base_backoff`, `outbox.retry.max_backoff`, `outbox.housekeeping.retention_days`, `outbox.reaper.stuck_after`.
- Bootstrap em `internal/infrastructure/runtime/bootstrap.go` (modo `worker`): cria `Storage`, `Registry`, `Dispatcher`, `Housekeeping`; inicia goroutines respeitando `Config.Enabled`; respeita `Shutdown` para drenagem graceful.

Fluxo de alto nível:
1. Use case do agregado abre transação `pgx.Tx` (via `UnitOfWork` existente do projeto).
2. Use case chama `outbox.Publisher.Publish(ctx, tx, evt)` antes do commit. Publisher consulta `Registry.SubscriptionsFor(evt.Type)` e insere `outbox_events(...)` + N `outbox_deliveries(status='pending', next_retry_at=now())` na mesma `tx`.
3. Use case faz `tx.Commit()` — evento e deliveries ficam atomicamente persistidos com o estado do agregado.
4. Dispatcher (goroutine no `cmd/worker`) acorda a cada 250ms e roda `Storage.ClaimReady(batch=100, instanceID)`: `UPDATE outbox_deliveries SET status='claimed', claimed_at=now(), claimed_by=$1 WHERE id IN (SELECT id FROM outbox_deliveries WHERE status='pending' AND next_retry_at <= now() ORDER BY id LIMIT $2 FOR UPDATE SKIP LOCKED) RETURNING ...`.
5. Para cada Claim, Dispatcher executa o `Handler` correspondente com timeout configurável. Trace OTel é continuado a partir de `headers.traceparent`.
6. Resultado:
   - Sucesso → `MarkProcessed(id, processed_at=now())` + métrica `deliveries_processed_total{subscription}` + histograma `delivery_latency_ms{subscription}`.
   - Erro transitório → calcular `next_retry = now() + min(base * 2^attempts + jitter, max)`; `MarkFailed(id, last_error, attempts+1, next_retry_at)`; se `attempts+1 >= 8`, transição para `MarkDLQ(id, dead_letter_at=now(), last_error)` + métrica `deliveries_dlq_total{subscription}`.
   - Erro permanente (`errors.Is(err, ErrPermanent)`) → `MarkDLQ` imediatamente.
7. Reaper (cron @every 1m) executa `ReleaseStuck(now()-5m)`: `UPDATE outbox_deliveries SET status='pending', claimed_by=NULL, claimed_at=NULL WHERE status='claimed' AND claimed_at < $1` — recupera linhas presas por crash de worker.
8. Housekeeping (cron @daily) executa `PurgeOlderThan(90d)`: `DELETE FROM outbox_deliveries WHERE status IN ('processed','dead_letter') AND COALESCE(processed_at, dead_letter_at) < now()-interval '90 days'` + delete cascateado de `outbox_events` sem deliveries restantes.

Decisão arquitetural:
Adotar Outbox como Publisher opt-in paralelo ao `events.Bus` existente (ADR-003 preservada). Schema two-table (`outbox_events` imutável + `outbox_deliveries` por handler) com `FOR UPDATE SKIP LOCKED` para coordenação multi-instância sem broker. Polling com `time.Ticker` 250ms para entrega; `robfig/cron/v3` apenas para tarefas periódicas operacionais (housekeeping diário, reaper minuto-a-minuto). LISTEN/NOTIFY é evolução futura condicionada a métrica. Particionamento por advisory lock descartado por over-engineering. Feature flag global Viper habilita degradação operacional sem mexer no caminho de publish. Observabilidade OTel completa desde o MVP.

## Dados e Integrações
Domínios de dados:
- `outbox_events` (imutável após insert): `id UUID/ULID PK`, `event_type TEXT NOT NULL`, `event_version SMALLINT NOT NULL DEFAULT 1`, `aggregate_type TEXT NOT NULL`, `aggregate_id TEXT NOT NULL`, `partition_key TEXT`, `payload JSONB NOT NULL`, `headers JSONB NOT NULL DEFAULT '{}'`, `correlation_id TEXT`, `causation_id TEXT`, `created_at TIMESTAMPTZ NOT NULL DEFAULT now()`.
- `outbox_deliveries`: `id UUID/ULID PK`, `event_id UUID/ULID NOT NULL REFERENCES outbox_events(id) ON DELETE CASCADE`, `subscription_name TEXT NOT NULL`, `status TEXT NOT NULL CHECK (status IN ('pending','claimed','processed','dead_letter'))`, `attempts SMALLINT NOT NULL DEFAULT 0`, `next_retry_at TIMESTAMPTZ NOT NULL DEFAULT now()`, `last_error TEXT`, `processed_at TIMESTAMPTZ`, `dead_letter_at TIMESTAMPTZ`, `claimed_at TIMESTAMPTZ`, `claimed_by TEXT`, `created_at TIMESTAMPTZ NOT NULL DEFAULT now()`, `updated_at TIMESTAMPTZ NOT NULL DEFAULT now()`. Constraint de unicidade `(event_id, subscription_name)` para idempotência do publish.
- Índices: `outbox_deliveries (status, next_retry_at, id)` para o claim; `outbox_deliveries (subscription_name, status)` para queries operacionais; `outbox_deliveries (status, COALESCE(processed_at, dead_letter_at))` parcial para housekeeping; `outbox_deliveries (status, claimed_at)` parcial para reaper.

Integrações:
- pgx/v5: única dependência de runtime do Outbox. Usa `pgx.Tx` recebida do `UnitOfWork` existente para garantir atomicidade do publish.
- robfig/cron/v3: scheduler de jobs periódicos (housekeeping, reaper). Versão a ser fixada na techspec.
- OpenTelemetry SDK 1.44: instrumentação de métricas e tracing. Tracing propaga via `headers.traceparent` no evento.
- spf13/viper: bind do `outbox.Config`.
- mockery/v2: gera mock de `outbox.Storage` para testes unitários do Dispatcher.
- testcontainers-go/modules/postgres: usado nos testes de integração e teste de concorrência multi-instância.

Consistência requerida:
forte (publish e deliveries inseridos na mesma transação `pgx.Tx` do agregado; commit do agregado garante presença das linhas no Outbox).

## Volumetria e Capacidade
Volume atual:
zero — feature nova. Cálculo baseado em projeção do produto: tráfego de eventos do MeControla esperado para o primeiro ano.

Pico esperado:
100 eventos/s sustentados, picos curtos de 200 eventos/s. Com média de 1–3 handlers por evento, isto traduz para 100–600 deliveries/s no pico e ~1M deliveries/dia em regime médio.

Taxa de crescimento:
2–3× ano-sobre-ano nos primeiros 2 anos, conforme novos módulos do produto adotem o Outbox. Cenário de crescimento que ultrapasse 500 ev/s sustentado dispara re-discovery para broker externo ou particionamento por shard.

SLO alvo:
Latência de entrega p95 < 1s e p99 < 2s (mantendo p95 dentro do alvo). Disponibilidade do dispatcher 99,9% mensal (limite herdado do worker). Backlog de pending por subscription < 10×TPS médio em condições normais.

Gargalos conhecidos:
- Query de polling (`SELECT ... FOR UPDATE SKIP LOCKED LIMIT 100`) sobre `outbox_deliveries(status, next_retry_at, id)` — mitigado por índice composto e batch fixo.
- Escrita transacional adicional (+1 linha por handler) no caminho de publish — monitorar p95 do publish após go-live; pode exigir batching em V2 se número médio de handlers crescer acima de 3.
- Crescimento sem limites se housekeeping falhar — mitigado por métrica `deliveries_pending`/`deliveries_total_count` exposta + alerta de capacidade.
- Pressão de polling em fila vazia (~4 qps por instância) — aceitável para 1–3 réplicas; revisar intervalo ou polling adaptativo se >5 réplicas.

## Segurança e Compliance
Classificação dos dados:
Payload JSONB pode carregar dados de negócio que incluam PII (nomes, e-mails, dados financeiros) quando o publisher decidir. Sem segredos (senhas, tokens, chaves) — regra explícita do contrato. Dados são internos ao produto, não saem do DB exceto via handlers in-process autorizados.

Autenticação e autorização:
Dispatcher e Publisher rodam dentro do binário do produto com a mesma credencial do app no Postgres — sem nova superfície de autenticação externa. Não há endpoint de rede exposto pelo Outbox. Autorização para escrever em `outbox_events`/`outbox_deliveries` herda do role usado pela aplicação; role separado para `cmd/migrate` aplicar DDL é desejável (já é padrão do projeto).

Gestão de segredos:
Outbox não introduz novos segredos. Conexão Postgres usa as credenciais já gerenciadas pelo projeto (Viper + variáveis de ambiente). Política reforçada no godoc do Publisher: payload nunca deve conter segredos.

Criptografia:
Em trânsito: TLS já configurado na conexão Postgres do projeto (não é responsabilidade do Outbox alterar). Em repouso: sem criptografia adicional no MVP (baseline padrão); depende da criptografia de disco do banco gerenciado. Decisão revisitável se requisito regulatório exigir envelope encryption.

Auditoria e rastreabilidade:
`outbox_events` é tabela imutável após publish, funcionando como log auditável das emissões durante 90d. `outbox_deliveries.last_error` registra causa da última falha. Traces OTel propagados via `headers.traceparent` permitem reconstruir o caminho publisher→handler em ferramentas de observabilidade. Logs estruturados via `slog` incluem `event_id`, `event_type`, `subscription_name`, `attempt`, `correlation_id`, `error_class` — nunca payload bruto.

Compliance/LGPD:
Retenção de 90d via housekeeping fecha o ciclo de vida do dado pessoal incluído voluntariamente em payload — alinhado com prática geral de minimização. Validação obrigatória com Legal/Compliance antes do deploy em ambientes regulados (registrado em `## Itens em Aberto`). Direito de eliminação requer purge sob demanda para um `aggregate_id` específico — não implementado no MVP; runbook documenta query manual de purge quando demandado.

## Confiabilidade e Resiliência
SLA/SLO:
Latência de entrega p95 < 1s e p99 < 2s, calculados sobre `delivery_latency_ms{subscription}`. Disponibilidade do dispatcher 99,9% mensal. Taxa de DLQ por subscription < 0,1% das deliveries totais em janela de 24h (acima disso, indica problema no handler).

RTO/RPO:
RTO do dispatcher: 5 minutos (tempo para reaper liberar claims-stuck após crash de worker). RPO: zero para eventos publicados antes do commit (atomicidade SQL); zero para eventos commitados (sobrevivem em `outbox_events`/`outbox_deliveries` até processados ou explicitamente purgados).

Estratégia de retry/idempotência:
Política global: 8 tentativas com backoff exponencial e jitter (`next_retry = now() + min(base * 2^attempts * (0.5 + rand), max)`, base=1s, max=5min). Janela total no pior caso ~30–60min. Após 8 falhas, transição para `dead_letter` com `dead_letter_at` preenchido. Erro permanente do handler (`errors.Is(err, outbox.ErrPermanent)`) força transição imediata para DLQ. Idempotência é regra obrigatória de implementação do handler — chave canônica = `event.ID` (ULID); documentada no godoc de `outbox.Handler` e em `AGENTS.md`.

Degradação/contingência:
Feature flag global `outbox.dispatcher.enabled` (default `true`) via Viper. Quando `false`, dispatcher não inicia o loop de polling; publish continua funcionando (eventos vão para o banco e ficam aguardando). Religar flag drena fila acumulada. Falha de DB derruba publish junto (acoplamento natural ao DB compartilhado) — sem fallback in-memory (quebraria at-least-once). Pico de carga não controlado é mitigado por batch fixo (100) e pode acumular `pending` controladamente até alerta disparar.

Rollback:
Deploy 1 (código + migration + flag `false`): rollback = revert do código; migration mantida em produção (idempotente, `CREATE TABLE IF NOT EXISTS` + índices). Deploy 2 (flag `true`): rollback = flag para `false` via config (sem necessidade de redeploy se config for live-reload; com redeploy se config requer restart). Rollback de schema (`0002_outbox.down.sql`) reservado para caso extremo de problema estrutural — só após drenar/exportar dados pendentes para evitar perda.

## Observabilidade e Operação
Métricas:
- `outbox.events.published.total` (counter, labels: `event_type`).
- `outbox.deliveries.pending` (gauge, labels: `subscription_name`) — coletada via query periódica.
- `outbox.deliveries.processed.total` (counter, labels: `subscription_name`).
- `outbox.deliveries.failed.total` (counter, labels: `subscription_name`, `error_class`).
- `outbox.deliveries.dlq.total` (counter, labels: `subscription_name`).
- `outbox.delivery.latency_ms` (histogram, labels: `subscription_name`) — mede `now() - event.OccurredAt` na conclusão.
- `outbox.poll.duration_ms` (histogram) — mede custo do query de claim.
- `outbox.poll.batch_size` (histogram) — mede quantos itens vieram por ciclo.
- `outbox.reaper.released.total` (counter) — quantas linhas o reaper soltou em cada execução.
- `outbox.housekeeping.deleted.total` (counter) — quantas linhas o housekeeping apagou.

Logs:
- `slog.InfoContext` no startup: `outbox.dispatcher.started` com `tick_interval`, `batch_size`, `instance_id`.
- `slog.InfoContext` em sucesso (sampled): `outbox.delivery.processed` com `event_id`, `event_type`, `subscription_name`, `attempt`, `latency_ms`, `correlation_id`.
- `slog.WarnContext` em falha transitória: `outbox.delivery.failed` com `event_id`, `subscription_name`, `attempt`, `error`, `next_retry_at`.
- `slog.ErrorContext` em transição para DLQ: `outbox.delivery.dlq` com `event_id`, `subscription_name`, `total_attempts`, `error`.
- `slog.WarnContext` no reaper: `outbox.reaper.released` com `count`, `older_than`.
- `slog.InfoContext` no housekeeping: `outbox.housekeeping.purged` com `count`, `retention_days`.
- Política: payload nunca aparece em logs; redaction garantida pelo encoder de log.

Traces:
- Publisher cria span `outbox.publish` com atributos `event.type`, `event.id`, `aggregate.id`, `subscriptions.count`; injeta `traceparent` em `event.headers`.
- Dispatcher cria span `outbox.deliver` por delivery com atributos `event.type`, `subscription_name`, `attempt`; extrai contexto pai de `event.headers.traceparent` e usa como parent, conectando publisher → handler.
- Handler executa dentro do span filho `outbox.handle.<subscription>`.

Alertas:
- `outbox.deliveries.dlq.total > 0` em janela de 5min — severity `warning` por padrão; `critical` para subscriptions explicitamente marcadas como críticas (label `criticality=high`).
- `outbox.deliveries.pending > 10×TPS médio` em janela de 5min — severity `critical` (indica dispatcher parado ou backlog crescente).
- `histogram_quantile(0.95, outbox.delivery.latency_ms) > 1s` em janela de 15min — severity `warning`.
- `histogram_quantile(0.99, outbox.delivery.latency_ms) > 2s` em janela de 30min — severity `warning` (gatilho para discussão de promover LISTEN/NOTIFY).
- `outbox.reaper.released.total > N` repetidamente — severity `warning` (indica crashes recorrentes do worker).
- `outbox.housekeeping.deleted.total = 0` por 48h — severity `critical` (housekeeping pode estar parado).

Dashboards/Runbooks:
- Dashboard inicial proposto na techspec com 6 painéis: pending por subscription, latência p95/p99 por subscription, taxa de processed/s por subscription, DLQ count por subscription, idade do registro mais antigo pendente, atividade do reaper/housekeeping. Queries Prometheus equivalentes a serem listadas explicitamente.
- Runbook inicial cobre: como desligar/religar dispatcher via flag, como inspecionar DLQ por subscription, como re-enfileirar manualmente uma delivery do DLQ (`UPDATE outbox_deliveries SET status='pending', attempts=0, next_retry_at=now() WHERE id=$1`), como purgar dados de um aggregate por demanda LGPD, como diagnosticar handler em loop (correlate `attempts` crescente com `last_error`), como agir em incidente de pending crescente.

## Performance e Escalabilidade
Latência alvo:
p95 entrega < 1s, p99 entrega < 2s. Componentes do orçamento de latência:
- Polling: 0–250ms (uma janela de Ticker).
- Claim transação: 10–50ms típico em índice composto.
- Handler: variável; alvo < 500ms para handlers internos típicos.
- Mark transação: 10–30ms.

Estratégia de escala:
Horizontal linear no número de réplicas do `cmd/worker` graças a `SKIP LOCKED` (sem leader election). Vertical via batch size (default 100, ajustável). Sem broker; quando volumetria justificar (>500 ev/s sustentado), promover para LISTEN/NOTIFY (sem mudança de schema) ou broker externo (mantendo contrato `Publisher`).

Limites conhecidos:
- ~500–1000 deliveries/s antes de pressionar polling e exigir LISTEN/NOTIFY ou broker.
- Número médio de handlers por evento acima de 5 começa a pressionar caminho de publish — exigirá batching de insert.
- Réplicas do worker acima de 5 começam a tornar polling ~20 qps em fila vazia — exigirá polling adaptativo ou LISTEN/NOTIFY.

Teste de carga:
Benchmark obrigatório na techspec antes do go-live: publish loop com 1, 10, 100 e 1000 publishes/s medindo p95 do publish; dispatcher drenando 1M deliveries pré-populadas medindo throughput sustentado e p95 de entrega; teste de concorrência com 5 dispatchers paralelos validando zero double-processing.

## Custos e Orçamento
Orçamento estimado:
80–120 horas de engenharia (1 dev sênior, 2–3 sprints) + ~10 horas de DBA/SRE para review e validação de plano de schema e métricas + ~5 horas de Legal/Compliance para validar retenção 90d. Sem custo direto de infraestrutura no MVP.

Drivers de custo:
- Hora-engenharia para implementação, testes (incluindo concorrência), instrumentação OTel, ADR, runbook e benchmark.
- Storage adicional no Postgres: aproximadamente 90GB em regime médio (1M deliveries/dia × ~1KB/linha × 90d), bem dentro do dimensionamento típico do produto.
- IO/CPU do Postgres com polling + escrita extra: estimado < 15% de aumento sobre baseline em volumetria média; sem custo direto até o limite do plano atual.
- Custo de oportunidade: time alocado não trabalha em features de produto durante o esforço.

Guardrails de custo:
- Monitorar CPU e IO do Postgres antes e depois do go-live; se aumento sustentado > 15% sobre baseline, revisar intervalo de polling ou batch.
- Métrica `deliveries_total_count` exposta; alerta se atravessar 2× o esperado para 90d (indica housekeeping falhando ou retenção mal calibrada).
- Cap de handlers por evento monitorado em métrica `subscriptions_per_event_type`; review obrigatório se algum event_type ultrapassar 5 handlers.

Plano de otimização:
- Ajustar `outbox.dispatcher.tick_interval` para cima em ambientes de baixa volumetria (ex.: staging com 1s) reduzindo carga de polling.
- Aumentar `outbox.dispatcher.batch_size` para 200–500 se p95 de polling crescer.
- Migrar para polling adaptativo (cresce intervalo quando vazio, reduz quando ativo) como otimização V1.5 se carga de polling se mostrar significativa.
- Promover para LISTEN/NOTIFY como evolução V2 condicionada a p99 > 2s consistente.

## Riscos e Mitigações
- Risco: contenção de escrita extra no caminho de publish degradar p95 de endpoints transacionais (H5 do brainstorming).
  Impacto: aumento de p95 em APIs críticas; experiência do usuário degradada.
  Mitigação: benchmark obrigatório no MVP; índices adequados em `outbox_deliveries`; monitorar `pg_stat_statements` por 7 dias após go-live; batching de insert reservado para V2 se número médio de handlers > 3.
  Dono: tech lead backend.
- Risco: handler não-idempotente causa side-effects duplicados em retry (H6).
  Impacto: cobranças, notificações, integrações inconsistentes — pode chegar ao usuário final.
  Mitigação: regra obrigatória documentada no godoc de `outbox.Handler` e em `AGENTS.md`; PR template com checklist de idempotência; chave canônica `event_id` exposta no contrato; review pareado obrigatório para primeiras subscriptions reais.
  Dono: tech lead da área que criar a subscription.
- Risco: housekeeping falha silenciosamente, tabela cresce indefinidamente, índices incham e degradam polling.
  Impacto: latência geral do Postgres; eventual indisponibilidade do dispatcher.
  Mitigação: métrica `outbox.housekeeping.deleted.total` exposta; alerta `=0 por 48h`; runbook com query de purge manual; monitorar `deliveries_total_count`.
  Dono: SRE/DBA.
- Risco: polling agressivo de múltiplas réplicas pressionar CPU do Postgres (H8).
  Impacto: latência geral do DB; degradação de outras cargas.
  Mitigação: começar com 1–3 réplicas; monitorar `pg_stat_statements`; permitir ajuste de `tick_interval` via config; considerar polling adaptativo ou LISTEN/NOTIFY se necessário.
  Dono: SRE.
- Risco: retenção 90d incompatível com requisito regulatório de auditoria (H7).
  Impacto: violação de compliance; perda de evidência para auditoria.
  Mitigação: validar com Legal/Compliance antes do deploy em ambientes regulados; alternativa = arquivar em cold storage antes de purgar (sem implementação no MVP).
  Dono: Tech lead + Compliance.
- Risco: uso incorreto pela equipe (escolher `events.Bus` quando deveria usar `outbox.Publisher` para evento crítico).
  Impacto: perda de evento crítico em produção quando buffer encher ou processo crashar.
  Mitigação: critérios de escolha documentados em `AGENTS.md`; godoc dos dois Publishers com exemplos contrastantes; revisão técnica obrigatória para qualquer Publish novo.
  Dono: tech lead backend.
- Risco: ADR-003 e ADR nova entrarem em conflito conceitual no futuro, criando ambiguidade.
  Impacto: drift de padrão; novos devs ficam sem direção clara.
  Mitigação: ADR nova referencia ADR-003 explicitamente e estabelece quando cada caminho prevalece; revisão periódica das ADRs como parte do ciclo de governança.
  Dono: tech lead backend.

## Trade-offs e Decisões
Alternativas consideradas:
- Polling Single-Table com fan-out agregado (Alt 1 do scorecard) — descartada por perda de observabilidade granular por handler.
- Polling Two-Table (Alt 2 — recomendada e adotada).
- LISTEN/NOTIFY com Polling Fallback (Alt 3) — descartada para o MVP por complexidade não justificada pelo SLO atual; promovida a evolução V2.
- Polling Particionado por hash de partition_key (Alt 4) — descartada por over-engineering vs. escopo aprovado (FIFO global fora-de-escopo).

Decisão tomada:
Alternativa 2 — Polling Two-Table (`outbox_events` + `outbox_deliveries`) com `time.Ticker` 250ms, `robfig/cron/v3` para housekeeping (`@daily`) e reaper (`@every 1m`), `FOR UPDATE SKIP LOCKED` para coordenação multi-instância, retry 8× com backoff exponencial + jitter, DLQ por delivery, retenção 90d, feature flag global Viper, observabilidade OTel completa, Publisher opt-in coexistindo com `events.Bus`.

Trade-off aceito:
+1 escrita transacional por handler no caminho de publish em troca de observabilidade granular, retries independentes e DLQ por delivery; latência média de ~250ms (uma janela de Ticker) ao preferir simplicidade operacional sobre LISTEN/NOTIFY; janela total de retry até ~60min para reduzir pressão em DLQ durante manutenções curtas; p99 ocasional 1–2s desde que p95 < 1s seja mantido. Ônus de idempotência fica no desenvolvedor da subscription.

## Plano de Entrega e Rollout
Fases:
- Fase 1 (sprint 1): schema + migrations + Storage pgx + tipos base (`Event`, `Subscription`, `Handler`, `Registry`, `Config`, `errors`). Testes unitários do Storage com testcontainers. ADR escrita e aprovada.
- Fase 2 (sprint 2): Publisher + Dispatcher + retry/DLQ + métricas OTel + logs slog. Testes unitários do Dispatcher com mock de Storage. Teste de integração end-to-end com testcontainers (publish → dispatcher → handler dummy → processed). Benchmark básico.
- Fase 3 (sprint 3): Housekeeping + Reaper via `robfig/cron/v3`. Teste de concorrência multi-instância (3–5 dispatchers paralelos). Bootstrap no `cmd/worker` com feature flag. Runbook + dashboard sugerido na techspec. ADR finalizada. Documentação em `AGENTS.md`.

Migração:
Migration `0002_outbox.up.sql` cria tabelas e índices; idempotente via `CREATE TABLE IF NOT EXISTS`. Migration `0002_outbox.down.sql` reservada para rollback estrutural extremo. Aplicada via `cmd/migrate` no pipeline de deploy padrão do projeto. Sem migração de dados existentes (feature nova).

Feature flags/canary:
Feature flag global `outbox.dispatcher.enabled` em config Viper, default `false` no primeiro deploy em produção. Após smoke test em staging com flag `true` validando o ciclo completo, deploy 2 ativa o flag em produção em horário de baixa carga e observa métricas por 1h antes de fechar incidente. Sem canary por réplica no MVP — todas as réplicas do `cmd/worker` recebem o mesmo flag.

Critério de rollback:
- Imediato (flag `false`): se p95 de publish degradar > 20% sobre baseline; se DLQ disparar inesperadamente; se `pg_stat_statements` mostrar pressão anômala. Sem necessidade de redeploy se config for live-reload.
- Estrutural (`0002_outbox.down.sql`): apenas se problema de schema for confirmado e dados pendentes puderem ser exportados/descartados sem perda funcional.
- Código (revert): se bug funcional do Dispatcher for descoberto pós-deploy 2; flag para `false` mitiga até o revert ser aplicado.

## Decomposição em Épicos e Features
### Epic 01 - Schema e fundação de persistência do Outbox
Objetivo: estabelecer schema two-table no Postgres com índices, constraints e migrations reversíveis, mais a camada `Storage` com implementação pgx e mock para uso por testes unitários do Dispatcher.
Feature 01: Migration `0002_outbox.up.sql` e `down.sql` criando `outbox_events`, `outbox_deliveries`, índices e constraints de unicidade
Feature 02: Interface `outbox.Storage` com contratos mínimos e mock gerado por mockery
Feature 03: Implementação `outbox.storage_pgx.go` com testes de integração via testcontainers Postgres

### Epic 02 - Publisher transacional e Registry de subscriptions
Objetivo: oferecer um Publisher opt-in que insere evento e deliveries na mesma transação do agregado, com Registry estático resolvido em build time e validação no bootstrap.
Feature 01: Tipo `outbox.Event` (ULID, headers, payload JSONB, metadados) e contrato `outbox.Handler`
Feature 02: `outbox.Registry` com `Register`/`SubscriptionsFor` e validação de duplicidade
Feature 03: `outbox.Publisher` com `Publish(ctx, tx, evt)` e testes unitários cobrindo atomicidade do insert

### Epic 03 - Dispatcher com polling, retry, DLQ e coordenação multi-instância
Objetivo: implementar o loop principal do Dispatcher com claim via SKIP LOCKED, execução de handlers, retry com backoff exponencial + jitter, DLQ por delivery e suporte a múltiplas réplicas sem broker.
Feature 01: Loop principal do Dispatcher com `time.Ticker` configurável e claim em batch
Feature 02: Política de retry (8 tentativas, backoff exp + jitter, transição para DLQ) e tratamento de `outbox.ErrPermanent`
Feature 03: Teste de concorrência multi-instância com 3–5 dispatchers paralelos validando zero double-processing
Feature 04: Testes unitários do Dispatcher com mock de Storage cobrindo regras de retry, DLQ e timeout de handler

### Epic 04 - Housekeeping e reaper via robfig/cron/v3
Objetivo: garantir retenção de 90d para registros finalizados e recuperação de claims-stuck após crash, via jobs `robfig/cron/v3` rodando como segunda goroutine no `cmd/worker`.
Feature 01: Job de housekeeping `@daily` chamando `Storage.PurgeOlderThan(90d)` com métrica
Feature 02: Job de reaper `@every 1m` chamando `Storage.ReleaseStuck(5m)` com métrica
Feature 03: Integração via wrapper `outbox.Housekeeping` com bootstrap controlado pela config

### Epic 05 - Observabilidade OpenTelemetry, logs estruturados e tracing
Objetivo: instrumentar o Outbox com métricas, logs e tracing suficientes para operação, oncall e investigação de incidentes, sem expor payload em logs.
Feature 01: Catálogo de métricas OTel (counters, gauges, histograms) com labels por subscription
Feature 02: Logs `slog` estruturados com redaction de payload e campos canônicos (event_id, subscription, attempt, correlation_id)
Feature 03: Tracing OTel propagado via `headers.traceparent` ligando publisher→handler
Feature 04: Documentação de dashboard sugerido (queries Prometheus) e runbook inicial na techspec

### Epic 06 - Bootstrap no cmd/worker, feature flag e rollout
Objetivo: integrar Outbox ao ciclo de vida do `cmd/worker`, com goroutines separadas para Dispatcher e Cron, feature flag global Viper e plano de rollout em dois deploys.
Feature 01: Bootstrap do Outbox em `internal/infrastructure/runtime/bootstrap.go` no modo worker
Feature 02: Config Viper (`outbox.dispatcher.enabled`, ticks, batch, retry, retention) com defaults documentados
Feature 03: ADR ("Outbox transacional como Publisher opt-in") referenciando ADR-003 e atualização em `AGENTS.md`/`CLAUDE.md` documentando quando usar cada Publisher

## Itens em Aberto
- Catálogo inicial de `event_type`/`event_version` real a ser publicado em sprint subsequente (MVP usa apenas handler dummy).
- Caso de uso real de produção que acompanhará o handler dummy no segundo release — alinhar com PO/produto.
- Tamanho de batch ideal do Dispatcher após benchmark (default proposto: 100).
- Validação de retenção 90d com Legal/Compliance antes do deploy em ambientes regulados (Hipótese H7).
- Mecanismo de purge sob demanda LGPD (direito de eliminação) — runbook manual no MVP; tooling automatizado em sprint futura.
- Decisão final do mecanismo de feature flag se Viper config exigir restart no projeto (avaliar live-reload).
- Política de evolução de schema para `event_version` (ADR separada planejada na techspec ou sprint seguinte).
- Mecanismo automatizado de re-enfileiramento de DLQ (no MVP é query manual via runbook).
