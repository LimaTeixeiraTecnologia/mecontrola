# Runbook: Deploy em Produção — MeControla

> **Status:** vivo — reflete a stack atual (Docker Compose + GHCR + VPS Hostinger).
> **Público:** engenheiros responsáveis por subir, atualizar ou recuperar a produção.
> **Onde ler este runbook:** qualquer lugar; **onde executar os comandos** está indicado em cada bloco (`Local`, `VPS`, `GitHub Actions`).

---

## 1. Sumário

1. [Visão geral do fluxo](#2-visão-geral-do-fluxo)
2. [Pré-requisitos](#3-pré-requisitos)
3. [Preparação inicial da VPS](#4-preparação-inicial-da-vps)
4. [Build da imagem Docker](#5-build-da-imagem-docker)
5. [Deploy completo na VPS](#6-deploy-completo-na-vps)
6. [Aplicação robusta de migrations](#7-aplicação-robusta-de-migrations)
7. [Rollout](#8-rollout)
8. [Rollback](#9-rollback)
9. [Verificações pós-deploy](#10-verificações-pós-deploy)
10. [Troubleshooting](#11-troubleshooting)
11. [Comandos úteis](#12-comandos-úteis)
12. [Checklist rápido](#13-checklist-rápido)

---

## 2. Visão geral do fluxo

```text
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│  Local / CI     │────▶│  GHCR           │────▶│  VPS            │
│  docker build   │     │  (registry)     │     │  docker pull    │
└─────────────────┘     └─────────────────┘     └────────┬────────┘
                                                         │
                              ┌──────────────────────────┘
                              ▼
                    ┌─────────────────┐
                    │ 1. git pull     │
                    │ 2. docker pull  │
                    │ 3. migrate up   │  ← executa ANTES do app
                    │ 4. up server    │
                    │ 5. up worker    │
                    │ 6. healthcheck  │
                    │ 7. rollback se  │     health falhar
                    └─────────────────┘
```

**Regras de ouro:**

- A imagem de produção **sempre** usa uma tag imutável (SHA curto do commit). Nunca use `latest` em produção.
- As **migrations são aplicadas antes** de `server` e `worker` subirem.
- O rollback automático só funciona se houver uma imagem anterior rodando no servidor.
- Todo comando que altera estado na VPS deve ser executado com um usuário que pertença ao grupo `docker` (ou root, se for setup inicial).

---

## 3. Pré-requisitos

### 3.1 Na máquina local (macOS)

O runbook assume que você executa os comandos de build/deploy a partir de um Mac com Docker Desktop ou Docker Engine instalado.

| Ferramenta | Versão / uso |
|------------|--------------|
| `git`      | qualquer     |
| `docker`   | 24+          |
| `task`     | 3.51.1       |
| `ssh`      | OpenSSH      |
| Acesso ao repositório GitHub | push para `main` dispara CI/CD |

Verifique localmente:

```bash
# Local
docker --version
task --version
ssh -V
```

### 3.2 Na VPS

| Ferramenta | Versão / uso |
|------------|--------------|
| Ubuntu 22.04+ / Debian 12 | SO recomendado |
| Docker Engine + Compose v2 | orquestração dos containers |
| `git`      | atualização do repo de deploy |
| Acesso SSH com chave | sem senha, preferencialmente `ed25519` |

Verifique na VPS:

```bash
# VPS
docker --version
docker compose version
git --version
```

### 3.3 No GitHub

- Repositório configurado com os secrets do environment `staging` (ou outro environment de produção):
  - `VPS_HOST`
  - `VPS_USER`
  - `VPS_DEPLOY_PATH`
  - `VPS_SSH_KEY`
  - `GHCR_USER`
  - `GHCR_TOKEN`
  - `STAGING_HEALTH_URL` (opcional, mas recomendado)
  - `TELEGRAM_BOT_TOKEN` + `TELEGRAM_CHAT_ID` (opcional, notificação)

> Para configurar os secrets pela primeira vez, use `deployment/scripts/setup-github-secrets.sh`.

---

## 4. Preparação inicial da VPS

> **Quando executar:** apenas na primeira vez que a VPS for provisionada ou quando for refazer do zero.

### 4.1 Acesso inicial

```bash
# Local
ssh root@187.77.45.48
```

> Se você usa uma chave SSH específica (não a padrão do `ssh-agent`), adicione `-i /caminho/da/chave`.
>
> ```bash
> ssh -i ~/.ssh/<sua_chave> root@187.77.45.48
> ```

### 4.2 Hardening básico

O script `deployment/scripts/vps-hardening.sh` instala/configura:

- `fail2ban` (SSH + Caddy)
- `unattended-upgrades`
- hardening do SSH (senha desabilitada, root apenas com chave)
- swapfile 2 GB
- UFW com regras padrão

```bash
# VPS (como root ou sudo)
cd /opt
sudo mkdir -p mecontrola
sudo chown "$(id -un):$(id -gn)" /opt/mecontrola
git clone https://github.com/LimaTeixeiraTecnologia/mecontrola.git /opt/mecontrola
cd /opt/mecontrola
sudo bash deployment/scripts/vps-hardening.sh
```

**Atenção:** o script de SSH hardening exige que você tenha uma chave pública em `~/.ssh/authorized_keys` antes de rodá-lo, senão você pode se trancar para fora.

### 4.3 Firewall

Se o hardening já foi rodado, o firewall já está configurado. Para reaplicar apenas as regras:

```bash
# VPS (como root)
bash deployment/scripts/vps-firewall.sh --force-enable
```

Regras resultantes:

- `deny incoming` default
- `allow outgoing` default
- `22/tcp`  — SSH
- `80/tcp`  — HTTP (Caddy + ACME)
- `443/tcp` — HTTPS

### 4.4 Clone do repositório de deploy

```bash
# VPS
export VPS_DEPLOY_PATH="/opt/mecontrola"
mkdir -p "$(dirname "$VPS_DEPLOY_PATH")"
git clone https://github.com/LimaTeixeiraTecnologia/mecontrola.git "$VPS_DEPLOY_PATH"
cd "$VPS_DEPLOY_PATH"
```

### 4.5 Configurar o `.env` de produção

```bash
# VPS
cd /opt/mecontrola
cp .env.example .env
chmod 600 .env
nano .env                 # ou vim
```

Campos **obrigatórios** em `ENVIRONMENT=production` (a validação do app rejeita `CHANGE_ME_*`):

```env
ENVIRONMENT=production
APP_MODE=server
PORT=8080
SERVICE_NAME_API=mecontrola-api
SERVICE_NAME_WORKER=mecontrola-worker
CORS_ALLOWED_ORIGINS=https://www.mecontrola.app.br,https://mecontrola.app.br

DB_HOST=postgres          # nome do serviço no compose; migrations usam postgres direto
DB_PORT=5432
DB_USER=mecontrola
DB_PASSWORD=<SENHA_FORTE_MIN_16_CHARS>
DB_NAME=mecontrola_db
DB_SSL_MODE=disable

APP_DOMAIN=mecontrola.app.br
CADDY_EMAIL=alerts@mecontrola.app.br

OTEL_LGTM_ADMIN_PASSWORD=<SENHA_FORTE_GRAFANA>

IDENTITY_GATEWAY_SHARED_SECRET_CURRENT=<hex_32_bytes>
OPENROUTER_API_KEY=<chave_openrouter>
META_ACCESS_TOKEN=<token_meta>
META_PHONE_NUMBER_ID=<id>
META_APP_SECRET=<segredo>
META_VERIFY_TOKEN=<verify_token>
KIWIFY_CLIENT_ID=...
KIWIFY_CLIENT_SECRET=...
KIWIFY_ACCOUNT_ID=...
KIWIFY_WEBHOOK_SECRET=...
KIWIFY_PRODUCT_ID_MONTHLY=...
KIWIFY_PRODUCT_ID_QUARTERLY=...
KIWIFY_PRODUCT_ID_ANNUAL=...
```

> Dica: gere o gateway secret com `openssl rand -hex 32`.

### 4.6 Autenticar a VPS no GHCR (se a imagem for privada)

```bash
# Local
bash deployment/scripts/setup-ghcr-login.sh
```

Se preferir fazer manualmente:

```bash
# VPS
echo '<GHCR_PAT_COM_READ_PACKAGES>' | docker login ghcr.io -u '<GITHUB_USER>' --password-stdin
```

---

## 5. Build da imagem Docker

### 5.1 Opção A — build local (testes / debugging)

```bash
# Local (na raiz do repositório)
task docker:build IMAGE_TAG=<TAG>
```

Exemplo:

```bash
# Local
export IMAGE_TAG="$(git rev-parse --short HEAD)"
task docker:build IMAGE_TAG="$IMAGE_TAG"
```

A imagem será gerada como `mecontrola:<TAG>`.

Validações pós-build:

```bash
# Local
docker inspect --format='{{.Config.User}}' "mecontrola:$IMAGE_TAG"   # deve ser nonroot
docker inspect --format='{{.Size}}' "mecontrola:$IMAGE_TAG"          # deve ser ≤ 30 MB
```

### 5.2 Opção B — build e push manual para GHCR

```bash
# Local
export IMAGE_TAG="$(git rev-parse --short HEAD)"
export IMAGE_NAME="ghcr.io/limateixeiratecnologia/mecontrola"

docker build \
  --file deployment/docker/Dockerfile \
  --tag "${IMAGE_NAME}:${IMAGE_TAG}" \
  --build-arg VERSION="${IMAGE_TAG}" \
  .

docker push "${IMAGE_NAME}:${IMAGE_TAG}"
```

> Em produção, prefira o build da CI (Opção C).

### 5.3 Opção C — build e push via CI/CD (recomendado)

```bash
# Local
git add .
git commit -m "feat: ..."
git push origin main
```

O workflow `.github/workflows/ci-cd.yml` executa:

1. `go build`
2. lint
3. testes unitários e de integração
4. `govulncheck`
5. build e push da imagem para `ghcr.io/limateixeiratecnologia/mecontrola:<SHORT_SHA>`
6. scan Trivy (CRITICAL/HIGH)
7. assinatura cosign keyless
8. deploy na VPS
9. healthcheck externo
10. notificação Telegram

Acompanhe em:

```bash
# Local
gh run list --workflow=ci-cd.yml --repo LimaTeixeiraTecnologia/mecontrola
```

---

## 6. Deploy completo na VPS

### 6.1 Opção A — script `deploy.sh` (recomendado)

O script faz:

1. `git pull` na VPS
2. Login no GHCR (se `GHCR_TOKEN` fornecido)
3. Captura a imagem anterior para rollback
4. `docker compose pull` da nova imagem
5. Garante que `otel-lgtm` esteja saudável
6. Executa as migrations
7. Sobe `server` e `worker`
8. Aguarda healthcheck do `server`
9. Em caso de falha, reverte para a imagem anterior

#### 6.1.1 Executando da sua máquina local

```bash
# Local
export VPS_HOST="187.77.45.48"
export VPS_USER="root"
export VPS_DEPLOY_PATH="/opt/mecontrola"
export GHCR_USER="<GITHUB_USER>"
export GHCR_TOKEN="<GHCR_PAT>"
export IMAGE_TAG="<SHORT_SHA_OU_TAG>"

# Se sua chave SSH não for a padrão, descomente:
# export VPS_SSH_KEY="$HOME/.ssh/<sua_chave>"

bash deployment/scripts/deploy.sh "$IMAGE_TAG"
```

#### 6.1.2 Executando diretamente na VPS

```bash
# VPS
cd /opt/mecontrola
export IMAGE_TAG="<SHORT_SHA_OU_TAG>"
export GHCR_USER="<GITHUB_USER>"
export GHCR_TOKEN="<GHCR_PAT>"
export LOCAL_DEPLOY="true"

bash deployment/scripts/deploy.sh "$IMAGE_TAG"
```

### 6.2 Opção B — passo a passo manual (para entender ou debugar)

```bash
# VPS
cd /opt/mecontrola
export IMAGE_TAG="<SHORT_SHA_OU_TAG>"

# 1. Atualizar código
git pull --ff-only

# 2. Login no GHCR (se imagem privada)
echo '<GHCR_PAT>' | docker login ghcr.io -u '<GITHUB_USER>' --password-stdin

# 3. Pull da nova imagem
IMAGE_TAG="$IMAGE_TAG" docker compose \
  --env-file .env \
  -f deployment/compose/compose.yml \
  -f deployment/compose/compose.prod.yml \
  pull server worker

# 4. Garantir observabilidade
IMAGE_TAG="$IMAGE_TAG" docker compose \
  --env-file .env \
  -f deployment/compose/compose.yml \
  -f deployment/compose/compose.prod.yml \
  up -d --remove-orphans otel-lgtm

# 5. Aplicar migrations
IMAGE_TAG="$IMAGE_TAG" docker compose \
  --env-file .env \
  -f deployment/compose/compose.yml \
  -f deployment/compose/compose.prod.yml \
  run --rm --no-deps migrate

# 6. Subir server e worker
IMAGE_TAG="$IMAGE_TAG" docker compose \
  --env-file .env \
  -f deployment/compose/compose.yml \
  -f deployment/compose/compose.prod.yml \
  up -d --no-deps server worker

# 7. Aguardar healthcheck
for i in $(seq 1 24); do
  STATUS=$(docker inspect --format='{{.State.Health.Status}}' mecontrola-server-1 2>/dev/null || echo 'unknown')
  echo "[$i/24] server health: $STATUS"
  [[ "$STATUS" == "healthy" ]] && break
  sleep 5
done
```

---

### 6.3 Atualizando variáveis de ambiente (novas `.env`)

> O `deploy.sh` atualiza o código-fonte (`git pull`), mas **nunca** altera o `.env` de produção. Variáveis novas devem ser adicionadas manualmente na VPS antes do deploy.

#### Por que não automatizar?

- `.env` não está versionado (contém segredos).
- Uma variável nova com valor errado pode derrubar a produção.
- O `configs/config.go` rejeita placeholders como `CHANGE_ME_*` em `ENVIRONMENT=production`.

#### Processo recomendado

1. **Sempre que uma PR introduzir uma nova variável**, o autor deve também atualizar `.env.example` no repositório.

2. Antes do deploy, compare o `.env.example` atualizado com o `.env` de produção na VPS:

```bash
# Local
ssh root@187.77.45.48 "cat /opt/mecontrola/.env" > /tmp/vps-env
```

3. Identifique chaves que existem em `.env.example` mas não no `.env` da VPS:

```bash
# Local (rode na raiz do repo)
cp .env.example /tmp/example-env

# Lista chaves que faltam na VPS
comm -23 \
  <(grep -E '^[A-Z_]+=' /tmp/example-env | cut -d= -f1 | sort) \
  <(grep -E '^[A-Z_]+=' /tmp/vps-env | cut -d= -f1 | sort)
```

4. Na VPS, edite o `.env` e adicione as novas variáveis com valores reais:

```bash
# VPS
nano /opt/mecontrola/.env
# ou vim /opt/mecontrola/.env
```

5. Valide o `.env` antes de rodar o deploy:

```bash
# VPS
cd /opt/mecontrola

# a) Garantir que não há placeholders em produção
if grep -E 'CHANGE_ME_' .env; then
  echo "ERRO: existem placeholders no .env de produção."
  exit 1
fi

# b) Validar que o Docker Compose consegue resolver todas as variáveis
export IMAGE_TAG="<TAG_DA_NOVA_IMAGEM>"
IMAGE_TAG="$IMAGE_TAG" docker compose \
  --env-file .env \
  -f deployment/compose/compose.yml \
  -f deployment/compose/compose.prod.yml \
  config > /dev/null
```

> O `docker compose config` valida a sintaxe e a presença das variáveis obrigatórias do Compose. A validação final das configs da aplicação acontece no boot do container `server` (healthcheck).

6. Agora sim, execute o deploy normalmente conforme [seção 6.1](#61-opção-a--script-deploysh-recomendado).

#### Checklist de novas `.env`

- [ ] `.env.example` foi atualizado no repositório.
- [ ] Identifiquei todas as variáveis novas obrigatórias.
- [ ] Adicionei os valores reais em `/opt/mecontrola/.env`.
- [ ] Não deixei nenhum `CHANGE_ME_*` em produção.
- [ ] Validei o `.env` subindo um container de teste.
- [ ] Execute o `deploy.sh`.

---

## 7. Aplicação robusta de migrations

### 7.1 Como funciona

- As migrations estão em `migrations/*.up.sql` e são embedadas no binário (`migrations.FS`).
- O comando `mecontrola migrate` usa `golang-migrate` com driver `pgx/v5`.
- Em produção, o container `migrate` do `compose.prod.yml` conecta **diretamente no PostgreSQL** (`DB_HOST=postgres`, `DB_PORT=5432`), **não no pgBouncer**.
- A tabela de controle é `public.schema_migrations`.

### 7.2 Comando automático (via deploy)

Já está incluído no `deploy.sh`:

```bash
IMAGE_TAG=<TAG> docker compose \
  --env-file .env \
  -f deployment/compose/compose.yml \
  -f deployment/compose/compose.prod.yml \
  run --rm --no-deps migrate
```

### 7.3 Verificar status das migrations

O binário não expõe comando de `status` nativo do `golang-migrate`. Verifique diretamente na tabela de controle via `psql`:

```bash
# VPS
docker exec -it mecontrola-postgres-1 psql -U mecontrola -d mecontrola_db -c \
  "SELECT version, dirty FROM public.schema_migrations ORDER BY version DESC LIMIT 10;"
```

Saída esperada:

```text
 version | dirty
---------+-------
 000008  | f
 000007  | f
 000006  | f
```

- `dirty = f` → ok.
- `dirty = t` → a migration falhou no meio. **Não prossiga com deploy.** Resolva manualmente ou restaure do backup.

### 7.4 Rollback de migrations (emergência)

> **Atenção:** rollback destrutivo. Só execute se souber exatamente o que está revertendo.

Reverter **uma** migration (sobrescreve o comando padrão do serviço `migrate`):

```bash
# VPS
IMAGE_TAG=<TAG> docker compose \
  --env-file .env \
  -f deployment/compose/compose.yml \
  -f deployment/compose/compose.prod.yml \
  run --rm --no-deps migrate migrate-down --steps 1
```

Reverter **todas**:

```bash
# VPS
IMAGE_TAG=<TAG> docker compose \
  --env-file .env \
  -f deployment/compose/compose.yml \
  -f deployment/compose/compose.prod.yml \
  run --rm --no-deps migrate migrate-down --steps -1
```

---

## 8. Rollout

### 8.1 Estratégia atual

O projeto usa **rolling recreate** via Docker Compose:

1. A nova imagem é baixada (`pull`).
2. `server` e `worker` são recriados (`up -d`).
3. O healthcheck do `server` decide se o rollout foi bem-sucedido.
4. Se falhar, `deploy.sh` reverte `server`/`worker` para a imagem anterior.

Não há blue/green nativo, mas o downtime é limitado ao tempo de boot do container (geralmente < 30 s).

### 8.2 Rollout manual controlado

Se quiser fazer rollout sem rodar migrations (útil para hotfixes que não tocam no banco):

```bash
# VPS
cd /opt/mecontrola
export IMAGE_TAG="<NOVA_TAG>"

# Apenas pull + recriar server/worker (pula migrate)
IMAGE_TAG="$IMAGE_TAG" docker compose \
  --env-file .env \
  -f deployment/compose/compose.yml \
  -f deployment/compose/compose.prod.yml \
  up -d --no-deps --pull always server worker
```

### 8.3 Rollout com validação gradual

Após subir, valide endpoints internos antes de considerar pronto:

```bash
# VPS
curl -fsS http://localhost:8080/health
curl -fsS http://localhost:8080/ready
```

---

## 9. Rollback

### 9.1 Rollback automático

O `deploy.sh` já faz rollback se o healthcheck do `server` não ficar `healthy` em até ~2 minutos:

```text
[...] iniciando rollback
Revertendo para imagem anterior: <TAG_ANTERIOR>
```

Se isso acontecer, investigue os logs antes de tentar novamente.

### 9.2 Rollback manual para uma tag específica

```bash
# VPS
cd /opt/mecontrola
export IMAGE_TAG="<TAG_ANTERIOR>"

IMAGE_TAG="$IMAGE_TAG" docker compose \
  --env-file .env \
  -f deployment/compose/compose.yml \
  -f deployment/compose/compose.prod.yml \
  up -d --no-deps --pull never server worker
```

### 9.3 Rollback de migration

Se o deploy incluiu uma migration que precisa ser desfeita, veja [7.4 Rollback de migrations](#74-rollback-de-migrations-emergência).

> Se a migration já foi executada e a nova versão do app depende dela, **não faça rollback do app sem também reverter a migration**, ou o app pode quebrar.

---

## 10. Verificações pós-deploy

### 10.1 Containers

```bash
# VPS
docker compose \
  --env-file .env \
  -f deployment/compose/compose.yml \
  -f deployment/compose/compose.prod.yml \
  ps
```

Esperado:

- `postgres`     → `running (healthy)`
- `pgbouncer`    → `running (healthy)`
- `otel-lgtm`    → `running (healthy)`
- `server`       → `running (healthy)`
- `worker`       → `running (healthy)`
- `caddy`        → `running (healthy)`

### 10.2 Healthchecks da aplicação

```bash
# VPS
curl -fsS http://localhost:8080/health
curl -fsS http://localhost:8080/ready

# Externo (HTTPS)
curl -fsS https://<APP_DOMAIN>/health
curl -fsS https://<APP_DOMAIN>/ready
```

### 10.3 Logs

```bash
# VPS — últimos 100 logs do server
docker logs --tail 100 mecontrola-server-1

# VPS — acompanhar logs em tempo real
docker compose \
  --env-file .env \
  -f deployment/compose/compose.yml \
  -f deployment/compose/compose.prod.yml \
  logs -f server worker
```

### 10.4 Métricas e observabilidade

Acesse o Grafana via túnel SSH (a porta 3000 só escuta em `127.0.0.1`):

```bash
# Local
ssh -N -L 3000:127.0.0.1:3000 root@187.77.45.48
# Depois abra http://localhost:3000
```

---

## 11. Troubleshooting

### 11.1 Healthcheck do server falha

```bash
# VPS
docker logs --tail 200 mecontrola-server-1
```

Causas comuns:

- `.env` inválido (valores `CHANGE_ME_*` em produção).
- Banco não acessível (`pgbouncer` ou `postgres` não saudáveis).
- Migration não aplicada ou `schema_migrations.dirty = true`.

### 11.2 Migration falhou

1. Pare o deploy.
2. Verifique `schema_migrations`:

```bash
# VPS
docker exec mecontrola-postgres-1 psql -U mecontrola -d mecontrola_db -c \
  "SELECT version, dirty FROM public.schema_migrations;"
```

3. Se `dirty = true`, resolva a causa raiz, force o dirty flag para false se a migration foi parcialmente aplicada e considere restaurar do backup.

### 11.3 VPS não consegue fazer pull da imagem

```bash
# VPS
docker pull ghcr.io/limateixeiratecnologia/mecontrola:<TAG>
```

Se falhar com "denied":

```bash
# VPS
docker login ghcr.io -u '<GITHUB_USER>' --password-stdin
# cole o GHCR_PAT
```

### 11.4 Erro de permissão no `.git` durante deploy

O `deploy.sh` tenta autocorrigir ownership. Se falhar:

```bash
# VPS (como root ou usuário correto)
sudo chown -R <usuario_do_runner>:<grupo_do_runner> /opt/mecontrola
```

### 11.5 Worker não inicia

```bash
# VPS
docker logs --tail 200 mecontrola-worker-1
```

Verifique se `postgres`/`pgbouncer` estão saudáveis e se o `.env` possui `ENVIRONMENT=production`.

### 11.6 Caddy não consegue emitir certificado

Verifique DNS:

```bash
# Local
dig +short <APP_DOMAIN>
```

e logs:

```bash
# VPS
docker logs --tail 100 mecontrola-caddy-1
```

---

## 12. Comandos úteis

### Parar tudo (mantendo volumes)

```bash
# VPS
cd /opt/mecontrola
docker compose \
  --env-file .env \
  -f deployment/compose/compose.yml \
  -f deployment/compose/compose.prod.yml \
  down
```

### Parar tudo e remover volumes (⚠️ apaga dados)

```bash
# VPS
cd /opt/mecontrola
docker compose \
  --env-file .env \
  -f deployment/compose/compose.yml \
  -f deployment/compose/compose.prod.yml \
  down -v
```

### Reiniciar apenas postgres/pgbouncer

```bash
# VPS
docker compose \
  --env-file .env \
  -f deployment/compose/compose.yml \
  -f deployment/compose/compose.prod.yml \
  restart postgres pgbouncer
```

### Ver imagens em uso

```bash
# VPS
docker inspect --format='{{index .Config.Image}}' mecontrola-server-1
docker inspect --format='{{index .Config.Image}}' mecontrola-worker-1
```

### Limpar imagens antigas

```bash
# VPS
docker image prune -f --filter 'until=72h'
```

### Acessar o banco via psql

```bash
# VPS
docker exec -it mecontrola-postgres-1 psql -U mecontrola -d mecontrola_db
```

### Executar um comando de manutenção no worker

```bash
# VPS
docker exec mecontrola-worker-1 /app/mecontrola --help
```

---

## 13. Checklist rápido

Use este checklist antes de considerar um deploy concluído:

- [ ] `.env` de produção preenchido e `chmod 600`.
- [ ] `IMAGE_TAG` é um SHA curto ou tag semântica imutável (não `latest`).
- [ ] Imagem existe no GHCR: `docker pull ghcr.io/limateixeiratecnologia/mecontrola:<TAG>`.
- [ ] CI/CD passou (build, lint, testes, vulncheck, Trivy, cosign).
- [ ] Backup do banco realizado antes do deploy (se houver migration de risco).
- [ ] `deploy.sh` executado sem erros.
- [ ] `docker compose ps` mostra todos os serviços `healthy`.
- [ ] `curl https://<APP_DOMAIN>/health` retorna 200.
- [ ] `curl https://<APP_DOMAIN>/ready` retorna 200.
- [ ] Logs de `server` e `worker` não mostram panics ou erros de conexão.
- [ ] Grafana acessível via túnel SSH e métricas chegando.

---

## Referências internas

- `deployment/docker/Dockerfile`
- `deployment/compose/compose.yml`
- `deployment/compose/compose.prod.yml`
- `deployment/scripts/deploy.sh`
- `deployment/scripts/vps-hardening.sh`
- `deployment/scripts/vps-firewall.sh`
- `deployment/scripts/setup-ghcr-login.sh`
- `deployment/scripts/setup-github-secrets.sh`
- `.github/workflows/ci-cd.yml`
- `cmd/migrate/migrate.go`
- `configs/config.go`
