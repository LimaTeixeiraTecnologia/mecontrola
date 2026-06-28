<!-- spec-hash-prd: fafa3f89f1c694ad4c5c845ccda25ece7ad70b700a055a94c8f770db67c76895 -->
<!-- MANDATÓRIO: preenchido por `create-technical-specification` Etapa 7.1 com sha256 do PRD consumido.
     Rastreabilidade: `create-tasks` e `execute-task` comparam este hash com o atual do prd.md
     para detectar drift entre techspec e PRD. NÃO remover este comentário ao editar a techspec. -->

# Especificação Técnica — Infraestrutura de Produção Robusta

## Resumo Executivo

Esta especificação técnica detalha a implementação da infraestrutura de produção robusta definida no PRD `infra-producao-robusta-10k-dez-2026`. A abordagem escolhida migra o deployment de **Docker Compose single-host** para **Docker Swarm single-node**, habilitando **2 réplicas de `server` e 2 de `worker` no início**, com caminho claro para escalar para 3 réplicas de cada após upgrade de VPS.

As decisões centrais são:

- Orquestração via **Docker Swarm single-node** com services nomeados (`server-1`, `server-2`, `worker-1`, `worker-2`) para health checks individuais.
- **Caddy** como proxy de borda com 2 upstreams explícitos, health checks ativos e rate limiting (`caddy-ratelimit`).
- Adição de um **servidor HTTP de health check interno no `worker`** na porta `8081`.
- **Docker secrets** para injeção de segredos sensíveis, criados a partir do `.env` via script de deploy.
- **Backup do `.env` no S3** (SSE-S3 + IAM restricto) a cada deploy.
- **Advisory lock no `cmd/migrate`** para garantir execução única de migrations.
- **PostgreSQL** com `archive_mode=on`, `statement_timeout=30s` e tuning ajustado.

> **Escopo desta tech spec:** infraestrutura, deployment, observabilidade e scripts operacionais. Não inclui mudanças de negócio ou de domínio.

---

## Arquitetura do Sistema

### Visão Geral dos Componentes

```text
Internet
   |
   v
Caddy (80/443) ── TLS + rate limit + headers de segurança
   |
   |-- health check --> server-1:8080
   |-- health check --> server-2:8080
   |
   v
server-1 / server-2 (app Go, port 8080)
   |
   |-- pgbouncer:6432 (pool de conexões)
   |
   v
postgres:5432 (PostgreSQL 16 + pgBackRest)
   |
   +-- WAL archive --> S3 (AWS us-east-1)
   +-- backups full/diff/incr --> S3

worker-1 / worker-2 (app Go, scheduler de jobs + health HTTP 8081)
   |
   +-- pgbouncer:6432
   +-- postgres:5432 (fila outbox, workflow kernel)

otel-lgtm (logs/métricas/traces)
   |
   +-- Grafana 3000 (localhost only)
   +-- Prometheus/Loki/Tempo

GitHub Actions (CI/CD)
   |
   +-- SSH --> VPS --> docker stack deploy
```

### Responsabilidades dos Componentes

| Componente | Responsabilidade |
|---|---|
| `server-1`, `server-2` | Receber requisições HTTP/webhook, orquestrar use cases, responder ao agente conversacional. |
| `worker-1`, `worker-2` | Processar jobs periódicos (outbox dispatcher, reapers, reconciliação, alertas). |
| `postgres` | Banco de dados relacional; fila de jobs via outbox; workflow kernel state. |
| `pgbouncer` | Pool de conexões PostgreSQL em transaction mode. |
| `pgbackrest` | Backups full/differential/incremental e PITR para S3. |
| `caddy` | Proxy reverso, TLS automático, load balancing, health checks, rate limit. |
| `otel-lgtm` | Coleta e visualização de logs, métricas e traces. |
| `migrate` | Aplica migrations do banco com advisory lock. |

### Fluxo de Dados

