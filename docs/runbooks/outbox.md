# Runbook — Outbox Transacional (MeControla)

<!-- RF-25 / RF-16 / RF-17 / RF-27 / RF-29 / RF-30 / RF-32 -->

**Última revisão:** 2026-06-02
**Responsável:** @JailtonJunior94
**Pacote:** `internal/infrastructure/outbox`
**Dashboard Grafana:** `docs/observability/outbox-dashboard.json` (uid `mecontrola-outbox-v1`)
**ADR:** `.specs/prd-mecontrola-foundation/adr-016-outbox-publisher-opt-in.md`

---

## Pré-requisitos operacionais

| Ferramenta | Uso |
|---|---|
| `psql` com acesso ao banco de produção/staging | Todos os snippets SQL abaixo |
| Acesso ao env de produção para editar variáveis | Seções 1, 5 |
| Dashboard Grafana com datasource Prometheus configurado | Diagnóstico visual |

---

## 1. Desligar/Religar o Dispatcher (kill-switch, RF-29)

**RTO: < 2 minutos.**

O Dispatcher é controlado pela variável de ambiente `OUTBOX_DISPATCHER_ENABLED`. O Publisher continua gravando no banco mesmo com o Dispatcher desligado — nenhum evento é perdido, apenas a entrega fica suspensa.

### Desligar

```bash
# 1. Editar variável de ambiente no env de produção / staging
OUTBOX_DISPATCHER_ENABLED=false

# 2. Restart do worker (graceful via SIGTERM — aguarda handlers in-flight até HandlerTimeout)
systemctl restart mecontrola-worker
# ou, em containers:
kubectl rollout restart deployment/mecontrola-worker
```

**Verificação:** O log de boot deve conter:
```
level=INFO msg="outbox.dispatcher.disabled" reason="OUTBOX_DISPATCHER_ENABLED=false"
```

### Religar

```bash
# 1. Reverter a variável
OUTBOX_DISPATCHER_ENABLED=true

# 2. Restart
systemctl restart mecontrola-worker
# ou:
kubectl rollout restart deployment/mecontrola-worker
```

**Verificação:** O log de boot deve conter:
```
level=INFO msg="outbox.dispatcher.started" tick_interval=500ms batch_size=50
```

---

## 2. Inspecionar DLQ (RF-16)

Deliveries em `dead_letter` são o sinal de que o handler falhou após `OUTBOX_MAX_ATTEMPTS` tentativas (default: 15) ou retornou `outbox.ErrPermanent`.

### Contar deliveries em DLQ por subscription

```sql
-- Total por subscription_name
SELECT
    subscription_name,
    COUNT(*)           AS dlq_count,
    MAX(dead_letter_at) AS last_dlq_at,
    MAX(last_error)    AS last_error
FROM outbox_deliveries
WHERE status = 'dead_letter'
GROUP BY subscription_name
ORDER BY dlq_count DESC;
```

### Ver detalhes de um delivery específico

```sql
-- Substitua <subscription_name> e <event_id>
SELECT
    d.id,
    d.event_id,
    d.subscription_name,
    d.status,
    d.attempts,
    d.last_error,
    d.dead_letter_at,
    d.claimed_by,
    e.event_type,
    e.aggregate_type,
    e.aggregate_id,
    e.occurred_at
FROM outbox_deliveries d
JOIN outbox_events e ON e.id = d.event_id
WHERE d.subscription_name = '<subscription_name>'
  AND d.status = 'dead_letter'
ORDER BY d.dead_letter_at DESC
LIMIT 50;
```

### Ver payload do evento (atenção: nunca logar output — RF-30)

```sql
-- Apenas para diagnóstico local; não logar nem colar em tickets
SELECT id, event_type, aggregate_type, aggregate_id, occurred_at
FROM outbox_events
WHERE id = '<event_id>';
```

> **IMPORTANTE (RF-30):** O campo `payload` contém dados de negócio. Nunca incluir o payload em logs, tickets, relatórios de incidente ou mensagens de Slack. Inspect apenas localmente via sessão `psql` criptografada.

---

