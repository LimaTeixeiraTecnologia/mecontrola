# Load & Chaos — Runbook MVP

> Data: 2026-06-16
> Escopo: gap #6 ops (`docs/runbooks/2026-06-15-mvp-gap-analysis.md`). Cobre load test, chaos test e drill de backup/restore antes do primeiro deploy em Hostinger.
> Artefatos: `scripts/loadtest/`, `scripts/chaos/`.

## Quando rodar

| Trigger | Mínimo obrigatório |
|---------|--------------------|
| Antes do primeiro deploy (Hostinger) | TUDO neste runbook, baseline registrada. |
| Antes de release major | Load Kiwify + outbox throughput + 1 chaos. |
| Após mudança em `internal/platform/outbox/` ou middleware de rate limit | Load Kiwify + outbox throughput. |
| Após mudança em `cmd/worker/` ou `internal/budgets/` | `kill-worker-mid-alert.sh`. |
| Trimestralmente | `backup-restore-drill.sh` + 1 chaos rotativo. |

## Pré-requisitos

```bash
brew install k6 postgresql jq age openssl    # macOS
# Linux: ver scripts/loadtest/README.md
docker --version                                # >= 24
go version                                      # >= 1.22
```

Stack local:

```bash
task local:up
task migrate:up
```

Envs comuns (use `.env.loadtest` separado):

```bash
export BACKEND="http://localhost:8080"
export DATABASE_URL="postgres://mecontrola:mecontrola@localhost:5432/mecontrola?sslmode=disable"
export KIWIFY_WEBHOOK_SECRET="<segredo do app>"
export KIWIFY_PRODUCT_ID_MONTHLY="<product_id mensal>"
export TELEGRAM_WEBHOOK_SECRET="<token do bot>"
export PG_CONTAINER="mecontrola-postgres"
```

## Parte 1 — Load

### 1.1 Webhook Kiwify (50 VUs × 2 min)

```bash
k6 run scripts/loadtest/kiwify-webhook.js
```

Threshold:

| Métrica | Limite | Falha sinaliza |
|---------|--------|----------------|
| `http_req_duration{status:202}` p95 | < 500 ms | DB pool saturado / lock em `onboarding_sessions` |
| `http_req_failed` | < 1 % | Rate limit do middleware ou erros 5xx |

Sinais a observar enquanto roda:

- Grafana Explore / Prometheus: `http_server_requests_total{handler="kiwify_webhook"}` e `mecontrola_db_pool_in_use`.
- Logs Loki: `{service="api"} |= "kiwify" |= "ERROR"`.
- Pgsql ad hoc:
  ```sql
  SELECT wait_event_type, count(*) FROM pg_stat_activity
   WHERE datname='mecontrola' GROUP BY 1 ORDER BY 2 DESC;
  ```

### 1.2 Webhook Telegram (30 VUs × 2 min)

```bash
k6 run scripts/loadtest/telegram-webhook.js
```

Threshold:

| Métrica | Limite |
|---------|--------|
| p95 | < 1 s (LLM dominates) |
| `http_req_failed` | < 2 % |

Falha esperada: rate limit do OpenRouter. Mitigar reduzindo `VUS=10` ou usando mock provider.

### 1.3 Outbox throughput (1000 eventos sintéticos)

```bash
bash scripts/loadtest/outbox-throughput.sh
```

Baseline esperado em dev local: 150–250 events/s. Falha sinaliza:

- Dispatcher parado → `task local:logs app | rg outbox.dispatcher`.
- Índice `idx_outbox_events_pending` ausente → conferir migrations.
- Broker downstream lento → métrica `mecontrola_outbox_publish_duration_seconds`.

Limpeza:

```sql
DELETE FROM mecontrola.outbox_events
 WHERE event_type='loadtest.synthetic.v1'
   AND created_at < now() - interval '1 hour';
```

### 1.4 Lock contention no `onboarding_sessions` na virada de mês

O cenário é coberto indiretamente pelo `kiwify-webhook.js` rodando entre `23:50` e `00:10` do último dia do mês. Para repro determinístico:

```bash
DATE_FAKE="2026-06-30T23:55:00Z" k6 run scripts/loadtest/kiwify-webhook.js
```

Métrica chave: `mecontrola_onboarding_session_repo_lock_wait_ms` (se exposta). Caso contrário, capturar via:

```sql
SELECT relation::regclass, mode, count(*)
  FROM pg_locks WHERE relation::regclass::text LIKE 'mecontrola.onboarding_sessions%'
 GROUP BY 1,2;
```

## Parte 2 — Chaos

Antes de cada chaos: capture baseline de `/healthz`, métricas e `count(*)` das tabelas tocadas.

### 2.1 Kill server no meio do onboarding

Pré-requisito: iniciar `cmd/server` com PID file.

```bash
go build -o ./bin/server ./cmd/server
./bin/server & echo $! > /tmp/mecontrola-server.pid
export SERVER_RESTART_CMD='./bin/server & echo $! > /tmp/mecontrola-server.pid'
bash scripts/chaos/kill-server-mid-onboarding.sh
```

Critério de sucesso: `PASS: onboarding sobreviveu ao kill+restart` impresso. `GET /state` retorna 200 antes e depois.

Falha → checar:
- `mecontrola.onboarding_sessions` tem linha com `token=<TOKEN>` e `status` consistente.
- Logs: `service=api msg="onboarding.session.load"`.