1. Requisição HTTP chega pelo Caddy (portas 80/443).
2. Caddy aplica rate limit por IP e encaminha para `server-1` ou `server-2` usando health checks ativos.
3. O server processa a requisição, grava no banco via pgbouncer e publica eventos na outbox.
4. O worker-1/worker-2 consomem a outbox e executam jobs assíncronos.
5. Métricas, logs e traces são enviados para o `otel-lgtm` via OTLP.
6. PostgreSQL arquiva WAL e realiza backups via pgBackRest para o S3.

---

## Design de Implementação

### 1. Docker Swarm

#### 1.1 Arquivo Compose para Swarm

Criar `deployment/compose/compose.swarm.yml` dedicado. Ele substituirá `compose.yml` + `compose.prod.yml` em produção. A estrutura inicial terá:

- Services: `postgres`, `pgbouncer`, `migrate`, `server-1`, `server-2`, `worker-1`, `worker-2`, `caddy`, `otel-lgtm`.
- Network `backend` do tipo `overlay` com `encrypted: true`.
- Network `frontend` do tipo `overlay` para exposição do Caddy.
- Volumes nomeados locais: `postgres-data`, `pgbackrest-repo`, `otel-lgtm-*`, `caddy-data`, `caddy-config`.

```yaml
networks:
  backend:
    driver: overlay
    encrypted: true
    attachable: false
  frontend:
    driver: overlay
    attachable: false
```

#### 1.2 Configuração de Deploy

Cada service nomeado terá:

```yaml
deploy:
  replicas: 1
  update_config:
    parallelism: 1
    delay: 20s
    failure_action: pause
    order: stop-first
  restart_policy:
    condition: any
    delay: 5s
    max_attempts: 3
    window: 120s
  resources:
    limits:
      cpus: '1.0'
      memory: 1G
    reservations:
      cpus: '0.25'
      memory: 128M
```

Os services `server-1`/`server-2` e `worker-1`/`worker-2` compartilham a mesma imagem (`ghcr.io/limateixeiratecnologia/mecontrola:${IMAGE_TAG}`), mas têm `command` ou `environment` distintos (`APP_MODE=server` vs `APP_MODE=worker`).

#### 1.3 Migração de Compose para Swarm

A migração ocorrerá em janela de manutenção única:

1. Notificar usuários pelo canal oficial.
2. Parar stack Compose atual (`docker compose down`).
3. Inicializar Swarm (`docker swarm init --advertise-addr <ip>`).
4. Criar Docker secrets a partir do `.env`.
5. Subir stack Swarm (`docker stack deploy -c compose.swarm.yml mecontrola`).
6. Executar migrations.
7. Validar health checks de `server-1`, `server-2`, `worker-1`, `worker-2`, Caddy.
8. Verificar logs e métricas.

> **Risco:** sem snapshot/rollback formal. Mitigação: testar exaustivamente em staging; manter backups S3 e configs no Git.

---

### 2. Caddy — Proxy de Borda

#### 2.1 Caddyfile

Atualizar `deployment/caddy/Caddyfile` para:

```caddy
{
    auto_https off
    admin off
}

(mecontrola-headers) {
    header -Server
    header X-Content-Type-Options nosniff
    header X-Frame-Options DENY
    header X-XSS-Protection "1; mode=block"
    header Referrer-Policy strict-origin-when-cross-origin
    header Permissions-Policy "geolocation=(), microphone=(), camera=()"
}

(rate-limit) {
    rate_limit {
        zone ip {
            key {remote_host}
            events 100
            window 1s
        }
        burst 200
        bad_request_code 429
    }
}

:80 {
    import mecontrola-headers
    import rate-limit

    reverse_proxy server-1:8080 server-2:8080 {
        lb_policy round_robin
        health_uri /healthz
        health_interval 10s
        health_timeout 5s
        health_status 200
        fail_duration 30s
        max_fails 3
    }
}

:443 {
    import mecontrola-headers
    import rate-limit

    tls {
        on_demand
    }

    reverse_proxy server-1:8080 server-2:8080 {
        lb_policy round_robin
        health_uri /healthz
        health_interval 10s
        health_timeout 5s
        health_status 200
        fail_duration 30s
        max_fails 3
    }
}
```

