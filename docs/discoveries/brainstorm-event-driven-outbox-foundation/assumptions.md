# Hipóteses e Premissas

## Hipóteses Confirmadas
| ID | Hipótese | Evidência | Impacto | Status |
| --- | --- | --- | --- | --- |
| H1 | PostgreSQL é o backend persistente do produto e suporta `FOR UPDATE SKIP LOCKED` + `LISTEN/NOTIFY`. | `go.mod` declara `github.com/jackc/pgx/v5 v5.9.2`; PostgreSQL 9.5+ suporta `SKIP LOCKED` nativamente; testes usam `testcontainers-go/modules/postgres`. | Permite implementar Outbox no MESMO schema do agregado preservando atomicidade transacional. | confirmada |
| H2 | `cmd/worker` está idle e pode hospedar dispatcher + cron sem conflito com cargas atuais. | Inspeção em `cmd/worker/worker.go` mostra `worker idle — aguardando jobs` aguardando `Context.Done()`. | Habilita rodar dispatcher (goroutine) e cron (goroutine) sem novo binário. | confirmada |
| H3 | Volumetria alvo do primeiro ano é média (10–100 ev/s, ~1M/dia, SLO p95 < 1s). | Resposta explícita do usuário na Rodada 1.4. | Polling com ticker de 250ms é suficiente; LISTEN/NOTIFY não obrigatório no MVP. | confirmada |
| H4 | Modelo de consumo é 1×N in-process: cada evento pode ter múltiplos handlers internos no mesmo binário. | Resposta explícita do usuário na Rodada 2.1. | Justifica schema two-table com `outbox_deliveries` por handler para retries independentes. | confirmada |

## Hipóteses Não Validadas
| ID | Hipótese | Risco se falsa | Como validar | Dono |
| --- | --- | --- | --- | --- |
| H5 | Carga de escrita extra no caminho de publish (+1 linha em `outbox_deliveries` por handler ativo) é aceitável vs. throughput atual. | Contenção em escritas, degradação de p95 de endpoints transacionais. | Benchmark com `pgbench` ou load test no caminho de publish na techspec; medir latência de transação antes/depois. | Time backend |
| H6 | Handlers serão escritos de forma idempotente desde o primeiro dia (chave: `event_id`). | Side-effects duplicados em produção (cobranças, e-mails, notificações). | Definir como requisito obrigatório na techspec; revisar via PR template e checklist de code review. | Tech lead da área |
| H7 | Housekeeping de 90d é suficiente; não há requisito regulatório que obrigue retenção maior. | Apagar evidência exigida por compliance/auditoria. | Confirmar com Legal/Compliance antes da execução; registrar em ADR. | Tech lead + Compliance |
| H8 | Carga de polling agressivo (~4 queries/s por instância em fila vazia) não impacta o desempenho do Postgres em produção. | Custo de CPU/IO no DB, contenção em índices. | Monitorar `pg_stat_statements` após deploy; ajustar intervalo do ticker se necessário. | SRE/DBA |

## Restrições Confirmadas
- Postgres é fixo como backend do Outbox (resposta Rodada 2.4).
- Sem broker externo no MVP (RabbitMQ, Kafka, NATS, SNS/SQS, etc.). Excluído explicitamente na Rodada 2.2.
- Sem integrações HTTP/webhooks externos no MVP — apenas handlers in-process. Excluído explicitamente na Rodada 2.2.
- Sem ordenação global FIFO entre `event_types` distintos. Ordem apenas dentro de `aggregate_id`/`partition_key`. Excluído na Rodada 2.2.
- Sem CDC/Debezium/leitura via WAL. Decorrência da restrição dominante "simplicidade operacional" + Postgres fixo (premissa de coerência A2 inferida — registrar em techspec).
- Linguagem Go 1.26.x; runtime monólito; dispatcher em goroutine do `cmd/worker`.
- Scheduler de housekeeping/reaper: `github.com/robfig/cron/v3` (versão estável atual). Dispatcher principal usa `time.Ticker` próprio (250ms) — decisão explícita Rodada 3.2.
- Política de retry: 8 tentativas com backoff exponencial + jitter (base 1s, cap 5min); depois `status='dead_letter'` com `dead_letter_at` preenchido (Rodada 4.3).
- SLO formal: latência p95 entrega < 1s; p99 ocasional 1–2s aceitável (Rodada 4.4).
- Schema two-table: `outbox_events` (imutável) + `outbox_deliveries` (linha por handler). Confirmado Rodada 4.1.
- Idempotência de handler é obrigatória (premissa técnica não-negociável de at-least-once).

## Preferências Não Bloqueantes
- LISTEN/NOTIFY como caminho de evolução futura se p99 degradar consistentemente acima de 2s.
- Possível migração para broker externo (Kafka/NATS) em V2 quando volumetria crescer; manter contrato `Publisher` desacoplado do transporte.
- Métricas OpenTelemetry desejáveis no MVP: `outbox.pending`, `outbox.published`, `outbox.dlq`, `outbox.latency_ms{handler}` — granularidade por `event_type` e por `subscription_name`.
- Configurabilidade futura do retry policy por subscription (no MVP é global).
