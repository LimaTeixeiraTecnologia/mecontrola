# Load tests — mecontrola

Scripts de carga para validar capacidade do MVP antes do primeiro deploy e antes de releases majors.

Cobertura:

- **Webhook Kiwify** — `kiwify-webhook.js` (k6, 50 VUs × 2 min).
- **Outbox throughput** — `outbox-throughput.sh` (bash, N eventos sintéticos, mede drenagem).

## Pré-requisitos

```bash
# macOS
brew install k6 postgresql

# Linux
sudo apt install -y postgresql-client
sudo gpg -k && sudo gpg --no-default-keyring --keyring /usr/share/keyrings/k6-archive-keyring.gpg --keyserver hkp://keyserver.ubuntu.com:80 --recv-keys C5AD17C747E3415A3642D57D77C6C491D6AC1D69
echo "deb [signed-by=/usr/share/keyrings/k6-archive-keyring.gpg] https://dl.k6.io/deb stable main" | sudo tee /etc/apt/sources.list.d/k6.list
sudo apt update && sudo apt install -y k6
```

Stack local rodando antes de cada teste:

```bash
task local:up   # postgres, mailpit, app
```

## Envs comuns

| Variável | Default | Descrição |
|----------|---------|-----------|
| `BACKEND` | `http://localhost:8080` | base URL da API |
| `VUS` | varia por script | número de virtual users |
| `DURATION` | `2m` | duração do teste |

## Executando

### Kiwify webhook

```bash
export KIWIFY_WEBHOOK_SECRET="<mesmo valor do .env do app>"
export KIWIFY_PRODUCT_ID="<product_id do plano mensal>"

k6 run scripts/loadtest/kiwify-webhook.js
```

Thresholds (falham o run quando violados):

| Métrica | Limite |
|---------|--------|
| `http_req_duration{status:202}` p95 | < 500 ms |
| `http_req_failed` rate | < 1 % |

Cada VU gera um `funnel_token` (sck) único de 43 chars + `order_id` único, então cada request cria uma sessão de onboarding nova — mantenha vacuum/autovacuum habilitados na tabela `onboarding_sessions`.

### Outbox throughput

```bash
export DATABASE_URL="postgres://mecontrola:mecontrola@localhost:5432/mecontrola?sslmode=disable"
export EVENT_COUNT=1000
export TIMEOUT_SEC=300

bash scripts/loadtest/outbox-throughput.sh
```

O script insere `EVENT_COUNT` eventos sintéticos (`event_type=loadtest.synthetic.v1`) com `run_id` único, e poll a cada 2 s até `published_at IS NOT NULL` em todos. Imprime `events/s` no final.

Limpeza pós-teste (eventos drenados ficam com `status=1`; opcionalmente):

```sql
DELETE FROM mecontrola.outbox_events
 WHERE event_type = 'loadtest.synthetic.v1'
   AND created_at < now() - interval '1 hour';
```

## Interpretando resultados

### Pass

`k6` imprime `checks........: 100.00%` e todas as thresholds com `✓`. O script bash imprime `PASS: N eventos drenados em Xs (~Y events/s)`.

### Falha por latência p95

Possíveis causas:
- DB pool saturado → ver `mecontrola_db_pool_in_use` no Grafana.
- LLM lento (WhatsApp) → ver `agent_run_duration_seconds` (substrato `internal/platform/agent`).
- Lock contention em `onboarding_sessions` na virada de mês → checar `pg_stat_activity` por `wait_event_type=Lock`.

### Falha por error rate

- Status 401 em Kiwify → `KIWIFY_WEBHOOK_SECRET` divergente entre script e app.
- Status 415 → `Content-Type` ausente (não deveria — o script já envia).
- Status 429 → rate limit. Ajuste `KIWIFY_WEBHOOK_RATE_LIMIT_PER_MIN` no app ou reduza `VUS`.
- Status 5xx → ver logs do app (`docker compose logs app`) e métrica `http_server_requests_total{code=~"5.."}`.

### Falha por timeout no outbox

- Dispatcher parado → verificar `task local:logs` por `outbox.dispatcher.tick`.
- Throughput esperado para 1 worker default: ~150–250 events/s em hardware local.
- Se `events/s < 50`: investigar `mecontrola_outbox_publish_duration_seconds` (broker lento) ou índice `idx_outbox_events_pending`.

## Quando rodar

- **Antes do primeiro deploy** (Hostinger) — baseline obrigatória.
- **Antes de release major** (mudança de schema, novo broker, troca de LLM).
- **Após mudança em** `internal/billing/infrastructure/http/server/middleware/rate_limit.go`, `internal/platform/outbox/`, `internal/platform/agent/` ou `internal/agents/`.

## Limitações conhecidas

- Não cobre carga de leitura (queries de extrato/relatório). Pode ser adicionado quando esses endpoints saírem do MVP.
- LLM rate-limit do OpenRouter é externo — testes longos podem ser barrados pelo provedor.
- O script de outbox usa `aggregate_user_id=NULL`, o que é permitido pela allowlist de sistema (ADR-004); não usar em staging com alerta de `outbox.event.missing_aggregate_user_id` ativo.
