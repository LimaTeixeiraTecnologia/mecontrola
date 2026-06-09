# Runbook: Auth Module — Incident Response

## Visão Geral

Este runbook cobre os 3 cenários de alerta principais do módulo de autenticação Auth Foundation. Cada cenário inclui: sintomas, diagnóstico, remediação e escalação.

**Dashboard de referência:** Grafana → "Auth Module"
**Contato de escalação:** Eng. Platform via canal #incidents

---

## Cenário 1 — Pico de assinaturas inválidas

**Alerta:** `auth_failed_total{reason="invalid_signature"} > 0 em 5 min`

### Sintomas

- Webhook Meta retornando 401 para requisições legítimas.
- Usuários reportando ausência de resposta no WhatsApp.
- Métrica `meta_signature_status_total{status="valid"}` zerada.

### Diagnóstico

```bash
# 1. Verificar métricas recentes
promql: rate(auth_failed_total{reason="invalid_signature"}[5m])

# 2. Verificar se houve rotação recente (checar log de deployments)
promql: meta_signature_status_total{status="rotated"}

# 3. Checar env vars na VPS
ssh $VPS_USER@$VPS_HOST
grep META_APP_SECRET $VPS_DEPLOY_PATH/.env
```

### Remediação

**Caso A — Secret incorreto após rotação:**
```bash
# Reverter conforme runbook auth-meta-secret-rotation.md Seção "Rollback"
```

**Caso B — Secret corrompido ou expirado no Meta:**
```bash
# Gerar novo secret no Meta Business Manager
# Seguir procedimento completo em auth-meta-secret-rotation.md
```

**Caso C — Ataque de spoofing:**
```bash
# Verificar IPs na VPS (logs do nginx/caddy)
# Bloquear IP se confirmado ataque
# Registrar fingerprint do payload malicioso nos logs de incidente
```

### Critério de Resolução

- `auth_failed_total{reason="invalid_signature"}` retorna a zero por 5 minutos consecutivos.

---

## Cenário 2 — Banco de dados indisponível

**Alerta:** `auth_failed_total{reason="db_unavailable"} > 1 em 1 min`

### Sintomas

- Webhook retornando 503 ao Meta (Meta reenvia com retry).
- Logs contendo `identity.usecase.establish_principal_failed` + `db_unavailable`.
- Todas as métricas de auth em queda (nenhum established_total incrementando).

### Diagnóstico

```bash
# 1. Verificar conectividade com Postgres
ssh $VPS_USER@$VPS_HOST
docker compose -f $VPS_DEPLOY_PATH/docker-compose.yml ps

# 2. Verificar logs do Postgres
docker compose -f $VPS_DEPLOY_PATH/docker-compose.yml logs --tail=100 postgres

# 3. Verificar uso de conexões
psql $DB_URL -c "SELECT count(*) FROM pg_stat_activity WHERE state = 'active';"

# 4. Verificar espaço em disco
df -h
```

### Remediação

**Caso A — Postgres parado:**
```bash
docker compose -f $VPS_DEPLOY_PATH/docker-compose.yml start postgres
# Aguardar 30s e verificar conectividade
```

**Caso B — Connection pool esgotado:**
```bash
# Reiniciar a aplicação para liberar conexões ociosas
docker compose -f $VPS_DEPLOY_PATH/docker-compose.yml up -d --no-deps --force-recreate api
```

**Caso C — Disco cheio:**
```bash
# Liberar espaço (logs antigos, backups expirados)
find $VPS_DEPLOY_PATH/logs -mtime +7 -delete

# Se auth_events cresceu demais, verificar housekeeping
psql $DB_URL -c "SELECT count(*), min(occurred_at) FROM auth_events;"
# Housekeeping roda mensalmente — se necessário, rodar manualmente via task
```

### Critério de Resolução

- Postgres acessível.
- `auth_failed_total{reason="db_unavailable"}` zerado.
- `auth_principal_established_total` incrementando normalmente.

---

## Cenário 3 — Outbox com falhas de publicação

**Alerta:** `outbox_publish_failed_total{kind=~"auth\\..*"} > 0`

### Sintomas

- Eventos de auth não chegando à tabela `auth_events`.
- Consumer de outbox com erros nos logs.
- Linhas acumulando em `platform_outbox_events` sem ser consumidas.

### Diagnóstico

```bash
# 1. Verificar fila de outbox pendentes
psql $DB_URL -c "
  SELECT type, count(*), max(occurred_at)
  FROM platform_outbox_events
  WHERE processed_at IS NULL
  GROUP BY type
  ORDER BY max(occurred_at) DESC
  LIMIT 20;
"

# 2. Verificar logs do consumer
# Filtrar por: identity.consumer.auth_events + ERROR level

# 3. Verificar se o consumer está registrado e ativo
# logs: consumer inicializa com "auth_events_consumer registered"
```

### Remediação

**Caso A — Consumer parado por crash:**
```bash
docker compose -f $VPS_DEPLOY_PATH/docker-compose.yml up -d --no-deps --force-recreate api
# Consumer reinicia com o servidor
```

**Caso B — Payload corrompido bloqueando o consumer:**
```bash
# Identificar event_id problemático nos logs
psql $DB_URL -c "
  UPDATE platform_outbox_events
  SET processed_at = now(), error = 'skipped-manually-incident-YYYY-MM-DD'
  WHERE id = '<event_id_problemático>';
"
```

**Caso C — Banco sobrecarregado (consumer lento):**
- Verificar `auth_events_housekeeping_deleted_total` — housekeeping pode estar atrasado.
- Se necessário, executar housekeeping manualmente via `psql`.

### Critério de Resolução

- `outbox_publish_failed_total{kind=~"auth\\..*"}` zerado.
- Fila de outbox pendentes reduzida a zero ou crescimento estabilizado.
- Tabela `auth_events` recebendo inserções normalmente.

---

## Escalação

| Severidade | Critério | Ação |
|-----------|---------|------|
| P1 | Cenário 2 por > 10 min com retry storm do Meta | Ligar para eng. on-call |
| P2 | Cenário 1 por > 15 min | Abrir incidente no Grafana Incident |
| P3 | Cenário 3 por > 30 min | Slack #incidents com contexto |

---

## Variáveis de Ambiente Referenciadas

| Variável | Descrição |
|----------|-----------|
| `META_APP_SECRET` | Secret atual para validação HMAC |
| `META_APP_SECRET_NEXT` | Slot NEXT durante rotação |
| `STAGING_SMOKE_WA` | Número WhatsApp E.164 do usuário de smoke em staging |
| `DB_URL` | Connection string Postgres (formato `postgres://user:pass@host/db`) |
| `VPS_HOST` | Hostname ou IP da VPS Hostinger |
| `VPS_USER` | Usuário SSH na VPS |
| `VPS_DEPLOY_PATH` | Caminho do deploy na VPS |

---

## Links Úteis

- Runbook de rotação de secret: [auth-meta-secret-rotation.md](./auth-meta-secret-rotation.md)
- Dashboard Grafana: "Auth Module"
- Smoke test: `task auth:smoke`
