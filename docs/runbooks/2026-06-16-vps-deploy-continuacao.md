# VPS Deploy — Continuação

**Data:** 2026-06-16
**VPS:** 187.77.45.48 (Ubuntu 24.04, KVM 2, 8GB RAM)
**Domínio:** api.mecontrola.app.br → 187.77.45.48 (DNS propagado)
**Repo:** /opt/mecontrola (clonado, .env configurado)

## Estado atual (2026-06-16 18:43 UTC-3)

- [x] Docker instalado (v29.5.3)
- [x] Hardening executado (fail2ban, UFW, SSH key-only, swap 2GB)
- [x] Repo clonado em /opt/mecontrola
- [x] .env de produção configurado (todos os secrets)
- [x] DNS api.mecontrola.app.br → 187.77.45.48
- [x] Kiwify webhook registrado (URL + token)
- [x] Meta App criado (MeControla, modo desenvolvimento)
- [x] Imagem Docker construída na VPS
- [x] Migrations aplicadas
- [x] postgres rodando (healthy)
- [x] pgbouncer rodando (healthy)
- [x] server rodando (healthy)
- [x] worker rodando (healthy)
- [x] caddy rodando (TLS Let's Encrypt obtido, HTTPS funcionando)
- [x] `curl https://api.mecontrola.app.br/health` → `{"status":"healthy"}`
- [x] Caddy healthcheck corrigido no código (`--spider`, 30s interval/start_period) — aplica no próximo deploy
- [ ] GitHub Secrets configurados (CI/CD) — ver Passo 6
- [ ] Meta webhook registrado
- [ ] Meta número real configurado (Etapa 2)
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

### 8. caddy healthcheck unhealthy — CORRIGIDO
- **Causa:** `wget -qO-` gravava body no stdout; flag correta para health check é `--spider` (HEAD request sem output).
- **Fix aplicado** em `deployment/compose/compose.yml`:
  - `["CMD-SHELL", "wget -qO- ..."]` → `["CMD", "wget", "--spider", "-q", "http://localhost:2019/"]`
  - `interval` alterado de 10s para 30s (admin API não precisa de polling agressivo)
  - `start_period` alterado de 10s para 30s (caddy aguarda TLS Let's Encrypt na inicialização)
- **Após próximo deploy:** `docker ps` deve mostrar caddy como "(healthy)".

---

## Alias útil na VPS

```bash
alias mc='docker compose --env-file /opt/mecontrola/.env -f /opt/mecontrola/deployment/compose/compose.yml -f /opt/mecontrola/deployment/compose/compose.prod.yml'
```

Uso:
```bash
mc ps
mc logs -f server worker
IMAGE_TAG=latest mc up -d --no-deps server worker
mc restart pgbouncer
```

---

## Passo 6 — GitHub Secrets (CI/CD automático)

> **Atenção:** `cd.yml` usa `environment: staging`. Os secrets de deploy e smoke **devem ser criados no Environment `staging`**, não em repository secrets genéricos. Secrets de repository não são visíveis para jobs com `environment:`.

### 6.0 — O que foi feito no código

- `deployment/compose/compose.yml`: caddy healthcheck corrigido (`-qO-` → `--spider`, porta 2019, interval 30s)
- `deployment/compose/compose.prod.yml`: pgBackRest vars com `:-` default (sem erro quando não configurado)
- `deployment/scripts/setup-github-secrets.sh`: script que configura os 8 secrets automaticamente
- `deployment/scripts/setup-ghcr-login.sh`: script para autenticar VPS no GHCR (se imagem privada)

### 6.1 — Criar o Environment `staging`

**GitHub → Settings → Environments → New environment**

- Nome: `staging`
- Protection rules: nenhuma (ou configurar "Required reviewers" se quiser aprovação manual)
- Clicar em **Configure environment**

### 6.2 — Executar o script de setup (automatizado)

O script lê a chave SSH local, busca o `DB_PASSWORD` da VPS via SSH, pede o `STAGING_SMOKE_WA` e cria todos os 8 secrets no environment `staging`:

```bash
# Da máquina local (com ssh access à VPS e gh CLI autenticado)
chmod +x deployment/scripts/setup-github-secrets.sh
./deployment/scripts/setup-github-secrets.sh
```

Variáveis de ambiente opcionais (sobrescrevem os defaults):
```bash
VPS_SSH_KEY_PATH=~/.ssh/id_ed25519 \  # default
STAGING_SMOKE_WA=+5511912345678 \      # evita prompt interativo
./deployment/scripts/setup-github-secrets.sh
```

Secrets criados pelo script:

| Secret | Valor |
|--------|-------|
| `VPS_HOST` | `187.77.45.48` |
| `VPS_USER` | `root` |
| `VPS_DEPLOY_PATH` | `/opt/mecontrola` |
| `VPS_SSH_KEY` | conteúdo de `VPS_SSH_KEY_PATH` (chave privada local) |
| `STAGING_WEBHOOK_URL` | `https://api.mecontrola.app.br/api/v1/whatsapp/inbound` |
| `STAGING_META_APP_SECRET` | `fd8f6781034975836f51ea505b3b0a13` |
| `STAGING_SMOKE_WA` | número WhatsApp de teste com `+55` |
| `STAGING_DB_URL` | `postgres://mecontrola:<DB_PASSWORD>@187.77.45.48:5432/mecontrola_db` |

### 6.3 — GHCR login na VPS (só se imagem for privada)

Verificar visibilidade: **GitHub → Packages → mecontrola → Package settings**

Se privada, executar:
```bash
# Requer PAT com escopo read:packages
# Criar em: github.com/settings/tokens/new → read:packages
GHCR_USER=JailtonJunior94 \
GHCR_PAT=<token> \
./deployment/scripts/setup-ghcr-login.sh
```

Se pública, nenhuma ação necessária.

### 6.4 — Disparar CI/CD (primeiro deploy automático)

```bash
# Empurrar commit para disparar o pipeline completo:
# CI: lint → unit → integration → security → governance → card-audit → build-image
# CD: dispara automaticamente via workflow_run após build-image

git commit --allow-empty -m "ci: trigger initial CI/CD pipeline"
git push

# Acompanhar:
gh run watch --repo LimaTeixeiraTecnologia/mecontrola
```

Ou disparar apenas o CD manualmente (se a imagem já existe no GHCR):
```bash
gh workflow run cd.yml \
  --repo LimaTeixeiraTecnologia/mecontrola \
  --field image_tag=<SHA-curto>
```

---

## Passo 7 — Registrar webhook na Meta

**developers.facebook.com → MeControla → Casos de uso → Personalizar → Etapa 1 → Configurar webhooks**

- URL: `https://api.mecontrola.app.br/api/v1/whatsapp/inbound`
- Verify Token: `17ea0b0afefe53a17b85bde058363d06`
- Campo: `messages`

---

## Passo 8 — Meta número real (Etapa 2)

**Personalizar → Etapa 2. Configuração da produção**

- Adicionar número real: `+55 11 9 3621-2870`
- Obter novo Phone Number ID e Access Token permanente (System User)
- Atualizar na VPS:

```bash
sed -i 's|^META_PHONE_NUMBER_ID=.*|META_PHONE_NUMBER_ID=<novo-id>|' /opt/mecontrola/.env
sed -i 's|^META_ACCESS_TOKEN=.*|META_ACCESS_TOKEN=<token-permanente>|' /opt/mecontrola/.env
IMAGE_TAG=latest docker compose --env-file /opt/mecontrola/.env \
  -f /opt/mecontrola/deployment/compose/compose.yml \
  -f /opt/mecontrola/deployment/compose/compose.prod.yml \
  up -d --no-deps server worker
```

---

## Passo 9 — pgBackRest S3 (backup offsite)

Configurar após VPS estável. Requer bucket S3 (AWS ou Cloudflare R2).

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
curl -f https://api.mecontrola.app.br/health

# Logs
docker compose --env-file /opt/mecontrola/.env \
  -f /opt/mecontrola/deployment/compose/compose.yml \
  -f /opt/mecontrola/deployment/compose/compose.prod.yml \
  logs -f server worker
```

---

## Credenciais importantes (NÃO commitar)

- **VPS SSH:** `ssh root@187.77.45.48` com `~/.ssh/id_ed25519`
- **META_VERIFY_TOKEN:** `17ea0b0afefe53a17b85bde058363d06`
- **KIWIFY_WEBHOOK_SECRET:** `47cyjfb3gag`
- **META_APP_SECRET:** `fd8f6781034975836f51ea505b3b0a13`
- **DB_PASSWORD e demais secrets:** em `/opt/mecontrola/.env` na VPS
