# Auditoria de Infraestrutura — Production Readiness
**Data:** 2026-06-15
**Escopo:** mecontrola · VPS Hostinger + Docker + Go + PostgreSQL + Caddy

---

## Veredicto Executivo

| Dimensão | Status | Nota |
|----------|--------|------|
| Aplicação Go (código) | ✅ Production-ready | Sólido, sem gaps |
| Containers e Compose | ⚠️ Quase pronto | Faltam healthchecks em 3 serviços |
| Segurança de acesso | ✅ Production-ready | UFW, SCRAM-SHA-256, HSTS, headers OK |
| Backup e recuperação | ❌ Não-pronto para VPS | pgBackRest ausente; pg_dump script não substitui |
| Monitoramento e alertas | ❌ Não-pronto | Dashboard existe, alertas não existem |
| Endurecimento do OS | ❌ Gap crítico para VPS | fail2ban e unattended-upgrades ausentes |
| Connection pooling | ⚠️ Risco em carga alta | PgBouncer ausente; pool interno (MaxConns=10) |
| PostgreSQL tuning | ❌ Sem config de produção | Nenhum postgresql.conf no repositório |

**Resposta objetiva:**
- Para **Fly.io** (deployment atual): MVP robusto? **Sim.** Production-ready sem falso positivo? **Sim, com ressalva nos healthchecks.**
- Para **VPS Hostinger** (cenário do prompt): MVP robusto? **Não.** Production-ready? **Não.** Gaps críticos bloqueiam go-live.

---

## 1. Aplicação Go — ✅ PRODUCTION-READY

Sem gaps. Todas as 7 regras obrigatórias atendidas.

| Regra | Arquivo | Evidência |
|-------|---------|-----------|
| `/health` + `db.Ping()` 2s timeout | `cmd/server/server.go:109` | `WithHealthChecks({"database": dbManager.Ping})` |
| Graceful shutdown (SIGTERM) | `cmd/server/server.go:51` | `signal.NotifyContext(..., syscall.SIGTERM)` + `dbManager.Shutdown(15s)` |
| Env vars validadas no boot | `configs/config.go:542` | `Validate()` com crash explícito em config ausente |
| Logs JSON em stdout | `configs/config.go:493` | `LOG_FORMAT=json` default; `slog.NewJSONHandler(os.Stderr)` |
| Migrations separadas do startup | `cmd/migrate/migrate.go` | CLI separado; servidor usa `.migrations-disabled` no boot |
| Driver pgx/v5 (nunca database/sql) | `go.mod:11` | `github.com/jackc/pgx/v5 v5.10.0` via devkit-go |
| Connection string via env var | `configs/config.go:233` | `DBConfig.DSN()` composta de `DB_*` vars; nunca hardcoded |

**Ponto de atenção:** `db.Ping()` com timeout de 2s depende da implementação do devkit-go (`JailtonJunior94/devkit-go`). Verificar se `manager.Ping` respeita esse timeout.

---

## 2. Containers e Compose — ⚠️ QUASE PRONTO

### Conforme
- `restart: unless-stopped` em todos os serviços persistentes de produção (`compose.prod.yml`)
- Resource limits (cpu + memory) em server, worker, caddy e postgres
- Imagem Go multistage: builder `golang:alpine` + runtime `gcr.io/distroless/static-debian12:nonroot` (UID 65532)
- Porta da API não exposta ao host; tráfego exclusivo via Caddy
- Security hardening: `no-new-privileges`, `read_only: true`, `cap_drop: ALL`, `user: 65532:65532`

### Gap — Healthchecks ausentes em 3 serviços (ALTO)

Apenas `postgres` tem `healthcheck`. Os três abaixo não têm:

**`server`** — impede que o compose saiba se a API respondeu antes de receber tráfego:
```yaml
healthcheck:
  test: ["CMD-SHELL", "wget -qO- http://localhost:8080/health || exit 1"]
  interval: 10s
  timeout: 5s
  retries: 3
  start_period: 10s
```

**`worker`** — impede restart inteligente em caso de travamento:
```yaml
healthcheck:
  test: ["CMD-SHELL", "pgrep worker || exit 1"]
  interval: 30s
  timeout: 5s
  retries: 3
```

**`caddy`** — Traefik/proxy sem healthcheck gera falso-positivo de "tudo ok" quando reverse proxy caiu:
```yaml
healthcheck:
  test: ["CMD-SHELL", "wget -qO- http://localhost:80/health || exit 1"]
  interval: 10s
  timeout: 5s
  retries: 3
```

