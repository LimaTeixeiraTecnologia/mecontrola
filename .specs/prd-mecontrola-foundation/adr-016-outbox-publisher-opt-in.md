# Registro de Decisão Arquitetural (ADR)

## Metadados

- **Título:** Outbox transacional como Publisher opt-in
- **Data:** 2026-06-02
- **Status:** Aceita
- **Decisores:** Tech lead backend, autor do PRD `prd-outbox-event-driven`
- **Relacionados:**
  - [ADR-003 — Eventbus tipado via generics + emissão pós-UoW.Commit](./adr-003-typed-eventbus-generics.md) (coexiste; não revogada)
  - [ADR-002 — Database Manager central UoW](./adr-002-database-manager-central-uow.md) (Publisher recebe `database.DBTX` exposto pelo UoW)
  - [ADR-004 — Error sentinels + RFC 7807](./adr-004-error-sentinels-rfc7807.md) (sentinels do Outbox NÃO entram no mapeamento — caminho assíncrono)
  - PRD: `.specs/prd-outbox-event-driven/prd.md` (v4, hash `67f3ee7d23cca61bc3483523fe530f4c7051b41eea1fb1839f5bbd7052dd4506`)
  - Techspec: `.specs/prd-outbox-event-driven/techspec.md`

## Contexto

O MeControla é um monólito Go organizado em módulos de domínio. A única forma atual de propagar eventos entre módulos é o eventbus in-process `events.Bus` (ADR-003), que descarta mensagens quando o canal do subscriber está cheio e não sobrevive a crash do processo — apropriado para sinais voláteis, inadequado para side-effects críticos (notificações, projeções, integrações futuras) que precisam ser entregues depois do commit transacional do agregado, mesmo diante de falha, deploy ou reinício.

Sem uma fundação canônica para entrega assíncrona com garantia, cada nova feature tende a inventar a sua própria solução (goroutine fire-and-forget, chamada HTTP inline na transação, cronjob específico). Esse padrão acumula dívida estrutural e expõe o produto a perda silenciosa de eventos.

Restrições adicionais (todas registradas no PRD):
- Backend persistente fixo: PostgreSQL no mesmo schema/DB do agregado.
- Sem dependência de broker externo (sem RabbitMQ, Kafka, NATS, SNS/SQS).
- Scheduler obrigatoriamente `github.com/robfig/cron/v3` (v3.0.1 — D-04).
- Dispatcher como goroutine dentro do `cmd/worker` existente — sem novo binário.
- Coordenação multi-instância exclusivamente via `FOR UPDATE SKIP LOCKED`.

## Decisão

Implementar **Transactional Outbox em duas tabelas** (`outbox_events` + `outbox_deliveries`) como caminho **opt-in paralelo** ao `events.Bus`, exposto pelo pacote `internal/infrastructure/outbox/`:

- `outbox.Publisher` recebe a `database.DBTX` exposta pelo `UnitOfWork[T].Do` (ADR-002) e insere o evento + N deliveries na **mesma transação** do agregado. O commit do agregado é a fronteira de durabilidade (atomicidade SQL).
- `outbox.Dispatcher` roda como goroutine no `cmd/worker` (`runtime.Subsystem` chamado `outbox.Subsystem`); polling com `time.Ticker` (default 500ms); claim via `FOR UPDATE SKIP LOCKED`; retry exponencial com jitter (base 2s, cap 5min, até 15 tentativas); DLQ por delivery.
- `outbox.Cron` (via `robfig/cron/v3@v3.0.1`) registra housekeeping `@daily` (retenção 90d) e reaper `@every 1m` (libera deliveries `claimed` há mais de 5min) — alinhado com US-10 RTO 5min.
- Feature flag global `OUTBOX_DISPATCHER_ENABLED` (Viper, lida no boot; restart obrigatório para alterar). Default `true`.
- Observabilidade OTel completa desde o MVP (counters, histograms, gauges com label `subscription_name`; logs `slog` sem payload; traces propagados via `headers.traceparent`).

**Critério de escolha entre Publishers**:

| Característica | `events.Bus` (ADR-003) | `outbox.Publisher` (esta ADR) |
|---|---|---|
| Durabilidade | volátil (in-memory) | persistente (Postgres) |
| Sobrevive a crash | não | sim |
| Garantia | best-effort (drop em buffer cheio) | at-least-once |
| Retry / DLQ | não | sim |
| Latência típica | < 1ms | ~250–500ms (1 tick) |
| Quando usar | sinais voláteis intra-processo, propagação de eventos para observabilidade interna | side-effects críticos: notificações, projeções, integrações futuras |
| Custo de uso | gratuito | +1 INSERT por handler na transação do agregado |

**Regra de revisão**: se a perda do evento causaria inconsistência observável pelo usuário ou cobrança duplicada/perdida, **usar `outbox.Publisher`**. Caso contrário, `events.Bus` é suficiente.

## Alternativas Consideradas