> Nota: o domínio real será configurado para TLS automático via Let's Encrypt. A configuração acima usa `on_demand` como placeholder; em produção real, substituir por domínio explícito.

#### 2.2 Rate Limiting

- Plugin `caddy-ratelimit` já está incluído no `Dockerfile.caddy`.
- Limite: 100 req/s por IP, burst 200.
- Código de retorno: 429 Too Many Requests.

---

### 3. Health Checks

#### 3.1 Server

O server já utiliza `devkit-go/pkg/http_server/chi_server` com health checks configurados em `cmd/server/server.go`:

```go
httpserver.WithHealthChecks(map[string]httpserver.HealthCheckFunc{
    "database": dbManager.Ping,
})
```

Endpoints disponíveis:
- `/healthz` — liveness (retorna 200 se processo vivo).
- `/readyz` — readiness do framework.
- `/readiness` — readiness customizado que retorna 503 durante shutdown.

**Ação necessária:** garantir que o endpoint de readiness reflita a capacidade real de atender tráfego (banco conectado, módulos inicializados). O endpoint `/readyz` do devkit-go já executa os health checks registrados.

#### 3.2 Worker

O worker atualmente não expõe HTTP. Será adicionado um servidor HTTP mínimo na porta `8081` para health checks:

```go
package worker

import (
    "context"
    "net/http"
    "time"
)

type healthServer struct {
    db      *sqlx.DB
    manager *worker.Manager
}

func (h *healthServer) livez(w http.ResponseWriter, _ *http.Request) {
    w.WriteHeader(http.StatusOK)
}

func (h *healthServer) readyz(w http.ResponseWriter, _ *http.Request) {
    ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
    defer cancel()

    if err := h.db.PingContext(ctx); err != nil {
        http.Error(w, err.Error(), http.StatusServiceUnavailable)
        return
    }
    w.WriteHeader(http.StatusOK)
}
```

O health server será iniciado em goroutine no `cmd/worker/worker.go` e encerrado graciosamente no shutdown.

---

### 4. Segredos e Configuração

#### 4.1 Docker Secrets

Criar script `deployment/scripts/create-secrets.sh` que lê o `.env` e cria/rotaciona secrets no Swarm:

```bash
#!/usr/bin/env bash
set -euo pipefail

ENV_FILE="${1:-.env}"
STACK="${STACK:-mecontrola}"

create_secret() {
  local name="$1"
  local value="$2"
  printf '%s' "$value" | docker secret create "${STACK}_${name}" - 2>/dev/null || \
    printf '%s' "$value" | docker secret create "${STACK}_${name}_$(date +%s)" - | \
    xargs -I {} docker service update --secret-rm "${STACK}_${name}" --secret-add "source={},target=${name}" "${STACK}_server-1"
}

# Exemplo para DB_PASSWORD
create_secret "DB_PASSWORD" "$(grep '^DB_PASSWORD=' "$ENV_FILE" | cut -d= -f2-)"
# ... outros segredos
```

> A rotação de secrets no Swarm requer recriar o secret com novo nome e fazer `docker service update --secret-rm/--secret-add`. O script deve orquestrar isso.

Mapeamento no `compose.swarm.yml`:

```yaml
services:
  server-1:
    secrets:
      - source: mecontrola_db_password
        target: DB_PASSWORD
        mode: 0440
```

A aplicação Go continuará lendo variáveis de ambiente. O Docker Swarm monta secrets em `/run/secrets/<target>` e pode ser exposto como env via `env_file` ou entrypoint. A abordagem mais simples é manter o `.env` como fonte de verdade no host, criar secrets, e montar os secrets como arquivos que a aplicação lê via wrapper.

**Decisão prática:** como a aplicação já lê `.env` via viper, a transição menos invasiva é:

1. Manter `.env` no host (chmod 600).
2. No `compose.swarm.yml`, montar o `.env` como `env_file` para cada service.
3. Criar Docker secrets apenas para os segredos mais críticos (DB_PASSWORD, tokens de APIs) e montá-los como variáveis de ambiente.

Isso atende parcialmente RF-10 no início e pode evoluir para 100% secrets.

#### 4.2 Backup do `.env` no S3

Hook em `deployment/scripts/deploy.sh` e `deploy-local.sh`:

```bash
backup_env_to_s3() {
  local env_file="${VPS_DEPLOY_PATH}/.env"
  local s3_uri="s3://${PGBACKREST_S3_BUCKET}/mecontrola-env-backups/.env-$(date -u +%Y%m%d-%H%M%S)"
  aws s3 cp "$env_file" "$s3_uri" --sse AES256 --storage-class STANDARD || true
}
```

Requisitos:
- AWS CLI instalado na VPS.
- IAM restricto permitindo apenas `PutObject` no prefixo `mecontrola-env-backups/`.
- Variáveis `AWS_ACCESS_KEY_ID` e `AWS_SECRET_ACCESS_KEY` configuradas no `.env` ou via Docker secrets.

---

### 5. Banco de Dados e pgBackRest

#### 5.1 PostgreSQL

Ajustes em `deployment/postgres/postgresql.conf`:

```conf
# Conexões
max_connections = 100
superuser_reserved_connections = 3

# WAL / PITR
wal_level = replica
archive_mode = on
archive_command = 'pgbackrest --stanza=mecontrola archive-push %p'

# Proteção
statement_timeout = 30s

# Performance (ajustar conforme KVM2)
shared_buffers = 512MB
effective_cache_size = 1.5GB
work_mem = 16MB
maintenance_work_mem = 128MB
```

> `archive_mode=on` requer reinício do PostgreSQL. Será ativado via `pgbackrest-setup.sh` (fase 1) e restart do container.

#### 5.2 pgBouncer

Ajustes em `deployment/pgbouncer/pgbouncer.ini`:

```ini
[databases]
mecontrola_db = host=postgres port=5432 dbname=mecontrola_db

[pgbouncer]
pool_mode = transaction
listen_port = 6432
listen_addr = 0.0.0.0
auth_type = md5
auth_file = /etc/pgbouncer/userlist.txt
max_client_conn = 300
default_pool_size = 15
max_db_connections = 60
reserve_pool_size = 5
reserve_pool_timeout = 3
```

> Considerando 2 servers + 2 workers + migrate + admin, `max_db_connections=60` mantém folga sobre `max_connections=100`.

#### 5.3 pgBackRest

Configuração existente em `deployment/pgbackrest/pgbackrest.conf` já aponta para S3. Ajustes:

```ini
[global]
repo1-type=s3
repo1-s3-bucket=mecontrola-backups
repo1-s3-region=us-east-1
repo1-s3-endpoint=s3.amazonaws.com
repo1-path=/pgbackrest
repo1-retention-full=4
repo1-retention-diff=7
repo1-retention-archive-type=diff
repo1-retention-archive=2
repo1-cipher-type=aes-256-cbc
start-fast=y
stop-auto=y

[mecontrola]
pg1-path=/var/lib/postgresql/data
pg1-port=5432
pg1-database=mecontrola_db
```

Cron do postgres:

```cron
# Full semanal (domingo 02:00)
0 2 * * 0 PGBACKREST_S3_KEY=... PGBACKREST_S3_KEY_SECRET=... pgbackrest --stanza=mecontrola --type=full backup
# Diff diário
0 2 * * 1-6 PGBACKREST_S3_KEY=... PGBACKREST_S3_KEY_SECRET=... pgbackrest --stanza=mecontrola --type=diff backup
# Incremental a cada 6h
0 */6 * * * PGBACKREST_S3_KEY=... PGBACKREST_S3_KEY_SECRET=... pgbackrest --stanza=mecontrola --type=incr backup
```