Arquivo a alterar: `deployment/compose/compose.yml`

---

## 3. Reverse Proxy — ⚠️ DIVERGÊNCIA ARQUITETURAL

O prompt manda **Traefik v3.3**. O projeto usa **Caddy 2**. Isso não é um problema per se — Caddy é uma escolha válida — mas gera divergência com as regras do prompt.

### Estado do Caddy

| Regra (equivalente Traefik) | Status Caddy | Observação |
|----------------------------|--------------|------------|
| HTTP→HTTPS redirect | ✅ Implícito | ACME email config force redirect |
| TLS mínimo TLS 1.2 | ✅ Padrão Caddy | Caddy 2 usa TLS 1.2+ por padrão |
| Security headers | ✅ Configurado | HSTS, nosniff, CSP, X-Frame-Options no Caddyfile |
| `exposedByDefault: false` | ✅ N/A | Caddy é config-explícita, sem auto-discovery |
| Rate limiting no edge | ❌ Ausente | Só rate limiting de aplicação (env vars); sem middleware Caddy |
| Dashboard com basicauth | ❌ Ausente | Caddy admin API não exposta, mas sem basicauth documentado |

**Recomendação:** Atualizar o prompt de infraestrutura para refletir Caddy como reverse proxy, ou planejar migração para Traefik — não fazer os dois em paralelo.

---

## 4. Backup — ❌ NÃO-PRONTO PARA VPS

### O que existe
- `deployment/scripts/pg-dump.sh` — lógica adequada:
  - `pg_dump` com compressão gzip
  - Criptografia via `age` (equivalente funcional de aes-256-cbc)
  - Upload S3-compatible (B2/R2) via rclone
  - Retenção configurável (padrão 30 dias)

### O que falta (gaps bloqueantes para VPS)

| Item | Status | Impacto |
|------|--------|---------|
| pgBackRest instalado | ❌ Ausente | Sem PITR, sem WAL archiving, sem restore incremental |
| `pgbackrest.conf` | ❌ Ausente | Sem stanza, sem repositório configurado |
| Cron: full domingo 01:00 | ❌ Ausente | Risco de perda de dados |
| Cron: diff seg-sáb 01:00 | ❌ Ausente | Risco de perda de dados |
| Cron: check domingo 03:00 | ❌ Ausente | Backups corrompidos passam despercebidos |
| Alerta se backup > 25h | ❌ Ausente | Falha silenciosa de backup |
| RTO medido e documentado | ❌ Ausente | Impossível comprometer SLA de recovery |
| Restore testado em VPS separada | ❌ Ausente | Backup sem teste não é backup |

**Nota para Fly.io:** O ambiente atual (Fly.io managed Postgres) tem backups gerenciados pela plataforma. O script `pg-dump.sh` é backup auxiliar. O risco aqui é para cenário VPS self-managed.

---

## 5. Monitoramento e Alertas — ❌ NÃO-PRONTO

### O que existe
- Dashboard Grafana: `deployment/grafana/mecontrola-platform.json`
  - Painel de HTTP latência, error rate, DB latência, health probes
- Métricas coletadas via OpenTelemetry + Prometheus
- Promtail configurado (`deployment/promtail/config.yml`) para coleta de logs

### O que falta — todos os alertas obrigatórios ausentes

**Monitoramento sem alerta é decoração — não é aceito** (regra explícita do prompt).

| Alerta obrigatório | Status |
|--------------------|--------|
| Disco > 80% | ❌ Ausente |
| RAM > 90% | ❌ Ausente |
| PostgreSQL não responde | ❌ Sem regra de alerta (métrica de health probe existe) |
| API 5xx > 1% das requests | ❌ Sem regra de alerta (métrica de error rate existe) |
| SSL expirando em < 30 dias | ❌ Ausente |
| Último backup > 25h | ❌ Ausente |
| Cache hit ratio < 95% | ❌ Ausente (não coletado) |
| Buffer hit ratio < 90% | ❌ Ausente (não coletado) |

**Arquivos ausentes:**
- `deployment/monitoring/prometheus-rules.yaml`
- `deployment/monitoring/alertmanager.yml`

---

## 6. Endurecimento do OS — ❌ NÃO-PRONTO PARA VPS

| Item | Status | Observação |
|------|--------|------------|
| UFW ativo com regras mínimas | ✅ Script disponível | `deployment/scripts/vps-firewall.sh` — SSH/80/443 |
| fail2ban | ❌ Ausente | Nenhuma referência no repositório |
| unattended-upgrades | ❌ Ausente | Nenhuma referência no repositório |