### 2.2 Kill worker no meio do dispatch de alerta

```bash
export WORKER_BIN="go run ./cmd/worker"
bash scripts/chaos/kill-worker-mid-alert.sh
```

Critério de sucesso: tabela `budget_alerts_sent` termina com **exatamente 1** linha para o tuple `(card_id, invoice_id, threshold=90)`, `notified_at IS NOT NULL`.

Falha sinaliza:
- Duplicação (`count > 1`) → violação de G2 idempotência; investigar UNIQUE em `budget_alerts_sent`.
- `notified_at` permanece NULL após restart → worker não retoma alertas; investigar query de seleção em `budgets/infrastructure/jobs/handlers/`.

### 2.3 Postgres indisponível por 30 s

```bash
bash scripts/chaos/disconnect-postgres.sh
```

Critério de sucesso: `/healthz` volta 200 dentro de 60 s após `docker unpause`. `psql SELECT 1` funciona.

Falha sinaliza:
- Pool não recicla conexões mortas → conferir `pgx` `MaxConnLifetime`.
- `/healthz` precisa de mais que 60 s → ajustar `RECOVER_SEC` ou checar liveness probe.

## Parte 3 — Backup/restore drill

Independente do `task security:backup-restore-smoke` (que valida o ciclo cifrado rclone+age remoto). Este aqui é um sanity check local rápido:

```bash
bash scripts/chaos/backup-restore-drill.sh
```

Critério: counts iguais em `users`, `cards`, `transactions`, `budget_alerts_sent` antes e depois.

Após este drill local passar, rodar o ciclo completo:

```bash
task security:backup-restore-smoke
```

## Tabela consolidada de thresholds

| Teste | Métrica | Limite | Severidade |
|-------|---------|--------|------------|
| Kiwify load | p95 status 202 | < 500 ms | bloqueante deploy |
| Kiwify load | error rate | < 1 % | bloqueante deploy |
| Telegram load | p95 | < 1 s | bloqueante deploy |
| Telegram load | error rate | < 2 % | bloqueante deploy |
| Outbox throughput | drain de 1k | < 30 s | bloqueante deploy |
| Kill server | `/state` 200 antes/depois | igual | bloqueante deploy |
| Kill worker | count alertas | exatamente 1 | bloqueante deploy (G2) |
| Postgres pause 30s | `/healthz` recovery | < 60 s | bloqueante deploy |
| Backup/restore drill | diff counts | zero | bloqueante deploy |

## Quando falhar — onde olhar

| Sintoma | Logs / Métricas | Query SQL |
|---------|-----------------|-----------|
| 5xx em Kiwify | `service=api msg=~"kiwify.*"` ; `http_server_requests_total{code=~"5.."}` | `SELECT status, count(*) FROM mecontrola.billing_kiwify_events WHERE created_at > now() - interval '5 min' GROUP BY 1` |
| Outbox lento | `mecontrola_outbox_publish_duration_seconds`, `mecontrola_outbox_pending_total` | `SELECT status, count(*) FROM mecontrola.outbox_events WHERE published_at IS NULL GROUP BY 1` |
| Onboarding state vazio | `service=api msg="onboarding.session.load"` | `SELECT id, status, updated_at FROM mecontrola.onboarding_sessions WHERE token=$1` |
| Alerta duplicado | `service=worker msg="budget.alert.dispatch"` | `SELECT * FROM mecontrola.budget_alerts_sent WHERE card_id=$1 ORDER BY detected_at` |
| `/healthz` flapping | `service=api msg="healthz.check"` | `SELECT state, count(*) FROM pg_stat_activity WHERE datname='mecontrola' GROUP BY 1` |

## Limitações conhecidas

1. **Scripts nunca foram executados de verdade neste turno** — são entregáveis testados apenas via `bash -n` para sintaxe. O operador DEVE rodar pelo menos uma vez antes de declarar baseline.
2. `kill-worker-mid-alert.sh` assume schema atual de `budget_alerts_sent` (`card_id`, `invoice_id`, `threshold`, `notified_at`). Se o schema mudar, atualizar as queries.
3. `disconnect-postgres.sh` exige container Docker — instalações nativas precisam adaptar para `pg_ctl stop -m smart`.
4. `kiwify-webhook.js` não cobre o caminho "abandoned_cart" (payload distinto). Adicionar cenário separado se necessário.
5. LLM rate-limit é externo ao MVP; cenários longos do Telegram podem ser barrados pelo provedor — usar provider stub em CI.

## Próximos passos

- Após primeiro run, registrar baseline numérico (p95, events/s) em `docs/slo.md`.
- Adicionar `task chaos:*` em `taskfiles/` envolvendo estes scripts (fora do escopo deste turno — apenas entregáveis bash/k6 + runbook).
- Avaliar integração com Grafana annotation (`mcp__grafana__create_annotation`) marcando início/fim de cada chaos run.

## Referências

- Discovery do gap #6: `docs/runbooks/2026-06-15-mvp-gap-analysis.md`
- Status MVP: `docs/runbooks/2026-06-16-mvp-production-readiness-status.md`
- Backup/restore cifrado (rclone+age): `deployment/scripts/pg-restore-smoke.sh` via `task security:backup-restore-smoke`
- SLO/SLI: `docs/slo.md`
- Smoke E2E baseline: `scripts/smoke/mvp_e2e.sh`