---

### 6. Migrations — Advisory Lock

Implementar advisory lock em `cmd/migrate/migrate.go` antes de `migrator.Up()`:

```go
const migrationAdvisoryLockID int64 = 424242

func acquireMigrationLock(ctx context.Context, db *sql.DB) (func(), error) {
    var acquired bool
    if err := db.QueryRowContext(ctx, "SELECT pg_try_advisory_lock($1)", migrationAdvisoryLockID).Scan(&acquired); err != nil {
        return nil, fmt.Errorf("advisory lock query: %w", err)
    }
    if !acquired {
        return nil, fmt.Errorf("outro processo de migrate esta em execucao")
    }
    return func() {
        _, _ = db.ExecContext(context.Background(), "SELECT pg_advisory_unlock($1)", migrationAdvisoryLockID)
    }, nil
}
```

Uso:

```go
unlock, err := acquireMigrationLock(ctx, r.dbManager.DB())
if err != nil {
    return fmt.Errorf("migrate: %w", err)
}
defer unlock()

if err := migrator.Up(); err != nil { ... }
```

---

### 7. CI/CD e Deploy

#### 7.1 GitHub Actions

Adaptar `.github/workflows/ci-cd.yml` para executar `docker stack deploy` via SSH:

```yaml
deploy:
  name: Deploy Swarm Stack
  runs-on: ubuntu-24.04
  needs: [build-image, scan-image, sign-image]
  if: github.ref == 'refs/heads/main'
  environment: production
  steps:
    - uses: actions/checkout@...
    - name: Deploy to Swarm
      env:
        VPS_HOST: ${{ secrets.VPS_HOST }}
        VPS_USER: ${{ secrets.VPS_USER }}
        VPS_SSH_KEY: ${{ secrets.VPS_SSH_KEY }}
        VPS_DEPLOY_PATH: ${{ secrets.VPS_DEPLOY_PATH }}
        IMAGE_TAG: ${{ needs.build-image.outputs.image-tag }}
      run: bash deployment/scripts/deploy-swarm.sh "$IMAGE_TAG"
```

#### 7.2 Script deploy-swarm.sh

Criar `deployment/scripts/deploy-swarm.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail

IMAGE_TAG="${1:?IMAGE_TAG obrigatorio}"
VPS_HOST="${VPS_HOST:?}"
VPS_USER="${VPS_USER:?}"
VPS_DEPLOY_PATH="${VPS_DEPLOY_PATH:?}"
VPS_SSH_KEY="${VPS_SSH_KEY:-}"

SSH_OPTS=(-o BatchMode=yes -o StrictHostKeyChecking=accept-new -o ConnectTimeout=10)
[[ -n "$VPS_SSH_KEY" ]] && SSH_OPTS+=(-i "$VPS_SSH_KEY")

ssh "${SSH_OPTS[@]}" "${VPS_USER}@${VPS_HOST}" \
  bash -s -- "$IMAGE_TAG" "$VPS_DEPLOY_PATH" <<'REMOTE'
set -euo pipefail
TAG="$1"; DP="$2"
export IMAGE_TAG="$TAG"

cd "$DP"
git pull --ff-only

# Criar/atualizar secrets
bash deployment/scripts/create-secrets.sh "$DP/.env"

# Backup .env para S3
bash deployment/scripts/backup-env-s3.sh "$DP/.env"

# Migrate com advisory lock
docker stack deploy -c "$DP/deployment/compose/compose.swarm.yml" -c "$DP/deployment/compose/compose.swarm.migrate.yml" mecontrola-migrate || true
docker service logs mecontrola-migrate --follow || true

# Deploy stack principal
docker stack deploy -c "$DP/deployment/compose/compose.swarm.yml" mecontrola

# Aguardar health checks
for svc in server-1 server-2 worker-1 worker-2; do
  until docker service ps "mecontrola_${svc}" --format '{{.CurrentState}}' | grep -q 'Running'; do
    echo "aguardando $svc..."; sleep 5
  done
done

echo "Deploy concluido: $TAG"
REMOTE
```