**Impacto:** VPS sem fail2ban é alvo direto de brute-force em SSH e HTTP. Sem unattended-upgrades, patches de segurança dependem de ação manual.

Ação: criar `deployment/scripts/vps-hardening.sh` que instala e configura ambos.

---

## 7. PgBouncer — ⚠️ RISCO EM CARGA ALTA

- Aplicação conecta diretamente no PostgreSQL sem pooler externo.
- Pool interno via devkit-go com `MaxConns=10` (padrão `.env.example`).
- Sem PgBouncer, em carga alta o PostgreSQL absorve `N_pods × MaxConns` conexões diretas, podendo ultrapassar `max_connections=100`.

**Para Fly.io:** risco menor (instância única, gerenciado).
**Para VPS com múltiplos deploys/workers:** gap real.

Ação: adicionar `deployment/pgbouncer/pgbouncer.ini` e service no compose configurado na porta 6432 (bare metal) ou container (dev).

---

## 8. PostgreSQL Tuning — ❌ SEM CONFIG DE PRODUÇÃO

Nenhum `postgresql.conf` no repositório com os parâmetros obrigatórios:

| Parâmetro | Regra | Status |
|-----------|-------|--------|
| `shared_buffers` | 25% da RAM | ❌ Não configurado |
| `effective_cache_size` | 75% da RAM | ❌ Não configurado |
| `random_page_cost` | 1.1 (NVMe SSD) | ❌ Não configurado |
| `effective_io_concurrency` | 200 | ❌ Não configurado |
| `log_min_duration_statement` | 200ms | ❌ Não configurado |
| `archive_mode` | on | ❌ Não configurado |
| `wal_level` | replica | ❌ Não configurado |
| `log_connections` | on | ❌ Não configurado |
| `log_line_prefix` | user/db/app/client | ❌ Não configurado |

Ação: criar `deployment/postgres/postgresql.conf` com valores parametrizados por RAM (ex: variáveis `${SHARED_BUFFERS}` calculadas no script de provisionamento).

---

## Checklist de Go-Live — VPS Hostinger

```
BLOQUEANTES (sem esses, não ir para produção)
[ ] Healthchecks em server, worker e caddy no Compose
[ ] pgBackRest instalado e backup full executado com sucesso
[ ] Restore testado em VPS separada, RTO medido e documentado
[ ] AlertManager com regras para disco, RAM, PostgreSQL, 5xx, SSL, backup
[ ] fail2ban e unattended-upgrades ativos

IMPORTANTES (resolver antes do primeiro usuário real)
[ ] PgBouncer configurado na porta 6432
[ ] postgresql.conf com tuning de produção
[ ] Rate limiting no Caddy (ou confirmar que app-level é suficiente)
[ ] Cron de backup (full + diff + check) verificado
[ ] Alertas testados: desligar serviço e confirmar disparo

JÁ OK (não retrabalar)
[x] Aplicação Go: /health, graceful shutdown, env validation, migrations, pgx/v5, JSON logs
[x] Compose prod: restart, resource limits, distroless, porta interna, security hardening
[x] .env.example completo, .env no .gitignore
[x] CI/CD: migrations separadas do deploy
[x] UFW com regras mínimas
[x] SCRAM-SHA-256, rede interna Docker, sem 0.0.0.0/0
[x] Security headers Caddy (HSTS, nosniff, CSP, X-Frame-Options)
```

---

## Resumo de Arquivos a Criar/Alterar

| Arquivo | Ação | Gap |
|---------|------|-----|
| `deployment/compose/compose.yml` | Alterar | Gap 1 — healthchecks |
| `deployment/monitoring/prometheus-rules.yaml` | Criar | Gap 5 — alertas |
| `deployment/monitoring/alertmanager.yml` | Criar | Gap 5 — alertas |
| `deployment/scripts/vps-hardening.sh` | Criar | Gap 6 — fail2ban + unattended-upgrades |
| `deployment/pgbouncer/pgbouncer.ini` | Criar | Gap 4 — PgBouncer |
| `deployment/pgbackrest/pgbackrest.conf` | Criar | Gap 3 — pgBackRest |
| `deployment/postgres/postgresql.conf` | Criar | Gap 7 — tuning |
| `deployment/caddy/Caddyfile` | Alterar | Gap 2 — rate limiting |