## 3. Re-enfileirar Delivery do DLQ (RF-17)

Para reprocessar uma delivery do DLQ, redefina seu status para `pending`, zere as tentativas e defina `next_retry_at` como `now()`. O Dispatcher vai buscá-la no próximo ciclo de poll.

### Re-enfileirar por subscription_name (todos da DLQ)

```sql
BEGIN;

UPDATE outbox_deliveries
SET
    status        = 'pending',
    attempts      = 0,
    next_retry_at = now(),
    last_error    = NULL,
    dead_letter_at = NULL,
    updated_at    = now()
WHERE subscription_name = '<subscription_name>'
  AND status = 'dead_letter';

-- Verificar linhas afetadas antes de COMMIT
-- Se o resultado for 0, confirmar o subscription_name acima
SELECT COUNT(*) FROM outbox_deliveries
WHERE subscription_name = '<subscription_name>' AND status = 'pending';

COMMIT;
```

### Re-enfileirar um delivery específico por ID

```sql
BEGIN;

UPDATE outbox_deliveries
SET
    status        = 'pending',
    attempts      = 0,
    next_retry_at = now(),
    last_error    = NULL,
    dead_letter_at = NULL,
    updated_at    = now()
WHERE id = <delivery_id>
  AND status = 'dead_letter';

COMMIT;
```

> **Pré-condição:** Verificar o motivo da DLQ antes de re-enfileirar. Se o handler não foi corrigido, o delivery voltará à DLQ após 15 novas tentativas. Corrigir a causa raiz primeiro.

---

## 4. Purgar Evento por Demanda LGPD (RF-32)

Para remover dados de um agregado específico (ex: `DELETE /users/:id` com requisição LGPD), apagar os eventos e deliveries associados ao `aggregate_id`.

> **Política de segredos (RF-30):** Payload jamais deve circular em aplicação ou log. O purge aqui remove os dados do banco definitivamente.

```sql
BEGIN;

-- 1. Identificar eventos do agregado
SELECT id, event_type, occurred_at
FROM outbox_events
WHERE aggregate_id = '<aggregate_id>'
ORDER BY occurred_at;

-- 2. Confirmar contagem
SELECT COUNT(*) AS events_to_purge
FROM outbox_events
WHERE aggregate_id = '<aggregate_id>';

-- 3. Remover deliveries (CASCADE remove automaticamente, mas explicitamos para auditoria)
DELETE FROM outbox_deliveries
WHERE event_id IN (
    SELECT id FROM outbox_events WHERE aggregate_id = '<aggregate_id>'
);

-- 4. Remover eventos
DELETE FROM outbox_events
WHERE aggregate_id = '<aggregate_id>';

COMMIT;
```

**Verificação pós-purge:**

```sql
SELECT COUNT(*) FROM outbox_events WHERE aggregate_id = '<aggregate_id>';
-- Deve retornar 0
```

> **Auditoria:** Registrar data, `aggregate_id`, responsável e ticket LGPD correspondente. Guardar evidência fora do banco (ex: documento de purge no sistema de tickets).

---

## 5. Diagnosticar Pending Crescente

Quando `outbox_deliveries_pending` cresce continuamente sem ser drenado, o Dispatcher pode estar parado, sobrecarregado ou com todos os handlers em DLQ.

### Diagnóstico via dashboard

1. Abrir painel **"Pending por Subscription"** — identificar qual subscription acumula.
2. Verificar painel **"Processed Rate por Subscription"** — se zerado, Dispatcher não está consumindo.
3. Verificar painel **"DLQ Count por Subscription"** — se crescendo junto, handlers estão falhando.
4. Verificar painel **"Atividade Reaper + Housekeeping"** — se Reaper hiperativo (> 50 em 10m), worker crashou recentemente.

### Diagnóstico via SQL