> **Nota:** a execução do migrate via Swarm service requer cuidado para garantir que o service execute uma única vez e termine. Alternativa: executar `docker run --rm` com a imagem do migrate, similar ao fluxo atual.

#### 7.3 deploy-local.sh para Swarm

Adaptar para usar `docker stack deploy` em vez de `docker compose up`.

---

### 8. Observabilidade

#### 8.1 Métricas e Alertas

Manter `otel-lgtm` com retenção curta:
- Logs: 7 dias
- Métricas: 15 dias
- Traces: 7 dias

Adicionar/ajustar alertas no Grafana:
- CPU > 70% por 5 min
- RAM > 80%
- Disco > 80%
- WAL lag > 15 min
- Fila de jobs outbox > 1000
- Latência API p95 > 1s
- Taxa de erro 5xx > 0.5%

#### 8.2 Health Checks no Swarm

Configurar `healthcheck` em cada service do `compose.swarm.yml`:

```yaml
healthcheck:
  test: ["CMD", "wget", "-q", "--spider", "http://localhost:8080/healthz"]
  interval: 10s
  timeout: 5s
  retries: 3
  start_period: 30s
```

Para worker:

```yaml
healthcheck:
  test: ["CMD", "wget", "-q", "--spider", "http://localhost:8081/readyz"]
```

---

## Pontos de Integração

### AWS S3

- Bucket `mecontrola-backups` em `us-east-1`.
- Storage class `STANDARD` com lifecycle para `Glacier` após 90 dias.
- Criptografia SSE-S3.
- Credenciais via IAM restricto ou via secrets no `.env`.

### GitHub Actions

- Build e push de imagem para GHCR.
- Scan com Trivy.
- Assinatura com Cosign.
- Deploy via SSH + `docker stack deploy`.

### Telegram

- Alertas do Grafana via webhook do Telegram.
- Notificações de deploy (já existente no CI/CD).

---

## Abordagem de Testes

### Testes Unitários

- Testar o advisory lock em `cmd/migrate` com banco em memória ou mock.
- Testar o health server do worker.
- Testar scripts de shell com `bats` ou validação sintática (`shellcheck`).

### Testes de Integração

- Subir stack Swarm em ambiente de staging.
- Validar health checks de todos os services.
- Executar restore PITR em instância isolada.
- Testar deploy e rollback manual.

### Testes E2E

- Fluxo completo de webhook WhatsApp -> server -> outbox -> worker -> resposta.
- Verificar que múltiplos workers não processam o mesmo job.

---

## Sequenciamento de Desenvolvimento

### Ordem de Build

1. **Criar `compose.swarm.yml`** com services nomeados, networks overlay e secrets.
2. **Atualizar `Caddyfile`** com 2 upstreams, health checks ativos e rate limit.
3. **Adicionar health server ao worker** (`cmd/worker/health.go`).
4. **Implementar advisory lock em `cmd/migrate/migrate.go`**.
5. **Criar scripts auxiliares**:
   - `deployment/scripts/create-secrets.sh`
   - `deployment/scripts/backup-env-s3.sh`
   - `deployment/scripts/deploy-swarm.sh`
6. **Atualizar `deploy.sh` e `deploy-local.sh`** para Swarm.
7. **Ajustar CI/CD** (`ci-cd.yml`) para chamar `deploy-swarm.sh`.
8. **Atualizar PostgreSQL config** (`archive_mode=on`, `statement_timeout=30s`).
9. **Atualizar runbooks** (`deploy.md`, `restore-pitr.md`, `rollback.md`).
10. **Testar em staging** e migrar produção.

### Dependências Técnicas

- Bucket S3 criado e credenciais configuradas.
- Docker Swarm inicializado na VPS.
- Domínio apontado para a VPS.
- Credenciais SSH configuradas no GitHub Actions.

