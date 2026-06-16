# VPS Deploy — Continuação

**Data:** 2026-06-16
**VPS:** 187.77.45.48 (Ubuntu 24.04, KVM 2, 8GB RAM)
**Domínio:** api.mecontrola.app.br → 187.77.45.48 (DNS propagado)
**Repo:** /opt/mecontrola (clonado, .env configurado)

## Estado atual

- [x] Docker instalado (v29.5.3)
- [x] Hardening executado (fail2ban, UFW, SSH key-only, swap 2GB)
- [x] Repo clonado em /opt/mecontrola
- [x] .env de produção configurado (todos os secrets)
- [x] DNS api.mecontrola.app.br → 187.77.45.48
- [x] Kiwify webhook registrado (URL + token)
- [x] Meta App criado (MeControla, modo desenvolvimento)
- [ ] Build da imagem Docker na VPS
- [ ] Containers rodando
- [ ] GitHub Secrets configurados (CI/CD)
- [ ] Meta webhook registrado (após VPS no ar)
- [ ] Meta número real configurado (Etapa 2)
- [ ] pgBackRest S3 configurado

---

## Passo 1 — Build da imagem na VPS

```bash
ssh root@187.77.45.48
cd /opt/mecontrola
git pull
docker build -f deployment/docker/Dockerfile -t ghcr.io/limateixeiratecnologia/mecontrola:latest .
```

---

## Passo 2 — Subir banco

```bash
IMAGE_TAG=latest docker compose --env-file .env \
  -f deployment/compose/compose.yml \
  -f deployment/compose/compose.prod.yml \
  up -d postgres pgbouncer
```

Aguardar postgres healthy:

```bash
docker compose --env-file .env \
  -f deployment/compose/compose.yml \
  -f deployment/compose/compose.prod.yml \
  ps postgres
```

---

## Passo 3 — Rodar migrations

```bash
IMAGE_TAG=latest docker compose --env-file .env \
  -f deployment/compose/compose.yml \
  -f deployment/compose/compose.prod.yml \
  run --rm migrate
```

---

## Passo 4 — Subir aplicação

```bash
IMAGE_TAG=latest docker compose --env-file .env \
  -f deployment/compose/compose.yml \
  -f deployment/compose/compose.prod.yml \
  up -d server worker caddy
```

---

## Passo 5 — Verificar saúde

```bash
docker compose --env-file .env \
  -f deployment/compose/compose.yml \
  -f deployment/compose/compose.prod.yml \
  ps

curl -f http://localhost:8080/health
curl -f https://api.mecontrola.app.br/health
```

---

## Passo 6 — GitHub Secrets (CI/CD automático)

No repositório: **Settings → Secrets → Actions → New repository secret**

| Secret | Valor |
|--------|-------|
| `VPS_HOST` | `187.77.45.48` |
| `VPS_USER` | `root` |
| `VPS_DEPLOY_PATH` | `/opt/mecontrola` |
| `VPS_SSH_KEY` | conteúdo de `~/.ssh/id_ed25519` (chave privada) |
| `STAGING_WEBHOOK_URL` | `https://api.mecontrola.app.br/api/v1/whatsapp/inbound` |
| `STAGING_META_APP_SECRET` | `fd8f6781034975836f51ea505b3b0a13` |
| `STAGING_SMOKE_WA` | número WhatsApp de teste com +55 |
| `STAGING_DB_URL` | `postgres://mecontrola:<DB_PASSWORD>@187.77.45.48:5432/mecontrola_db` |

DB_PASSWORD está em `/opt/mecontrola/.env` na VPS.

---

## Passo 7 — Registrar webhook na Meta

Após VPS no ar com HTTPS funcionando:

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
IMAGE_TAG=latest docker compose --env-file .env \
  -f deployment/compose/compose.yml \
  -f deployment/compose/compose.prod.yml \
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

## Referências rápidas

```bash
# Alias útil na VPS
alias mc='docker compose --env-file /opt/mecontrola/.env -f /opt/mecontrola/deployment/compose/compose.yml -f /opt/mecontrola/deployment/compose/compose.prod.yml'

# Logs
mc logs -f server worker

# Status
mc ps

# Restart app sem derrubar banco
IMAGE_TAG=latest mc up -d --no-deps server worker

# Restart só banco
mc restart postgres pgbouncer
```

---

## Credenciais importantes (NÃO commitar)

- **VPS SSH:** `ssh root@187.77.45.48` com `~/.ssh/id_ed25519`
- **META_VERIFY_TOKEN:** `17ea0b0afefe53a17b85bde058363d06`
- **KIWIFY_WEBHOOK_SECRET:** `47cyjfb3gag`
- **DB_PASSWORD e demais secrets:** em `/opt/mecontrola/.env` na VPS