```sql
-- Quantidade pendente por subscription
SELECT subscription_name, COUNT(*) AS pending_count
FROM outbox_deliveries
WHERE status = 'pending'
GROUP BY subscription_name
ORDER BY pending_count DESC;

-- Delivery pendente mais antigo
SELECT
    subscription_name,
    MIN(next_retry_at) AS oldest_pending,
    EXTRACT(EPOCH FROM (now() - MIN(next_retry_at))) AS age_seconds
FROM outbox_deliveries
WHERE status = 'pending'
GROUP BY subscription_name
ORDER BY age_seconds DESC;

-- Claimed mas não liberados (claimed sem processamento > HandlerTimeout)
SELECT
    claimed_by,
    subscription_name,
    COUNT(*) AS stuck_claimed,
    MIN(claimed_at) AS oldest_claim
FROM outbox_deliveries
WHERE status = 'claimed'
  AND claimed_at < now() - interval '2 minutes'
GROUP BY claimed_by, subscription_name
ORDER BY stuck_claimed DESC;
```

### Ações corretivas

| Sintoma | Ação |
|---|---|
| Pending cresce, processed_rate = 0, Dispatcher log ausente | Verificar `OUTBOX_DISPATCHER_ENABLED=true` e reiniciar worker |
| Pending cresce, processed_rate > 0, mas DLQ também cresce | Verificar handler — corrigir causa raiz, depois re-enfileirar DLQ (seção 3) |
| `claimed` stuck > HandlerTimeout | Reaper vai liberar automaticamente em até 1 min; se persistir, restart do worker |
| Reaper liberando > 50/10m | Worker está crashando antes de completar handlers — investigar OOM / pânico |
| Housekeeping deleted == 0 por 48h | Verificar log do cron e se `OutboxConfig.RetentionDays` está configurado corretamente |

---

## Plano de Rollout (RF-27)

### Deploy 1 — Código + migration com flag OFF

1. Merge do PR com toda a entrega; definir `OUTBOX_DISPATCHER_ENABLED=false` no env de produção.
2. Executar `cmd/migrate up` — aplica `0002_outbox.up.sql` (idempotente; seguro se tabelas já existirem).
3. Smoke test no worker: bootstrap inicia, registry valida, Subsystem reporta `outbox.dispatcher.disabled` no log.
4. Verificar com `psql`: `SELECT COUNT(*) FROM outbox_deliveries WHERE status='pending'` cresce conforme caller usa `Publisher.Publish`.
5. Tempo total estimado: 2h.

### Deploy 2 — Ativação após smoke staging

1. Em staging, definir `OUTBOX_DISPATCHER_ENABLED=true`, restart do worker.
2. Disparar 100 eventos via handler dummy e validar `processed` em < 5s.
3. Em produção (horário de baixa carga), idem; observar dashboard por 1h.
4. Critérios de "ok":
   - `outbox_delivery_latency_ms` p95 < 1s, p99 < 2s.
   - `outbox_deliveries_dlq_total` == 0.
   - `pg_stat_statements` mostra CPU/IO < 15% acima de baseline.
5. Tempo total estimado: 1h (após smoke).

### Critério de Rollback (RF-29, RTO < 2 min)

| Situação | Ação |
|---|---|
| p95 publish > 20% acima de baseline, ou DLQ disparando | `OUTBOX_DISPATCHER_ENABLED=false` + restart (operacional, seção 1) |
| Bug funcional do Dispatcher pós-flag ativo | `git revert` da merge commit; manter migration aplicada |
| Schema com problema confirmado e dados drenados | `cmd/migrate down` aplica `0002_outbox.down.sql` |

---

## Alertas Sugeridos (referência)

| Alerta | Expressão PromQL | Severidade |
|---|---|---|
| DLQ crescendo | `increase(outbox_deliveries_dlq_total[5m]) > 0` | warning |
| Fila travada | `outbox_deliveries_pending > 10 * avg_over_time(rate(outbox_deliveries_processed_total[10m])[5m:])` | critical |
| Latência p95 | `histogram_quantile(0.95, sum by (subscription_name, le) (rate(outbox_delivery_latency_ms_bucket[15m]))) > 1000` | warning |
| Housekeeping parado | `increase(outbox_housekeeping_deleted_total[48h]) == 0` | critical |
| Reaper hiperativo | `increase(outbox_reaper_released_total[10m]) > 50` | warning |