---

## Monitoramento e Observabilidade

### Métricas a Expor

- `http_request_duration_seconds` (latência API)
- `http_requests_total` (por status)
- `outbox_pending_jobs` (fila de jobs)
- `worker_job_executions_total`
- `db_connections_active`
- `pgbouncer_pools_active`

### Logs

- Estruturados em JSON.
- Nível `info` em produção.
- Campos obrigatórios: `service`, `version`, `trace_id`, `error`.

### Dashboards

- Reutilizar dashboards Grafana existentes em `deployment/dashboards/`.
- Adicionar painel de health checks e fila de jobs.

---

## Considerações Técnicas

### Decisões Chave

| Decisão | Escolha | Justificativa | ADR |
|---|---|---|---|
| Orquestração | Docker Swarm single-node | Suporta réplicas e rolling updates sem complexidade de Kubernetes. | [ADR-001](adr-001-docker-swarm-single-node.md) |
| Nomenclatura de services | `server-1`/`server-2` separados | Permite health checks individuais no Caddy e controle fino. | [ADR-002](adr-002-services-separados.md) |
| Secrets | Docker secrets + `.env` no host | Balanceamento entre segurança e simplicidade de transição. | [ADR-003](adr-003-docker-secrets.md) |
| Worker health check | Servidor HTTP interno na porta 8081 | Padrão compatível com Swarm e Caddy. | [ADR-004](adr-004-worker-health-http.md) |

### Alternativas Rejeitadas

- **Kubernetes (k3s):** complexidade excessiva para VPS única.
- **Docker Compose com services duplicados manualmente:** não oferece rolling updates nativos.
- **Traefik em vez de Caddy:** exigiria trocar proxy já maduro no projeto.
- **Vault/Secret Manager externo:** fora do orçamento zero.

### Riscos Conhecidos

- **Swarm single-node não é HA:** falha do host ainda indisponibiliza tudo. Mitigado por backups S3.
- **Oversubscription de CPU na KVM2:** 2+2 réplicas exigem ~4,75 vCPU em 2 disponíveis. Requer monitoramento e upgrade.
- **Migração sem snapshot:** falha na migração exige recuperação manual. Mitigado por testes em staging.
- **Rotação de Docker secrets:** exige recriar secrets com nomes únicos e atualizar services. Script deve automatizar.

---

## Conformidade com Padrões

- **AGENTS.md**: manter fronteiras entre infraestrutura e domínio; não introduzir dependências de domínio na infraestrutura.
- **R-SEC-001**: segredos não em código; paths normalizados; comandos shell com argumentos explícitos.
- **R-TEST-001**: testes determinísticos; cobertura de caminho feliz e falha; testes de integração para IO.

---

## Arquivos Relevantes e Dependentes

- `.specs/prd-infra-producao-robusta-10k-dez-2026/prd.md`
- `deployment/compose/compose.yml`
- `deployment/compose/compose.prod.yml`
- `deployment/compose/compose.swarm.yml` (novo)
- `deployment/caddy/Caddyfile`
- `deployment/caddy/Dockerfile.caddy`
- `deployment/postgres/postgresql.conf`
- `deployment/pgbouncer/pgbouncer.ini`
- `deployment/pgbackrest/pgbackrest.conf`
- `deployment/docker/Dockerfile`
- `cmd/server/server.go`
- `cmd/worker/worker.go`
- `cmd/migrate/migrate.go`
- `deployment/scripts/deploy.sh`
- `deployment/scripts/deploy-local.sh`
- `deployment/scripts/deploy-swarm.sh` (novo)
- `deployment/scripts/create-secrets.sh` (novo)
- `deployment/scripts/backup-env-s3.sh` (novo)
- `.github/workflows/ci-cd.yml`
- `deployment/runbooks/deploy.md`
- `deployment/runbooks/restore-pitr.md`
- `deployment/runbooks/restore-vps.md`
- `deployment/runbooks/rollback.md`
