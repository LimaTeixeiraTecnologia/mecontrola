# VPS Deploy — Continuação

**Data:** 2026-06-16
**VPS:** 187.77.45.48 (Ubuntu 24.04, KVM 2, 8GB RAM)
**Domínio:** api.mecontrola.app.br → 187.77.45.48 (DNS propagado)
**Repo:** /opt/mecontrola (clonado, .env configurado)

## Estado atual (2026-06-16 ~21:00 UTC-3)

- [x] Docker instalado (v29.5.3)
- [x] Hardening executado (fail2ban, UFW, SSH key-only, swap 2GB)
- [x] Repo clonado em /opt/mecontrola
- [x] .env de produção configurado (todos os secrets)
- [x] DNS api.mecontrola.app.br → 187.77.45.48
- [x] Kiwify webhook registrado (URL + token)
- [x] Meta App criado (MeControla, modo desenvolvimento)
- [x] Imagem Docker construída na VPS (tag `:local`)
- [x] Migrations aplicadas (inclui 000002 — smoke user seed)
- [x] postgres rodando (healthy)
- [x] pgbouncer rodando (healthy)
- [x] server rodando (healthy) — imagem `:local`
- [x] worker rodando (healthy) — imagem `:local`
- [x] caddy rodando (TLS Let's Encrypt obtido, HTTPS funcionando)
- [x] `curl https://api.mecontrola.app.br/health` → `{"status":"healthy"}`
- [x] Caddy healthcheck corrigido no código (`--spider`, 30s) — aplica no próximo deploy via CI/CD
- [x] GitHub Environment `staging` criado
- [x] GitHub Secrets configurados (10 secrets no environment `staging`): `VPS_HOST`, `VPS_USER`, `VPS_DEPLOY_PATH`, `VPS_SSH_KEY`, `STAGING_WEBHOOK_URL`, `STAGING_META_APP_SECRET`, `STAGING_SMOKE_WA`, `STAGING_DB_URL`, `GHCR_USER`, `GHCR_TOKEN`
- [x] Meta webhook verificado (`/api/v1/whatsapp/inbound` — GET + POST) — `Configurar webhooks` ✅ na Etapa 2
- [x] Bot respondendo mensagens (onboarding funcionando com número de teste)
- [ ] **CI/CD pipeline passando** — 3 fixes pushados (1101eab), CI rodando
- [ ] **Meta número real verificado** — `+55 11 93621-2870` adicionado no Gerenciador ("Não verificado"), aguardando OTP (rate limit de SMS — tentar novamente amanhã via ligação)
- [ ] META_PHONE_NUMBER_ID e META_ACCESS_TOKEN atualizados no .env da VPS
- [ ] pgBackRest S3 configurado

---

## Fixes aplicados durante o deploy (contexto para debug futuro)

### 1. pgbouncer — imagem removida do Docker Hub
- **Problema:** `bitnami/pgbouncer` removeu todas as imagens do Docker Hub.
- **Fix:** migrado para `edoburu/pgbouncer:v1.25.2-p0` (ativa, amd64+arm64).
- **Atenção:** variáveis de ambiente são diferentes das bitnami:
  - `POSTGRESQL_HOST` → `DB_HOST`
  - `POSTGRESQL_USERNAME` → `DB_USER`
  - `POSTGRESQL_PASSWORD` → `DB_PASSWORD`
  - `POSTGRESQL_DATABASE` → `DB_NAME`
  - `PGBOUNCER_PORT` → `LISTEN_PORT`
  - `PGBOUNCER_POOL_MODE` → `POOL_MODE`
  - `PGBOUNCER_MAX_CLIENT_CONN` → `MAX_CLIENT_CONN`
  - `PGBOUNCER_AUTH_TYPE` → `AUTH_TYPE`

### 2. postgresql.conf sem listen_addresses
- **Problema:** postgres ouvia só em `localhost` (padrão), recusando conexões Docker da rede `backend`. Healthcheck (`pg_isready` via socket Unix) mostrava "Healthy" mesmo assim.
- **Fix:** adicionado `listen_addresses = '*'` em `deployment/postgres/postgresql.conf`.

### 3. AUTH_TYPE md5 vs scram-sha-256
- **Problema:** postgres inicializado com `--auth-host=scram-sha-256` (compose.prod.yml), pgbouncer configurado com `AUTH_TYPE: md5` → `FATAL: wrong password type (SQLSTATE 08P01)`.
- **Fix:** `AUTH_TYPE: scram-sha-256` no compose.yml.

### 4. OTEL insecure em production
- **Problema:** `devkit-go@v0.5.0` bloqueia `OTEL_EXPORTER_OTLP_INSECURE=true` quando `ENVIRONMENT=production`. Default do config era `true`.
- **Fix:** adicionar `OTEL_EXPORTER_OTLP_INSECURE=false` no `.env` da VPS.
- **Efeito colateral:** OTEL não exporta telemetria (otelcol sem TLS). Não bloqueia a app.
- **Fix definitivo futuro:** configurar endpoint OTLP externo com TLS (Grafana Cloud).

### 5. IDENTITY_GATEWAY_SHARED_SECRET e ONBOARDING_TOKEN_ENCRYPTION_KEY
- **Problema:** valores placeholder ou ausentes no `.env`.
- **Fix:** gerar e injetar no `.env` da VPS:
  ```bash
  GATEWAY_SECRET=$(openssl rand -hex 32)
  ONBOARDING_KEY=$(openssl rand -base64 32 | tr -d '\n')
  sed -i '/^IDENTITY_GATEWAY_SHARED_SECRET_CURRENT=/d' /opt/mecontrola/.env
  sed -i '/^IDENTITY_GATEWAY_SHARED_SECRET_NEXT=/d' /opt/mecontrola/.env
  sed -i '/^ONBOARDING_TOKEN_ENCRYPTION_KEY=/d' /opt/mecontrola/.env
  echo "IDENTITY_GATEWAY_SHARED_SECRET_CURRENT=${GATEWAY_SECRET}" >> /opt/mecontrola/.env
  echo "IDENTITY_GATEWAY_SHARED_SECRET_NEXT=${GATEWAY_SECRET}" >> /opt/mecontrola/.env
  echo "ONBOARDING_TOKEN_ENCRYPTION_KEY=${ONBOARDING_KEY}" >> /opt/mecontrola/.env
  ```
- **Requisitos de tamanho:**
  - `IDENTITY_GATEWAY_SHARED_SECRET_CURRENT`: hex, mínimo 64 chars (32 bytes)
  - `ONBOARDING_TOKEN_ENCRYPTION_KEY`: exatamente 32, 43 ou 44 chars (base64 de 32 bytes = 44 chars)

### 6. wget/pgrep quebrados na imagem distroless
- **Problema:** Dockerfile copiava `/usr/bin/wget` e `/usr/bin/pgrep` do Alpine (symlinks do BusyBox linkado com musl). Imagem final é `gcr.io/distroless/static-debian12:nonroot` (glibc) → `exec /usr/bin/wget: no such file or directory`.
- **Fix:** Dockerfile agora instala `busybox-static` e copia `/bin/busybox.static` como `wget` e `pgrep`:
  ```dockerfile
  FROM alpine:3.20 AS tools
  RUN apk add --no-cache busybox-static
  # ...
  COPY --from=tools /bin/busybox.static /usr/bin/wget
  COPY --from=tools /bin/busybox.static /usr/bin/pgrep
  ```

### 7. caddy sem env_file (CADDY_EMAIL e APP_DOMAIN vazios)
- **Problema:** caddy não tinha `env_file`, então `{$CADDY_EMAIL}` e `{$APP_DOMAIN}` expandiam para string vazia → erro de parse no Caddyfile.
- **Fix:** adicionado `env_file: ../../.env` ao serviço caddy no compose.yml.
- **Valores no .env da VPS:**
  - `APP_DOMAIN=api.mecontrola.app.br`
  - `CADDY_EMAIL=jailton.junior94@outlook.com`

### 8. caddy healthcheck unhealthy — CORRIGIDO NO CÓDIGO
- **Causa:** `wget -qO-` gravava body no stdout; flag correta para health check é `--spider` (HEAD request sem output).
- **Fix aplicado** em `deployment/compose/compose.yml`:
  - `["CMD-SHELL", "wget -qO- ..."]` → `["CMD", "wget", "--spider", "-q", "http://localhost:2019/"]`
  - `interval` alterado de 10s para 30s
  - `start_period` alterado de 10s para 30s
- **Aplica na VPS:** após o próximo deploy via CI/CD.

### 9. Meta webhook — GET /inbound ausente
- **Problema:** A Meta envia GET (verificação do hub.challenge) e POST (eventos) para a mesma callback URL. O router tinha `GET /verify` e `POST /inbound` em rotas separadas → Meta recebia 405 ao verificar `/inbound`.
- **Fix:** adicionado `sub.Get("/inbound", rt.verifyHandler.Handle)` em `whatsapp_router.go`.
- **Callback URL correta:** `https://api.mecontrola.app.br/api/v1/whatsapp/inbound` (GET + POST na mesma rota).

### 10. Migration 000002 — seed do smoke user ausente
- **Problema:** smoke test hardcoda `user_id = '00000000-0000-0000-0000-00005a17c8e7'` mas a migration de seed nunca foi mergeada (existia só em worktrees). Sem ela, webhook cria usuário com UUID aleatório → smoke test falha.
- **Fix:** `migrations/000002_seed_smoke_user_staging.up.sql` — insere smoke user se `app.smoke_wa` estiver configurado (skip silencioso em produção).
- **deploy.sh:** configura `app.smoke_wa` no postgres antes do migrate quando `STAGING_SMOKE_WA` está no env.

### 11. CI/CD — DB_PASSWORD missing no docker compose pull
- **Problema:** `docker compose pull` sem `--env-file` explícito → `DB_PASSWORD is required` no runner do CI.
- **Fix:** `deploy.sh` passa `COMPOSE_ENV="--env-file ${VPS_DEPLOY_PATH}/.env"` em todos os comandos compose.

### 12. CI/CD — GHCR auth na VPS
- **Problema:** imagem GHCR privada → VPS não conseguia fazer pull sem autenticação.
- **Fix:** `deploy.sh` faz `docker login ghcr.io` via SSH antes do pull quando `GHCR_TOKEN` está presente. Secrets `GHCR_USER` e `GHCR_TOKEN` adicionados ao environment `staging`.

### 13. Healthcheck falso positivo — curl localhost:8080 no host VPS

- **Problema:** deploy.sh fazia `curl http://localhost:8080/health` via SSH no host da VPS. Porta 8080 **não é exposta ao host** — só acessível internamente na rede Docker pelo Caddy. Todo deploy retornava "000" e disparava rollback falso.
- **Fix:** trocado para `docker inspect --format='{{.State.Health.Status}}' mecontrola-server-1`, que lê o healthcheck interno do container (wget localhost:8080/health já configurado no compose.yml).

### 14. otelcol crashava por basicauth/loki sem credenciais

- **Problema:** `config.prod.yml` incluía extensão `basicauth/loki` e exporter `otlphttp/loki`. Com `LOKI_API_KEY` vazio (Grafana Cloud não configurado), o processo do otelcol falhava na inicialização → sem container otelcol → migrate tentava conectar em `localhost:4317` → timeout de 4s por deploy.
- **Fix:** removidos `basicauth/loki`, `otlphttp/loki` e pipeline de logs do `config.prod.yml`. Mantidos: metrics → prometheus, traces → debug (sampling). Extensão agora só `health_check`.

### 15. TLS mismatch: server usava TLS para OTEL, otelcol sem TLS

- **Problema:** `OTEL_EXPORTER_OTLP_INSECURE=false` no .env + devkit-go@v0.5.0 bloqueia `insecure=true` quando `ENVIRONMENT=production`. otelcol escuta na 4317 sem TLS. Resultado: TLS handshake falha silenciosamente em todo export → sem métricas, sem traces.
- **Fix:** `ENVIRONMENT: staging` (este VPS é staging) + `OTEL_EXPORTER_OTLP_INSECURE: "true"` injetados via `compose.prod.yml` environment (override do .env). devkit-go só bloqueia insecure em `production`/`prod`.

### 16. Rollback re-deployava a mesma imagem com falha

- **Problema:** bloco de rollback usava `IMAGE_TAG=${IMAGE_TAG}` — o mesmo SHA recém-falho — em vez da imagem anterior.
- **Fix:** `deploy.sh` captura `PREVIOUS_TAG` via `docker inspect` antes do deploy. Rollback usa `IMAGE_TAG=${PREVIOUS_TAG}` quando diferente do atual.

### 17. CD dispatch manual com imagem inexistente
- **Problema:** dispatch manual do CD com SHA antes do CI completar → imagem não existe no GHCR → pull falha.
- **Causa raiz:** CD dispatch deve usar SHA de uma imagem já publicada pelo CI (`build-image` job). O fluxo correto é automático via `workflow_run`.

---

## Alias útil na VPS

```bash
alias mc='docker compose --env-file /opt/mecontrola/.env -f /opt/mecontrola/deployment/compose/compose.yml -f /opt/mecontrola/deployment/compose/compose.prod.yml'
```

Uso:
```bash
mc ps
mc logs -f server worker
export IMAGE_TAG=local && mc up -d --no-deps --force-recreate server worker
mc restart pgbouncer
```

> **Atenção:** `IMAGE_TAG=local mc ...` não propaga o env var para alias bash. Sempre usar `export IMAGE_TAG=xxx` antes, ou passar o docker compose completo sem alias.

---

## Passo 6 — GitHub Secrets ✅ CONCLUÍDO

10 secrets configurados no environment `staging`:

| Secret | Status |
|--------|--------|
| `VPS_HOST` | ✅ |
| `VPS_USER` | ✅ |
| `VPS_DEPLOY_PATH` | ✅ |
| `VPS_SSH_KEY` | ✅ |
| `STAGING_WEBHOOK_URL` | ✅ |
| `STAGING_META_APP_SECRET` | ✅ |
| `STAGING_SMOKE_WA` | ✅ |
| `STAGING_DB_URL` | ✅ |
| `GHCR_USER` | ✅ |
| `GHCR_TOKEN` | ✅ |

CI/CD pipeline: aguardando CI `fix(deploy): autentica no GHCR via SSH antes do docker compose pull` completar. CD dispara automaticamente via `workflow_run` após `build-image`.

```bash
gh run watch --repo LimaTeixeiraTecnologia/mecontrola
```

---

## Passo 7 — Meta webhook ✅ CONCLUÍDO

- **URL:** `https://api.mecontrola.app.br/api/v1/whatsapp/inbound`
- **Verify Token:** `17ea0b0afefe53a17b85bde058363d06`
- **Status:** `Configurar webhooks` marcado como concluído (✅) na Etapa 2 do portal Meta
- **Fix aplicado:** `GET /inbound` adicionado ao router para satisfazer handshake da Meta

---

## Passo 8 — Meta número real (Etapa 2) — PENDENTE (aguardando OTP)

### Estado atual
- Número `+55 11 93621-2870` adicionado no Gerenciador do WhatsApp Business como **"MeControla"**
- Status: **"Não verificado"**
- WhatsApp Business App excluído do celular (número desconectado)
- **Bloqueio:** rate limit de SMS (`You have requested a verification code too many times`) — aguardar ~24h

### Quando o rate limit liberar

1. Voltar em: **developers.facebook.com → MeControla → Casos de uso → Personalizar → Etapa 2 → Registre seu número**
2. Digitar `(11) 93621-2870` → escolher **"Ligação telefônica"** (evita o mesmo rate limit de SMS)
3. Inserir o código recebido

### Após verificar o número

**Pegar no portal Meta (Gerenciador do WhatsApp → Números de telefone → ⚙️):**
- Novo `Phone Number ID`
- Gerar Access Token permanente: **Meta Business Suite → Configurações → Usuários do sistema → Gerar token** com permissão `whatsapp_business_messaging`

**Atualizar na VPS:**
```bash
ssh root@187.77.45.48

sed -i 's|^META_PHONE_NUMBER_ID=.*|META_PHONE_NUMBER_ID=<novo-id>|' /opt/mecontrola/.env
sed -i 's|^META_ACCESS_TOKEN=.*|META_ACCESS_TOKEN=<token-permanente>|' /opt/mecontrola/.env

# Reiniciar server e worker
export IMAGE_TAG=local
docker compose --env-file /opt/mecontrola/.env \
  -f /opt/mecontrola/deployment/compose/compose.yml \
  -f /opt/mecontrola/deployment/compose/compose.prod.yml \
  up -d --no-deps server worker

# Testar
curl -sf https://api.mecontrola.app.br/health
```

**Testar end-to-end:** enviar mensagem do WhatsApp pessoal para `+55 11 93621-2870` → bot deve responder.

---

## Passo 9 — pgBackRest S3 (backup offsite) — PENDENTE

Configurar após VPS estável e CI/CD funcionando. Requer bucket S3 (AWS ou Cloudflare R2).

Variáveis a preencher no `.env`:

```
PGBACKREST_S3_ENDPOINT=
PGBACKREST_S3_BUCKET=
PGBACKREST_S3_REGION=
PGBACKREST_S3_KEY=
PGBACKREST_S3_KEY_SECRET=
```

Depois executar:

```bash
cd /opt/mecontrola
chmod +x deployment/scripts/pgbackrest-setup.sh
sudo ./deployment/scripts/pgbackrest-setup.sh
```

---

## Verificação rápida do estado

```bash
# Status de todos os containers
docker compose --env-file /opt/mecontrola/.env \
  -f /opt/mecontrola/deployment/compose/compose.yml \
  -f /opt/mecontrola/deployment/compose/compose.prod.yml \
  ps

# Health da API
curl -sf https://api.mecontrola.app.br/health | python3 -m json.tool

# Logs
docker compose --env-file /opt/mecontrola/.env \
  -f /opt/mecontrola/deployment/compose/compose.yml \
  -f /opt/mecontrola/deployment/compose/compose.prod.yml \
  logs -f server worker

# Pipeline CI/CD
gh run list --repo LimaTeixeiraTecnologia/mecontrola --limit 5
```

---

## Credenciais importantes (NÃO commitar)

- **VPS SSH:** `ssh root@187.77.45.48` com `~/.ssh/id_ed25519`
- **META_VERIFY_TOKEN:** `17ea0b0afefe53a17b85bde058363d06`
- **KIWIFY_WEBHOOK_SECRET:** `47cyjfb3gag`
- **META_APP_SECRET:** `fd8f6781034975836f51ea505b3b0a13`
- **DB_PASSWORD e demais secrets:** em `/opt/mecontrola/.env` na VPS
- **Phone Number ID e Access Token permanente:** obter após verificação do número (Passo 8)