- **Manter apenas `events.Bus`** (status quo). Descartada: perde garantia at-least-once exigida por side-effects críticos.
- **Adotar broker externo (RabbitMQ, Kafka, NATS)**. Descartada por R-03 do PRD: introduz nova infraestrutura, custo operacional e dependência sem necessidade para a volumetria alvo (10–100 ev/s).
- **CDC / Debezium via WAL do Postgres**. Descartada por FE-03: introduz componente externo (Debezium) e exige replicação lógica configurada; over-engineering para a volumetria alvo.
- **Single-table outbox com fan-out agregado** (Alt 1 do scorecard de brainstorming). Descartada: perde observabilidade granular por handler e dificulta DLQ por subscription.
- **LISTEN/NOTIFY como mecanismo principal** (Alt 3 do scorecard). Promovida a evolução V2 condicionada a métrica de p99 > 2s sustentado; complexidade desnecessária para o MVP.
- **Polling particionado por advisory lock por hash de `partition_key`** (Alt 4 do scorecard). Descartada por over-engineering (FIFO global está fora de escopo, FE-05).
- **Substituir `events.Bus` pela Outbox**. Descartada: revogaria ADR-003 sem ganho — eventos voláteis seguem servidos melhor por canais in-memory.

## Consequências

### Benefícios Esperados

- Caminho canônico, documentado e auditável para side-effects críticos — elimina incentivo a soluções ad-hoc.
- Garantia at-least-once preservada por atomicidade SQL nativa: evento existe se e somente se o agregado existe.
- Observabilidade granular por subscription (latência, DLQ, attempts) desde o primeiro deploy.
- Zero infraestrutura nova: roda no Postgres já existente, no `cmd/worker` já existente, sem broker, sem leader election externo.
- Escala horizontal linear com réplicas do worker via `SKIP LOCKED`.

### Trade-offs e Custos

- +1 escrita transacional por handler no caminho de publish. Para volumetria alvo (1–3 handlers/evento, 10–100 ev/s), aceitável; benchmark obrigatório monitora regressão.
- Idempotência do handler vira regra obrigatória do contrato (chave canônica `event_id`). Esforço transferido para o desenvolvedor da subscription; PR template fiscaliza.
- Latência mínima de entrega = 1 tick do Dispatcher (500ms default). Não recomendado para casos com SLO sub-100ms.
- Coexistência de dois Publishers exige documentação clara para evitar uso incorreto. ADR + AGENTS.md cobrem.
- Polling de ~2 qps por réplica em fila vazia. Aceitável para 1–3 réplicas; ajuste de tick necessário se réplicas crescerem.

### Riscos e Mitigações

- **Handler não-idempotente** → side-effect duplicado: mitigado por PR template (RF-40), godoc explícito, code review obrigatório nas primeiras subscriptions reais.
- **Pressão no Postgres por polling agressivo + escrita extra**: mitigado por benchmark pré-go-live, monitoramento de `pg_stat_statements` por 14d pós-deploy, tick configurável.
- **Housekeeping silenciosamente falho infla tabela**: mitigado por alerta `outbox.housekeeping.deleted.total == 0 por 48h`.
- **Conflito conceitual `events.Bus` × `outbox.Publisher` no time**: mitigado por critério explícito acima, revisão periódica das ADRs.

## Plano de Implementação

Detalhado na techspec `.specs/prd-outbox-event-driven/techspec.md` (3 fases: fundação → publisher+dispatcher → cron+bootstrap+docs) e rollout em 2 deploys (flag off → smoke staging → flag on em produção, RF-27).

## Monitoramento e Validação

- Métrica `outbox.delivery.latency_ms{subscription_name}` < 1s p95 sustentado por 14d.
- `outbox.deliveries.dlq.total` por subscription < 0,1% do total processado em janela de 24h.
- `pg_stat_statements` mostra CPU/IO < +15% sobre baseline pré-deploy em 14d.
- Pelo menos 1 subscription real (não dummy) em produção no segundo release usando `outbox.Publisher`.
- Critério para revisar: p99 > 2s sustentado por 7d → promover discussão de LISTEN/NOTIFY (V2).

## Impacto em Documentação e Operação

- `AGENTS.md` raiz: nova seção "Outbox vs events.Bus — critério de escolha".
- `CLAUDE.md`: referência cruzada à seção do AGENTS.
- `internal/infrastructure/outbox/AGENTS.md`: documenta contrato local do pacote.
- `docs/runbooks/outbox.md`: novo runbook (incidentes, DLQ, LGPD, re-enfileiramento).
- `docs/observability/outbox-dashboard.json`: dashboard Grafana sugerido (6 painéis).
- `.github/PULL_REQUEST_TEMPLATE.md`: checklist condicional para subscriptions Outbox.

## Revisão Futura

Reavaliar em até 12 meses ou quando qualquer um dos disparos abaixo ocorrer:
- Volumetria sustentada ultrapassar 500 ev/s → discussão de broker externo / particionamento.
- p99 de entrega ultrapassar 2s consistentemente → discussão de LISTEN/NOTIFY.
- Número médio de handlers por evento ultrapassar 5 → discussão de batching de insert no publish.
- Regulatório explícito exigir criptografia de payload em repouso → discussão de envelope encryption.
