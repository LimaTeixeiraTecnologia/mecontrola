# MeControla

[![CI](https://github.com/LimaTeixeiraTecnologia/mecontrola/actions/workflows/ci.yml/badge.svg)](https://github.com/LimaTeixeiraTecnologia/mecontrola/actions/workflows/ci.yml)
[![CD](https://github.com/LimaTeixeiraTecnologia/mecontrola/actions/workflows/cd.yml/badge.svg)](https://github.com/LimaTeixeiraTecnologia/mecontrola/actions/workflows/cd.yml)
[![Image Signed](https://img.shields.io/badge/image-cosign%20keyless-blue)](https://github.com/LimaTeixeiraTecnologia/mecontrola/actions/workflows/cd.yml)
[![SBOM](https://img.shields.io/badge/SBOM-SPDX--JSON-green)](https://github.com/LimaTeixeiraTecnologia/mecontrola/actions/workflows/cd.yml)
[![Governance](https://img.shields.io/badge/governance-AGENTS.md-orange)](./AGENTS.md)

Monolito modular em Go para fluxos financeiros conversacionais via WhatsApp.

---

## Índice

- [Pré-requisitos](#pré-requisitos)
- [Stack](#stack)
- [Módulos e responsabilidades](#módulos-e-responsabilidades)
- [Entrypoints](#entrypoints)
- [Configuração (.env)](#configuração-env)
- [Gestão de variáveis de ambiente e secrets](#gestão-de-variáveis-de-ambiente-e-secrets)
- [Subir só a infra](#subir-só-a-infra)
- [Subir tudo (infra + migrate + server + worker)](#subir-tudo-infra--migrate--server--worker)
- [Debug no VS Code](#debug-no-vs-code)
- [Comandos Task](#comandos-task)
- [Sequências comuns](#sequências-comuns)
- [Reset do banco de produção](#reset-do-banco-de-produção)
- [CI/CD](#cicd)
- [Docker Swarm](#docker-swarm)
- [Deploy da máquina local direto na VPS (deploy-local.sh)](#deploy-da-máquina-local-direto-na-vps-deploy-localsh)
- [Acesso Remoto](#acesso-remoto)
- [skills-lock.json](#skills-lockjson)
- [Contribuição](#contribuição)
- [Governance](#governance)

---

## Três ambientes canônicos

| Objetivo | Comando | Arquivo base |
|---|---|---|
| Paridade com produção localmente | `task swarm:local:up` | `compose.swarm.yml` (mesmo da VPS) |
| Desenvolvimento completo (Compose) | `task local:up` | `compose.yml` + `compose.local.yml` |
| Só infra + debug VS Code | `task local:infra` → F5 | `compose.yml` + `compose.local.yml` (serviços de infra) |

---

## Pré-requisitos

| Ferramenta | Versão | Observação |
|---|---|---|
| Docker Engine + Compose v2 | Docker 24+ | Obrigatório para infra local e Swarm |
| Go | 1.26+ | Versão declarada em `go.mod` |
| Task | 3.51.1 | Runner de tarefas (`Taskfile.yml`) |
| golangci-lint | v2.12.2 | Linter estático; `task setup` e `task lint:*` provisionam a versão pinada em `.tools/bin` |
| mockery | v2.53.6 | Geração de mocks |
| govulncheck | v1.1.4 | Auditoria de vulnerabilidades |
| trivy | v0.62.1 | Supply chain / SBOM |
| cosign | v2.4.3 | Assinatura keyless de imagem |
| gitsign | v0.12.0 | Assinatura keyless de commits |
| ngrok | qualquer | Opcional — túnel para webhook local |

Após instalar as ferramentas-base obrigatórias, execute:

```bash
task setup
```

`task setup` instala o `golangci-lint` pinado do repositório em `.tools/bin`, então uma versão global incompatível no `PATH` não interfere no fluxo local.

---

## Stack

| Componente | Tecnologia / Versão |
|---|---|
| Linguagem | Go 1.26.4 |
| Banco de dados | PostgreSQL 16 |
| Connection pooler | pgBouncer edoburu/pgbouncer:v1.25.2-p0 (pool mode: transaction) |
| Driver PostgreSQL | pgx/v5 v5.10.0 |
| Migrações | golang-migrate v4.19.1 |
| Roteador HTTP | go-chi/chi v5.3.0 |
| Observabilidade | OpenTelemetry v1.44.0 |
| Observabilidade local | grafana/otel-lgtm:0.7.5 |
| Proxy / TLS | Caddy 2 |
| Orquestração | Docker Swarm single-node |
| Registro de imagem | ghcr.io/limateixeiratecnologia/mecontrola |
| Supply chain | Trivy + cosign keyless + SBOM SPDX-JSON |

---

## Módulos e responsabilidades

| Módulo | Responsabilidade |
|---|---|
| `internal/agents` | Integração LLM via OpenRouter; port do Weather Mastra em Go; padrão Workflow/Tool com `WorkflowRegistry`; runtime Thread/Run auditável; structured output; dispatch via WhatsApp Cloud API |
| `internal/billing` | Webhook Kiwify, reconciliação de assinaturas, grace period `PAST_DUE` (3 dias), housekeeping de eventos de cobrança |
| `internal/budgets` | Orçamentos mensais, despesas por categoria, recorrência, resumo mensal, reaper/purge jobs |
| `internal/card` | CRUD de cartões, listagem paginada, fatura por competência, conformidade PCI RF-16 |
| `internal/categories` | Catálogo de categorias, dicionário com busca HTTP e ETag cache |
| `internal/identity` | Usuários, principal/auth, entitlements, gateway HMAC-SHA256, housekeeping de `auth_events` |
| `internal/onboarding` | Magic token, ativação via WhatsApp, outreach, expiração de tokens, limpeza de mensagens Meta |
| `internal/transactions` | Transações financeiras (DMMF / `Decide*`), idempotência, resumo mensal, recorrência materializada |
| `internal/platform` | Plataforma genérica reutilizável com subcamadas: **workflow kernel** (`Engine[S]`, `Step[S]`, suspend/resume via merge-patch RFC 7386); **agent** (Thread/Run auditável, WorkingMemory, PendingStep); **memory** (threads, mensagens, working memory, embeddings via pgvector); **llm** (OpenRouter provider); **scorer/evals**; outbox transacional; worker manager; WhatsApp Cloud API; idempotência; rate limit |
| `internal/bootstrap` | Inicialização da aplicação, wiring de todos os módulos |

---

## Entrypoints

O binário `mecontrola` expõe quatro subcomandos via Cobra:

```bash
mecontrola server          # HTTP server (Chi, porta configurada em PORT)
mecontrola worker          # Worker de background (outbox dispatcher, jobs agendados)
mecontrola migrate         # Aplica todas as migrations pendentes e sai
mecontrola migrate-down    # Reverte migrations (default: 1 step; use --steps -1 para reset total)
```

Migrações disponíveis:

| Versão | Descrição |
|---|---|
| `000001_initial_schema` | Baseline único do schema final, incluindo seeds de referência, jornada de ativação, ledger de escrita do agente, origem de transações e tabelas `platform_*` com `pgvector` |

---

## Configuração (.env)

Copie `.env.example` para `.env` e preencha os valores marcados com `CHANGE_ME_*`. Em `ENVIRONMENT=production`, qualquer variável com prefixo `CHANGE_ME_*` causa falha no `Config.Validate()` na inicialização.

```bash
cp .env.example .env
```

### Aplicação

```env
ENVIRONMENT=local   # local | production
APP_MODE=server
```

### HTTP

```env
PORT=8080
WORKER_HEALTH_ADDR=:8081
SERVICE_NAME_API=mecontrola-api
SERVICE_NAME_WORKER=mecontrola-worker
CORS_ALLOWED_ORIGINS=http://localhost:3000,http://localhost:4321,http://localhost:5173
AUTH_RATE_LIMIT_PER_USER_PER_MIN=120
AUTH_RATE_LIMIT_PER_USER_BURST=60
```

> **Produção:** `CORS_ALLOWED_ORIGINS` exige lista explícita de origens separadas por vírgula. Wildcard `*` ou valor vazio causam erro de boot.

### Banco de Dados

```env
DB_HOST=localhost
DB_PORT=5432
DB_USER=mecontrola
DB_PASSWORD=CHANGE_ME_USE_STRONG_PASSWORD
DB_NAME=mecontrola_db
DB_SSL_MODE=disable
DB_MAX_CONNS=10
DB_MIN_CONNS=2
DB_MAX_IDLE_CONNS=5
DB_CONN_MAX_LIFETIME=30m
DB_CONN_MAX_IDLE_TIME=5m

# Para testes de integração
DATABASE_URL=postgres://mecontrola:CHANGE_ME_USE_STRONG_PASSWORD@localhost:5432/mecontrola_db?sslmode=disable
```

### Observabilidade (OpenTelemetry)

```env
OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4317
OTEL_EXPORTER_OTLP_PROTOCOL=grpc
OTEL_EXPORTER_OTLP_INSECURE=true
OTEL_TRACE_SAMPLE_RATE=1.0
OTEL_SERVICE_VERSION=dev
LOG_LEVEL=debug
LOG_FORMAT=json
```

Em Docker Compose local, `localhost:4317` e `localhost:4318` sao validos porque o `otel-lgtm`
publica essas portas no host. Em producao com Swarm, use `OTEL_EXPORTER_OTLP_ENDPOINT=otel-lgtm:4317`
para workloads containerizados na rede `mecontrola_backend`.

### Docker Compose local (otel-lgtm)

```env
OTEL_LGTM_ADMIN_USER=admin
OTEL_LGTM_ADMIN_PASSWORD=CHANGE_ME_use_strong_password
```

### Grafana standalone (profile `observability`)

```env
GRAFANA_ADMIN_USER=admin
GRAFANA_ADMIN_PASSWORD=CHANGE_ME_use_strong_password
```

> **Produção:** altere `GRAFANA_ADMIN_PASSWORD` antes do primeiro boot.

### Deploy / Infraestrutura

```env
APP_DOMAIN=CHANGE_ME_yourdomain.com
CADDY_EMAIL=CHANGE_ME_your@email.com
IMAGE_NAME=ghcr.io/limateixeiratecnologia/mecontrola
IMAGE_TAG=latest
POSTGRES_IMAGE=postgres:16-alpine
BACKUP_REMOTE=CHANGE_ME_backup:mecontrola-backups
AGE_RECIPIENT=CHANGE_ME_age1...
RETENTION_DAYS=30
PGBACKREST_S3_BUCKET=CHANGE_ME_mecontrola-backups-123456789012-use1
PGBACKREST_REPO1_CIPHER_PASS=CHANGE_ME_gerar_senha_forte_32_plus_caracteres
ALERT_TELEGRAM_BOT_TOKEN=
ALERT_TELEGRAM_CHAT_ID=
```

### Outbox Transacional (RF-26 / D-03)

```env
OUTBOX_DISPATCHER_ENABLED=true
OUTBOX_DISPATCHER_TICK_INTERVAL=500ms
OUTBOX_DISPATCHER_BATCH_SIZE=50
OUTBOX_DISPATCHER_HANDLER_TIMEOUT=10s
OUTBOX_RETRY_MAX_ATTEMPTS=3
OUTBOX_RETRY_BASE_BACKOFF=2s
OUTBOX_RETRY_MAX_BACKOFF=5m
OUTBOX_HOUSEKEEPING_RETENTION_DAYS=90
OUTBOX_HOUSEKEEPING_SCHEDULE=@daily
OUTBOX_REAPER_INTERVAL=@every 1m
OUTBOX_REAPER_STUCK_AFTER=5m
```

### Kiwify

```env
KIWIFY_API_BASE_URL=https://public-api.kiwify.com
KIWIFY_CLIENT_ID=CHANGE_ME_generate_secure_client_id
KIWIFY_CLIENT_SECRET=CHANGE_ME_generate_secure_secret_key_min_64_chars
KIWIFY_ACCOUNT_ID=CHANGE_ME_generate_secure_account_id
KIWIFY_PRODUCT_ID_MONTHLY=CHANGE_ME_product_id_monthly
KIWIFY_PRODUCT_ID_QUARTERLY=CHANGE_ME_product_id_quarterly
KIWIFY_PRODUCT_ID_ANNUAL=CHANGE_ME_product_id_annual
KIWIFY_WEBHOOK_SECRET=CHANGE_ME_generate_secure_webhook_secret
KIWIFY_WEBHOOK_SECRET_NEXT=
KIWIFY_WEBHOOK_TOKEN_HEADER=X-Kiwify-Webhook-Token
KIWIFY_OAUTH_TOKEN_SAFETY_MARGIN=5m
KIWIFY_RATE_LIMIT_MAX_REQUESTS_PER_MIN=100
KIWIFY_RATE_LIMIT_BURST=10
KIWIFY_WEBHOOK_RATE_LIMIT_PER_MIN=60
KIWIFY_WEBHOOK_RATE_LIMIT_BURST=30
KIWIFY_WEBHOOK_TRUSTED_PROXIES=
KIWIFY_RECONCILIATION_INTERVAL=@hourly
KIWIFY_RECONCILIATION_BATCH_SIZE=200
KIWIFY_HTTP_TIMEOUT=10s
KIWIFY_HTTP_RETRY_MAX_ATTEMPTS=3
KIWIFY_HTTP_RETRY_BACKOFF=1s
```

### Billing

```env
BILLING_ENTITLEMENT_CACHE_CAPACITY=50000
BILLING_ENTITLEMENT_CACHE_TTL=5m
BILLING_ANONYMIZATION_SCHEDULE=@daily
BILLING_ANONYMIZATION_BATCH_SIZE=500
BILLING_ANONYMIZATION_RETENTION_DAYS=365
BILLING_KIWIFY_EVENTS_RETENTION_DAYS=90
BILLING_KIWIFY_EVENTS_HOUSEKEEPING_SCHEDULE=@daily
BILLING_KIWIFY_EVENTS_HOUSEKEEPING_BATCH=500
BILLING_GRACE_EXPIRATION_SCHEDULE=@daily
```

### Budgets

```env
BUDGETS_PENDING_REAPER_INTERVAL=@every 30s
BUDGETS_PENDING_TTL_HOURS=24
BUDGETS_ABANDONED_DRAFT_CRON=0 3 * * *
BUDGETS_RETENTION_PURGE_CRON=0 4 1 * *
BUDGETS_RETENTION_PURGE_BATCH_SIZE=500
BUDGETS_THRESHOLD_ALERTS_CRON=@hourly
BUDGETS_THRESHOLD_ALERTS_MODE=legacy
BUDGETS_THRESHOLD_ALERTS_SCAN_LIMIT=500
BUDGETS_THRESHOLD_CATEGORY_RATIO=0.80
BUDGETS_THRESHOLD_GOAL_RATIO=0.50
```

### Card

```env
CARD_INVOICE_DUE_ALERTS_ENABLED=false
CARD_INVOICE_DUE_ALERTS_CRON=@daily
CARD_INVOICE_DUE_WINDOW_DAYS=3
CARD_INVOICE_DUE_SCAN_LIMIT=500
```

### Transactions

```env
TRANSACTIONS_ENABLED=false
TRANSACTIONS_IDEMPOTENCY_TTL=24h
TRANSACTIONS_MONTHLY_SUMMARY_DEBOUNCE_WINDOW=1500ms
TRANSACTIONS_RECURRING_MATERIALIZER_CRON=@daily
TRANSACTIONS_MONTHLY_SUMMARY_RECONCILER_CRON=@daily
TRANSACTIONS_MONTHLY_SUMMARY_RECONCILER_LOOKBACK_HOURS=48
TRANSACTIONS_BRAZIL_TIMEZONE=America/Sao_Paulo
```

### Onboarding

```env
ONBOARDING_TOKEN_TTL_DAYS=7
ONBOARDING_OUTREACH_GAP_HOURS=2
ONBOARDING_OUTREACH_ENABLED=false
ONBOARDING_CHECKOUT_CORS_ORIGINS=https://www.mecontrola.app.br,https://mecontrola.app.br
ONBOARDING_TRUSTED_PROXIES=127.0.0.1/32,::1/128
ONBOARDING_CHECKOUT_RATE_LIMIT_PER_MIN=10
ONBOARDING_CHECKOUT_RATE_LIMIT_BURST=5
ONBOARDING_STATE_RATE_LIMIT_PER_MIN=30
ONBOARDING_STATE_RATE_LIMIT_BURST=10
ONBOARDING_KIWIFY_CHECKOUT_URLS=
ONBOARDING_KIWIFY_ALLOWED_HOSTS=pay.kiwify.com.br
ONBOARDING_TOKEN_ENCRYPTION_KEY=CHANGE_ME_32_byte_token_encryption_key
ONBOARDING_TOKEN_EXPIRATION_SCHEDULE=0 3 * * *
ONBOARDING_MAX_TOKEN_LOOKUP_ATTEMPTS=5
ONBOARDING_META_RETENTION_DAYS=30
ONBOARDING_META_CLEANUP_SCHEDULE=30 3 * * *
ONBOARDING_CARD_CLOSING_OFFSET_DAYS=10
ONBOARDING_ABANDONMENT_TTL_HOURS=48
ONBOARDING_ABANDONMENT_JOB_SCHEDULE=@hourly
ONBOARDING_ABANDONMENT_BATCH_SIZE=100
```

### WhatsApp / Meta Cloud API

```env
META_PHONE_NUMBER_ID=CHANGE_ME_meta_phone_number_id
META_ACCESS_TOKEN=CHANGE_ME_meta_access_token
META_APP_SECRET=CHANGE_ME_meta_app_secret
META_APP_SECRET_NEXT=
META_VERIFY_TOKEN=CHANGE_ME_meta_verify_token
META_BOT_NUMBER_E164=+5511900000000
META_BOT_NUMBER_DISPLAY=+55 11 9XXXX-XXXX
WHATSAPP_WEBHOOK_RATE_LIMIT_PER_MIN=600
WHATSAPP_WEBHOOK_RATE_LIMIT_BURST=100
```

> `META_APP_SECRET_NEXT` é opcional — preenchido apenas durante rotação zero-downtime do segredo.

### Alertas Telegram (Grafana)

```env
ALERT_TELEGRAM_BOT_TOKEN=
ALERT_TELEGRAM_CHAT_ID=
```

### Agent / LLM (OpenRouter)

```env
OPENROUTER_BASE_URL=https://openrouter.ai
OPENROUTER_API_KEY=CHANGE_ME_openrouter_api_key
AGENT_LLM_PRIMARY_MODEL=openai/gpt-4o-mini
AGENT_LLM_EMBED_MODEL=openai/text-embedding-3-small
AGENT_LLM_MAX_TOKENS=1536
AGENT_LLM_TEMPERATURE=0
RUN_REAL_LLM=   # defina como "1" para testes de conformidade com LLM real
```

> `RUN_REAL_LLM=1` habilita testes de conformidade que disparam chamadas reais ao OpenRouter via `OPENROUTER_API_KEY`. Nunca ative em pipelines de CI sem controle de custo.

### Gateway Auth (HMAC-SHA256)

Autenticação interna entre o agent LLM e a API. O segredo deve ser gerado com `openssl rand -hex 32`. `NEXT` é opcional e usado durante rotação zero-downtime.

```env
IDENTITY_GATEWAY_SHARED_SECRET_CURRENT=CHANGE_ME_openssl_rand_hex_32
IDENTITY_GATEWAY_SHARED_SECRET_NEXT=
IDENTITY_GATEWAY_AUTH_WINDOW=60s
IDENTITY_AUTH_EVENTS_HOUSEKEEPING_SCHEDULE=@daily
IDENTITY_AUTH_EVENTS_HOUSEKEEPING_BATCH=500
IDENTITY_AUTH_EVENTS_RETENTION_DAYS=90
```

### Workflow Kernel

```env
WORKFLOW_KERNEL_MAX_ATTEMPTS=3
WORKFLOW_KERNEL_RETRY_BASE_BACKOFF=200ms
WORKFLOW_KERNEL_RETRY_MAX_BACKOFF=5s
WORKFLOW_KERNEL_HOUSEKEEPING_RETENTION_DAYS=30
WORKFLOW_KERNEL_HOUSEKEEPING_SCHEDULE=@daily
WORKFLOW_KERNEL_HOUSEKEEPING_BATCH_SIZE=500
```

### Email

```env
EMAIL_PROVIDER=smtp   # smtp (local com mailpit) | resend (produção)
EMAIL_FROM_ADDRESS=noreply@mecontrola.local
EMAIL_FROM_NAME=MeControla
EMAIL_REPLY_TO=

# SMTP (local/mailpit)
SMTP_HOST=mailpit
SMTP_PORT=1025
SMTP_USERNAME=
SMTP_PASSWORD=
SMTP_STARTTLS=false
SMTP_TIMEOUT=10s

# Resend (produção)
RESEND_API_KEY=
RESEND_BASE_URL=https://api.resend.com
EMAIL_HTTP_TIMEOUT=10s
```

> **Produção:** defina `EMAIL_PROVIDER=resend` e preencha `RESEND_API_KEY`.

---

## Gestão de variáveis de ambiente e secrets

Produção usa dois arquivos separados em `deployment/config/`:

| Arquivo | Conteúdo | Criptografia | Commitado |
|---|---|---|---|
| `prod.env` | Variáveis não-secretas (ports, endpoints, feature flags, cron schedules) | Não | Sim |
| `prod.secrets.env` | Segredos (tokens, senhas, chaves de API, gateway secret) | SOPS + age | Sim |

O repositório **nunca** armazena a chave privada `age`. Ela fica em `key.txt` no disco local (já está no `.gitignore`).

### Ferramentas necessárias

- [sops](https://github.com/getsops/sops) — editor/encriptador de secrets
- [age](https://github.com/FiloSottile/age) — criptografia assimétrica

Verifique a chave pública configurada em `.sops.yaml`:

```bash
cat .sops.yaml
```

### 1. Gestão local (nesta máquina) — interface visual

O projeto inclui uma UI terminal para visualizar e editar secrets e variáveis de ambiente sem manipular SOPS manualmente.

```bash
go run ./cmd/configui
```

A interface abre:

- `deployment/config/prod.env` — leitura e edição direta.
- `deployment/config/prod.secrets.env` — descriptografa ao abrir, re-criptografa ao salvar usando a chave `key.txt`.

Fluxo recomendado para alterar um secret:

```bash
# 1. Abra a UI, edite o valor e salve
go run ./cmd/configui

# 2. Verifique que o arquivo de secrets ainda está criptografado
head -5 deployment/config/prod.secrets.env
# deve começar com sops/age headers, nunca valores em texto claro

# 3. Commit + push das alterações
git add deployment/config/prod.env deployment/config/prod.secrets.env
git commit -m "chore(secrets): atualiza X"
git push origin main

# 4. Deploy para a VPS
export VPS_HOST=187.77.45.48 VPS_USER=root VPS_DEPLOY_PATH=/opt/mecontrola
export AGE_PRIVATE_KEY="$(cat key.txt)"
bash deployment/scripts/deploy-full.sh --local "$(git rev-parse --short HEAD)"
```

> **Atenção:** `key.txt` não pode ser commitado. Se for perdida, os secrets em `prod.secrets.env` ficam irrecuperáveis.

### 2. Gestão manual local (CLI)

Se preferir não usar a UI:

```bash
# Editar variáveis não-secretas
vim deployment/config/prod.env

# Editar secrets (abre editor padrão com conteúdo descriptografado)
SOPS_AGE_KEY_FILE=key.txt sops deployment/config/prod.secrets.env

# Rotacionar um secret sem alterar seu valor (força recriação no Swarm)
MODE=rotate SOPS_AGE_KEY_FILE=key.txt bash deployment/scripts/deploy-full.sh --local
```

### 3. Gestão diretamente na VPS

Use apenas em emergência ou quando não tiver acesso ao repo local. Necessita SSH `root` e a chave privada `age`.

```bash
# Acesse a VPS
ssh root@187.77.45.48
cd /opt/mecontrola
```

Na VPS **não existe mais `.env` persistente**. Os secrets vivem como Docker Swarm secrets:

```bash
# Listar secrets atuais
docker secret ls --filter name=mecontrola_

# Inspecionar um secret (mostra apenas metadata, nunca o valor)
docker secret inspect mecontrola_DB_PASSWORD
```

Para alterar um secret diretamente na VPS:

```bash
# 1. Crie um arquivo temporário com o novo valor
cat > /tmp/new-secret.env <<EOF
DB_PASSWORD=novo-valor-seguro-minimo-16-caracteres
EOF
chmod 600 /tmp/new-secret.env

# 2. Atualize o Docker secret (rotaciona automaticamente nos services)
MODE=rotate bash deployment/scripts/create-secrets.sh /tmp/new-secret.env

# 3. Remova o arquivo temporário
shred -u /tmp/new-secret.env
```

Para alterar variáveis não-secretas (que vêm de `prod.env`), edite o arquivo na VPS e faça redeploy:

```bash
vim deployment/config/prod.env

# Redeploy com a tag atual
export IMAGE_TAG=$(git rev-parse --short HEAD)
python3 deployment/scripts/render-stack.py deployment/compose/compose.swarm.yml \
  --env-file deployment/config/prod.env \
  --secrets-env-file <(SOPS_AGE_KEY_FILE=/root/.config/sops/age/key.txt sops --decrypt deployment/config/prod.secrets.env) \
  > /tmp/stack.yml
docker stack deploy -c /tmp/stack.yml mecontrola
rm -f /tmp/stack.yml
```

> **Preferência:** sempre que possível, edite localmente via `cmd/configui` e faça deploy pelo `deploy-full.sh`. As alterações manuais na VPS não são rastreadas pelo git e podem ser sobrescritas no próximo deploy.

### 4. Onde fica a chave privada na VPS?

A VPS precisa da chave privada `age` apenas para descriptografar durante o deploy. O local padrão é:

```bash
/root/.config/sops/age/key.txt
```

O `deploy-full.sh` (quando disparado da máquina local) não deixa a chave privada persistente na VPS: ela trafega por `/tmp` e é removida ao final. Para deploys manuais iniciados diretamente na VPS, mantenha `key.txt` protegido com permissão `0600`.

---

## Subir só a infra

Use quando precisar apenas dos serviços de suporte (banco de dados, observabilidade, email) sem rodar a aplicação — por exemplo, ao desenvolver com `go run` direto ou ao depurar via VS Code.

```bash
task local:infra
```

Equivalente manual:

```bash
docker compose \
  -f deployment/compose/compose.yml \
  -f deployment/compose/compose.local.yml \
  up -d postgres otel-lgtm mailpit
```

Endpoints disponíveis após subir:

| Serviço | Endereço |
|---|---|
| PostgreSQL | `localhost:5432` |
| Grafana | `http://localhost:3000` (admin / valor de `OTEL_LGTM_ADMIN_PASSWORD`) |
| OTLP gRPC | `localhost:4317` |
| OTLP HTTP | `localhost:4318` |
| Mailpit Web UI | `http://localhost:8025` |

---

## Subir tudo (infra + migrate + server + worker)

Sobe o ambiente completo em sequência determinística. Use no dia a dia quando não precisar de debug com breakpoints.

```bash
task local:up
```

Sequência executada internamente:

1. `docker compose up -d postgres otel-lgtm mailpit` — aguarda healthcheck do postgres
2. `docker compose run --rm migrate` — aplica migrations pendentes e sai
3. `docker compose up -d server worker` — sobe e fica em background

Endpoints após subir:

| Serviço | Endereço |
|---|---|
| API | `http://localhost:8080` |
| Health | `http://localhost:8080/health` |
| Grafana | `http://localhost:3000` |
| Mailpit Web UI | `http://localhost:8025` |

Outros comandos de gerenciamento do ambiente local:

```bash
task local:down       # para e remove containers (preserva volumes)
task local:destroy    # para + remove volumes (apaga dados) — pede confirmação
task local:logs       # tail de todos os containers (Ctrl+C para sair)
task local:ps         # status dos containers
task local:db:restart # reinicia apenas postgres e pgbouncer
```

---

## Debug no VS Code

O projeto vem com `.vscode/launch.json` configurado para depurar `server`, `worker` e `migrate` individualmente ou em conjunto. Não é necessário subir os containers da app — apenas a infra.

**Pré-requisitos:** extensão [Go for VS Code](https://marketplace.visualstudio.com/items?itemName=golang.go) instalada, `.env` preenchido, infra no ar.

```bash
task local:infra   # postgres + otel-lgtm + mailpit
task migrate:up    # aplica migrations
# VS Code: F5 → selecionar configuração
```

Todas as configurações injetam automaticamente:

| Variável | Valor |
|---|---|
| `DB_HOST` | `localhost` |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `localhost:4317` |
| `LOG_LEVEL` | `debug` |
| `LOG_FORMAT` | `console` |

Configurações disponíveis em `.vscode/launch.json`:

| Configuração | Tipo | Quando usar |
|---|---|---|
| `server` | `go` — `cmd server` + `.env` | Depurar o HTTP server com breakpoints |
| `worker` | `go` — `cmd worker` + `.env` | Depurar jobs em background |
| `migrate` | `go` — `cmd migrate` + `.env` | Aplicar migrations em debug |
| `Test: current file` | `go/test` — arquivo atual | Depurar testes de um único arquivo |
| `Test: current package` | `go/test` — pacote atual | Depurar testes com seleção interativa |
| `Test: integration suite` | `go/test` — tag `integration` | Depurar testes de integração |
| `server (attach to PID)` | `go` — attach | Anexar a processo já em execução |
| `Stack: server + worker` | compound | Depurar fluxo completo; `stopAll: true` ao encerrar |

> Alternativa via CLI: `dlv debug ./cmd -- server`

---

## Comandos Task

O projeto usa [Task v3.51.1](https://taskfile.dev). Execute `task --list-all` para ver todas as tasks. O `Taskfile.yml` raiz inclui taskfiles especializados em `taskfiles/`.

### Setup e Inicialização

| Task | Objetivo | Quando executar |
|---|---|---|
| `task setup` | Instala pre-commit hooks, gitsign, golangci-lint pinado e configura assinatura de commits | Uma vez ao clonar |
| `task mocks:mocks` | Regenera mocks via mockery conforme `.mockery.yml` | Após alterar interfaces |
| `task mocks:clean` | Remove todos os mocks gerados | Antes de regenerar do zero |
| `task mocks:verify` | Falha se os mocks estiverem desatualizados | CI |

### Build

| Task | Objetivo |
|---|---|
| `task build:build` | Compila binário para o SO atual em `bin/mecontrola` (CGO_ENABLED=0, -trimpath) |
| `task build:all` | Cross-compile linux/darwin/windows × amd64/arm64 em `bin/` |
| `task build:docker:build IMAGE_TAG=<tag>` | Build da imagem Docker multi-stage distroless (≤30 MB, USER 65532) |
| `task build:clean` | Remove `bin/` e `.task/` |
| `task run` | Compila e executa o server localmente — requer infra no ar |

### Desenvolvimento local

| Task | Objetivo |
|---|---|
| `task local:infra` | Sobe postgres + otel-lgtm + mailpit sem aplicação |
| `task local:up` | Sequência completa: infra → migrate → server + worker |
| `task local:down` | Para e remove containers (preserva volumes) |
| `task local:destroy` | Para + remove volumes (apaga dados) — pede confirmação |
| `task local:logs` | Tail de todos os containers |
| `task local:ps` | Status dos containers |
| `task local:db:restart` | Reinicia apenas postgres e pgbouncer sem derrubar server/worker |

### Migrations

| Task | Objetivo |
|---|---|
| `task migrate:up` | Aplica todas as migrations pendentes (lê `.env`) |
| `task migrate:down` | Reverte todas as migrations (`--steps -1`) |
| `task migrate:create -- <nome>` | Cria novo par de arquivos SQL numerado em `migrations/` |

### Testes

| Task | Objetivo | Depende de |
|---|---|---|
| `task test:all` | Unitários + integração | Docker (integração) |
| `task test:unit` | Unitários com `-race` e cobertura em `coverage/unit.out` | — |
| `task test:integration` | Integração com testcontainers | Docker disponível |
| `task test:coverage` | Relatório HTML em `coverage/coverage.html` | `test:unit` |
| `task test:coverage:identity` | Cobertura do módulo identity com validação de pontos críticos (RF-17) | `test:unit` |
| `task test:e2e` | Testes E2E BDD com Godog (requer Docker) | Docker disponível |
| `task test:conformance:real` | Suite de conformidade do weather agent com LLM real (`RUN_REAL_LLM=1`) | OpenRouter API key |
| `task test:watch` | Re-executa unitários ao salvar | — |
| `task card:test` | Unitários do módulo card com `-race` | — |
| `task card:integration` | Integração do módulo card | Docker disponível |

### Lint e qualidade

| Task | Objetivo |
|---|---|
| `task lint:run` | golangci-lint pinado em `.tools/bin` + gates: auth-bypass, outbox-user-id, deadcode |
| `task lint:fix` | Aplica correções automáticas do linter |
| `task lint:fmt` | gofmt + goimports |
| `task lint:fmt:check` | Falha se arquivo não formatado (uso em CI) |
| `task lint:tidy` | `go mod tidy` + verifica drift em `go.mod`/`go.sum` |
| `task lint:pci` | Gate RF-16: bloqueia PAN/CVV/CVC/track/PIN em produção |
| `task lint:user-isolation` | Gate: UPDATE/DELETE sem `user_id` na WHERE em repos per-user |
| `task lint:auth-bypass` | Gate M-09: `RequireGatewayAuth` obrigatório antes de `InjectPrincipal` |
| `task lint:outbox-user-id` | Gate: `AggregateUserID` obrigatório em `EventInput` |
| `task lint:outbox-user-id:test` | Regressão do gate outbox-user-id com fixtures |
| `task lint:deadcode` | Gate RF-40: código morto detectado pelo deadcode |
| `task card:lint` | golangci-lint pinado em `.tools/bin` no escopo card (inclui regra forbidigo PCI) |
| `task card:audit` | Auditoria R0–R7: init, panic, clock, interface-assertion, zero-comentários, SQL em adapter, PCI |

### Validação rápida

| Task | Objetivo |
|---|---|
| `task check` | `lint:run` + `test:unit` + `security:vulncheck` — executar antes de abrir PR |
| `task ci:pipeline` | Pipeline CI completa (lint + testes + segurança + build) |
| `task ci:fast` | Subconjunto rápido para feedback em PR (lint + testes unitários) |
| `task ci:agent-boundary` | Gate de fronteira de dados do `internal/agents` |
| `task ci:platform-gates` | Gates R-WF-KERNEL-001, R-AGENT-WF-001 (kernel sem domínio, sem LLM, estados fechados) |
| `task ci:no-internal-agent` | Gate ADR-004 cutover: `internal/agent` ausente + sem imports proibidos |

### Gates de arquitetura

| Task | Objetivo |
|---|---|
| `task gates:platform` | 5 gates: kernel sem import de domínio, sem LLM no kernel, zero comments, cardinalidade controlada, tipos fechados sem string |
| `task gates:no-internal-agent` | Gate ADR-004: ausência de `internal/agent` confirmada |

### Segurança

| Task | Objetivo | Requer |
|---|---|---|
| `task security:vulncheck` | govulncheck nas dependências Go | govulncheck |
| `task security:scan` | vulncheck + audit | govulncheck |
| `task security:audit` | `go list -m -u all` + `go mod verify` | — |
| `task security:image-scan IMAGE_SHA=<sha>` | Trivy na imagem do GHCR (HIGH/CRITICAL) | trivy, acesso GHCR |
| `task security:sbom IMAGE_SHA=<sha>` | Gera `sbom.spdx.json` da imagem | trivy, acesso GHCR |
| `task security:sign-image IMAGE_REF=<ref> IMAGE_SHA=<sha>` | Assina imagem via cosign keyless + SBOM + provenance attestations | cosign, OIDC GitHub Actions |
| `task security:verify-image IMAGE_SHA=<sha>` | Verifica assinatura cosign keyless | cosign |
| `task security:vps:firewall VPS_HOST=<ip>` | Aplica regras ufw no VPS via SSH (22/80/443) | SSH + sudo no VPS |

### ngrok — webhooks locais

Use para testar integrações Meta/WhatsApp e Kiwify apontando para `localhost`.

| Task | Objetivo |
|---|---|
| `task ngrok:check` | Valida pré-requisitos (docker, ngrok configurado, `.env`, curl) |
| `task ngrok:server` | Sobe ambiente completo + abre túnel ngrok → `127.0.0.1:8080` |
| `task ngrok:caddy` | Sobe ambiente com perfil proxy + túnel → `:80` |
| `task ngrok:urls` | Imprime URLs públicas dos webhooks ativos (Meta verify/inbound, Kiwify) |
| `task ngrok:stop:tips` | Exibe como encerrar o túnel e desligar os containers |

### Benchmarks

| Task | Objetivo |
|---|---|
| `task bench:outbox` | Benchmark do outbox publisher com 5 runs |

### Deploy

| Task | Objetivo |
|---|---|
| `task swarm:local:up` | Paridade com produção localmente — init + deploy Swarm com `compose.swarm.yml` |
| `task swarm:prod:deploy:full IMAGE_TAG=<tag>` | Deploy completo na VPS — descriptografa secrets SOPS, migrations, deploy Swarm, healthcheck |
| `task swarm:prod:deploy:full:local` | Build local + transferência direta para VPS sem GHCR (build amd64 + `docker save\|load`) |
| `task swarm:prod:rollback PREVIOUS_TAG=<tag>` | Rollback manual para tag anterior |

---

## Sequências comuns

**Primeira vez (clone do zero):**

```bash
cp .env.example .env   # preencher CHANGE_ME_* e ajustar valores locais
task setup             # pre-commit + gitsign + golangci-lint pinado
task local:up          # infra + migrate + server + worker
```

**Ciclo de desenvolvimento diário:**

```bash
# Com Docker (server/worker em container):
task local:up

# Com debug no VS Code (server/worker no debugger):
task local:infra && task migrate:up
# → F5 no VS Code, selecionar "server + worker"

# Antes de abrir PR:
task check
```

**Reset completo do banco local:**

```bash
task local:destroy   # remove volumes (confirma prompt)
task local:up        # recria do zero
```

**Testar webhook com ngrok:**

```bash
task ngrok:server    # sobe ambiente completo + abre túnel
task ngrok:urls      # copia URLs → configurar no Meta/Kiwify Dashboard
# Ctrl+C para encerrar o túnel
task local:down      # para os containers
```

**Executar testes de conformidade do agent com LLM real:**

```bash
RUN_REAL_LLM=1 task test:conformance:real
```

---

## Reset do banco de produção

Procedimento para zerar o banco de produção e recriar o schema exclusivamente a partir das migrations atuais do projeto.

> ⚠️ **Operação destrutiva e irreversível.** Execute apenas em janela de manutenção e com backup validado.

### Antes de começar

1. Confirme que o deploy de produção usa a stack Swarm em `/opt/mecontrola`.
2. Faça backup antes do reset:

```bash
ssh root@187.77.45.48
cd /opt/mecontrola
task swarm:prod:pgbackrest:backup TYPE=full
task swarm:prod:pgbackrest:info
```

3. Pause a aplicação para evitar escrita concorrente durante o reset:

```bash
STACK=mecontrola
docker service scale \
  ${STACK}_server-1=0 \
  ${STACK}_server-2=0 \
  ${STACK}_worker-1=0 \
  ${STACK}_worker-2=0
```

### Reset na VPS usando a imagem atual

```bash
ssh root@187.77.45.48
cd /opt/mecontrola

STACK=mecontrola
IMAGE_NAME=${IMAGE_NAME:-ghcr.io/limateixeiratecnologia/mecontrola}
IMAGE_TAG=$(grep '^IMAGE_TAG=' .env | cut -d= -f2)

# Reverter TODAS as migrations
docker run --rm \
  --network "${STACK}_backend" \
  --env-file .env \
  -e ENVIRONMENT=production \
  -e DB_HOST=postgres \
  -e DB_PORT=5432 \
  "${IMAGE_NAME}:${IMAGE_TAG}" \
  migrate-down --steps -1

# Reaplicar TODAS as migrations
docker run --rm \
  --network "${STACK}_backend" \
  --env-file .env \
  -e ENVIRONMENT=production \
  -e DB_HOST=postgres \
  -e DB_PORT=5432 \
  "${IMAGE_NAME}:${IMAGE_TAG}" \
  migrate
```

### Verificação pós-reset

```bash
STACK=mecontrola
POSTGRES_CONTAINER=$(docker ps --filter name="${STACK}_postgres." --format '{{.Names}}' | head -n1)

# Confirma mecontrola.schema_migrations consistente (1 versão, dirty = false)
docker exec "${POSTGRES_CONTAINER}" \
  psql -U "${DB_USER:-mecontrola}" -d "${DB_NAME:-mecontrola_db}" \
  -c 'SELECT version, dirty FROM mecontrola.schema_migrations ORDER BY version;'

# Confirma seed do dicionário
docker exec "${POSTGRES_CONTAINER}" \
  psql -U "${DB_USER:-mecontrola}" -d "${DB_NAME:-mecontrola_db}" \
  -c 'SELECT COUNT(*) FROM mecontrola.category_dictionary;'
```

Resultado esperado:
- última versão em `mecontrola.schema_migrations` = `1`
- `dirty = false`
- `category_dictionary` com dados seedados

### Reativar a aplicação

```bash
STACK=mecontrola
docker service scale \
  ${STACK}_server-1=1 \
  ${STACK}_server-2=1 \
  ${STACK}_worker-1=1 \
  ${STACK}_worker-2=1
```

### Execução remota a partir do macOS

```bash
ssh root@187.77.45.48 '
  cd /opt/mecontrola &&
  STACK=mecontrola &&
  IMAGE_NAME=${IMAGE_NAME:-ghcr.io/limateixeiratecnologia/mecontrola} &&
  IMAGE_TAG=$(grep "^IMAGE_TAG=" .env | cut -d= -f2) &&
  docker run --rm --network "${STACK}_backend" --env-file .env \
    -e ENVIRONMENT=production -e DB_HOST=postgres -e DB_PORT=5432 \
    "${IMAGE_NAME}:${IMAGE_TAG}" migrate-down --steps -1 &&
  docker run --rm --network "${STACK}_backend" --env-file .env \
    -e ENVIRONMENT=production -e DB_HOST=postgres -e DB_PORT=5432 \
    "${IMAGE_NAME}:${IMAGE_TAG}" migrate
'
```

### Reset completo do banco local

```bash
task local:destroy
task local:up
```

---

## CI/CD

O fluxo de CI roda em `.github/workflows/ci.yml` (pull requests e merge group) e o fluxo de CD roda em `.github/workflows/cd.yml` (push na `main` e `workflow_dispatch`).

### CI (`.github/workflows/ci.yml`)

Roda em todo pull request e merge group.

| Job | O que faz |
|---|---|
| `quality` | `task lint:run` + `lint:fmt:check` + `lint:pci` + `lint:tidy` |
| `unit` | `task test:unit` |
| `integration` | `task test:integration` com Docker/testcontainers |
| `vulncheck` | `task security:vulncheck` |
| `gates` | `task ci:agent-boundary`, `ci:platform-gates`, `ci:no-internal-agent`, `ci:deploy-anti-storm` |
| `build` | `task build:build` |
| `quality-gates` | agregador — bloqueia merge se qualquer job acima falhar |
| `dependency-review` | análise de dependências do PR |

### CD (`.github/workflows/cd.yml`)

Roda em `push` na `main` e manualmente via `workflow_dispatch`.

| Job | Quando | O que faz |
|---|---|---|
| `quality` | sempre | mesmas verificações do CI |
| `unit` | sempre | `task test:unit` |
| `integration` | sempre | `task test:integration` |
| `vulncheck` | sempre | `task security:vulncheck` |
| `gates` | sempre | governance gates consolidados |
| `build` | sempre | `task build:build` + upload do binário como artifact |
| `quality-gates` | após jobs de qualidade | agregador |
| `build-image` | após quality-gates | build + push da imagem GHCR com tag = SHA curto + provenance + SBOM |
| `scan-sign-image` | após build-image | Trivy image scan + assinatura cosign keyless |
| `deploy` | `main`, após scan + sign | deploy Swarm em produção via runner GitHub-hosted usando SSH + SOPS + age |
| `healthcheck` | após deploy | valida `/healthz` e `/readyz` com retry |
| `notify` | `main` | notificação Telegram com status da run |

### E2E manual (`.github/workflows/e2e.yml`)

Workflow manual para testes E2E BDD com Godog (`task test:e2e`) e upload de `coverage/e2e.out`, com notificação opcional no Telegram ao final.

### Dependabot (`.github/workflows/auto-merge.yml`)

Dependabot atualiza semanalmente (gomod, github-actions, docker). PRs de minor/patch habilitam auto-merge automaticamente após checks passarem. PRs de major ficam abertos para revisão manual.

---

## Docker Swarm

A arquitetura de produção usa Docker Swarm single-node com 2 réplicas de `server` e 2 de `worker`. A stack está em `deployment/compose/compose.swarm.yml`.

**Serviços e recursos em produção:**

| Serviço | Réplicas | CPU (lim/res) | RAM (lim/res) | Notas |
|---|---|---|---|---|
| `postgres` | 1 | 1.0 / 0.25 | 2 GB / 512 M | — |
| `pgbouncer` | 1 | 0.25 / — | 128 M / — | Connection pooling |
| `postgres-exporter` | 1 | — | — | Métricas Prometheus |
| `node-exporter` | 1 | — | — | Métricas de nó |
| `migrate` | 1 | — | — | One-shot (`restart: none`) |
| `server-1`, `server-2` | 1 cada | 0.75 / — | 512 M / 128 M | UID 65532, read-only fs |
| `worker-1`, `worker-2` | 1 cada | 0.50 / — | 256 M / 64 M | UID 65532, read-only fs |
| `caddy` | 1 | 0.25 / — | 128 M / — | Reverse proxy |
| `otel-lgtm` | 1 | 0.50 / — | 512 M / — | Observabilidade |

**Secrets externos** (gerenciados via `task swarm:prod:secrets`):

```
mecontrola_DB_PASSWORD
mecontrola_META_ACCESS_TOKEN
mecontrola_META_APP_SECRET
mecontrola_KIWIFY_WEBHOOK_SECRET
mecontrola_KIWIFY_CLIENT_SECRET
mecontrola_OPENROUTER_API_KEY
mecontrola_ONBOARDING_TOKEN_ENCRYPTION_KEY
mecontrola_IDENTITY_GATEWAY_SHARED_SECRET_CURRENT
mecontrola_IDENTITY_GATEWAY_SHARED_SECRET_NEXT
```

### Swarm local (paridade com produção)

```bash
task swarm:local:up                      # comando canônico: init + deploy (idempotente)
task swarm:local:config                  # valida compose.swarm.yml
task swarm:local:ps                      # lista services
task swarm:local:logs                    # segue logs
task swarm:local:rm                      # remove stack local
```

### Deploy Swarm em produção

```bash
# Etapas individuais (não usa .env persistente na VPS)
task swarm:prod:sync                           # rsync código para VPS
task swarm:prod:secrets                        # cria/atualiza Docker secrets
task swarm:prod:migrate                        # migrations com advisory lock
task swarm:prod:deploy IMAGE_TAG=<tag>         # deploy + health check + rollback automático
task swarm:prod:ps                             # verifica services
task swarm:prod:health                         # verifica healthchecks

# Ou em um único comando:
IMAGE_TAG=<tag> task swarm:prod:sync swarm:prod:secrets swarm:prod:migrate swarm:prod:deploy swarm:prod:health
```

### Rollback

```bash
task swarm:prod:rollback
```

### Backup e restore PostgreSQL (pgBackRest)

```bash
task swarm:prod:pgbackrest:check              # verifica configuração
task swarm:prod:pgbackrest:backup TYPE=full   # backup completo
task swarm:prod:pgbackrest:backup TYPE=diff   # backup diferencial
task swarm:prod:pgbackrest:backup TYPE=incr   # backup incremental
task swarm:prod:pgbackrest:info               # lista backups disponíveis
```

Para restore PITR e recuperação completa da VPS, siga os runbooks:
- `deployment/runbooks/restore-pitr.md`
- `deployment/runbooks/restore-vps.md`

### Alertas e observabilidade

```bash
task swarm:prod:alert:test   # renderiza o provisioning do Grafana + dispara teste no Telegram
```

| Sinal | Retenção |
|---|---|
| Logs (Loki) | 7 dias |
| Traces (Tempo) | 7 dias |
| Métricas (Prometheus) | 15 dias |

---

## Deploy da máquina local direto na VPS (`deploy-local.sh`)

Deploy de um único comando, **da sua máquina direto para a VPS, sem depender do GHCR nem da CI/CD**. Use quando a pipeline estiver indisponível ou quando precisar subir uma correção rápida.

O script `deployment/scripts/deploy-local.sh` faz, em sequência:

1. **Build** da imagem `linux/amd64` localmente.
2. **Transferência** da imagem para a VPS via `docker save | gzip | ssh docker load` (sem `docker push`/GHCR).
3. **Preparação na VPS** — `git pull --ff-only` + `create-secrets.sh` + `backup-env-s3.sh` (se AWS configurado) + migrations.
4. **Deploy** — renderização de `compose.swarm.yml` via `render-stack.py` + `docker stack deploy` + server/worker com nova tag.
5. **Healthcheck** com rollback automático + alinhamento do `IMAGE_TAG` no `.env` da VPS.

### Pré-requisitos

| Requisito | Detalhe |
|---|---|
| Docker local | daemon ativo (build + `docker save`) |
| Acesso SSH por chave à VPS | sem senha (`BatchMode`); a chave padrão ou `VPS_SSH_KEY` |
| Árvore git limpa | a tag = short SHA do commit; suja é bloqueada (use `ALLOW_DIRTY=true` para burlar) |
| Secrets na VPS | `deployment/config/prod.env` + `deployment/config/prod.secrets.env` (SOPS + age) presentes no repo da VPS |

### Passo a passo

```bash
# 1. (recomendado) commit + push para manter a VPS em sync via git pull
git add -A && git commit -m "fix: ..." && git push

# 2. deploy — tag default = short SHA do HEAD
bash deployment/scripts/deploy-local.sh

# ou com uma tag explícita:
bash deployment/scripts/deploy-local.sh 1a2b3c4
```

Atalho via Task:

```bash
task deploy:local              # tag = short SHA do HEAD
task deploy:local -- 1a2b3c4   # tag explícita
```

Saída esperada ao final:

```
[..] 1/5 build ghcr.io/limateixeiratecnologia/mecontrola:<tag>
[..] 2/5 transferindo imagem para a VPS (docker save | ssh docker load)
[..] 3/5 migrate + 4/5 server/worker + healthcheck (no host)
[vps] migrate
[vps] up server worker
[vps] healthy após 10s
[vps] === verificação pós-deploy ===
[vps] mecontrola.schema_migrations (version dirty): 1|f
[vps] mecontrola-server-1 ...:<tag> Up 5 seconds (healthy)
[vps] mecontrola-worker-1 ...:<tag> Up 5 seconds (healthy)
[vps] HEAD host: <tag>
[..] 5/5 deploy concluído — <tag> em produção e saudável
```

### Variáveis de override

| Variável | Padrão | Uso |
|---|---|---|
| `IMAGE_TAG` | short SHA do `HEAD` | tag da imagem (também aceita como `$1`) |
| `VPS_HOST` | `187.77.45.48` | host da VPS |
| `VPS_USER` | `root` | usuário SSH |
| `VPS_DEPLOY_PATH` | `/opt/mecontrola` | raiz do deploy na VPS |
| `VPS_SSH_KEY` | (chave padrão) | caminho de uma chave SSH específica |
| `IMAGE_NAME` | `ghcr.io/limateixeiratecnologia/mecontrola` | nome base da imagem |
| `PLATFORM` | `linux/amd64` | plataforma alvo do build |
| `HEALTH_RETRIES` / `HEALTH_INTERVAL` | `24` / `5` | tentativas e intervalo (s) do healthcheck |
| `ALLOW_DIRTY` | `false` | permite build com árvore git suja |
| `SKIP_BUILD` | `false` | pula o build e reusa a imagem local existente |

```bash
# exemplo: deploy para outra VPS, reaproveitando a imagem já buildada
VPS_HOST=10.0.0.9 SKIP_BUILD=true bash deployment/scripts/deploy-local.sh
```

> **Segurança:** o script aborta antes de tocar a VPS se a árvore git estiver suja ou se o SSH falhar. Em falha de healthcheck, faz rollback automático para a imagem anterior. As migrations rodam **antes** do app; se falharem, o deploy aborta e os containers atuais permanecem intactos.

> **Quando usar a CI/CD em vez disto:** o caminho padrão de produção é a pipeline (build assinado por cosign + scan Trivy + SBOM). O `deploy-local.sh` é um atalho operacional — ele **não** assina a imagem nem gera SBOM.

---

## Acesso Remoto

### VPS — SSH direto

```bash
ssh root@187.77.45.48
cd /opt/mecontrola
```

### Banco de dados via DBeaver

Adicione os aliases ao `~/.zshrc` ou `~/.bashrc`:

```bash
alias mecontrola-db="ssh -N -L 5433:172.18.0.2:5432 root@187.77.45.48"
```

**Passo a passo:**

1. Em um terminal, abra o túnel e mantenha-o aberto:

   ```bash
   mecontrola-db
   ```

2. No DBeaver, crie uma nova conexão PostgreSQL:

   | Campo | Valor |
   |---|---|
   | Host | `localhost` |
   | Porta | `5433` (túnel local) |
   | Database | `mecontrola_db` |
   | User | `mecontrola` |
   | Password | valor de `DB_PASSWORD` no `.env` da VPS |
   | SSL | `disable` |

3. O túnel mapeia `localhost:5433` → container Postgres interno (`172.18.0.2:5432`).

### Grafana da VPS

Adicione os aliases ao `~/.zshrc` ou `~/.bashrc`:

```bash
alias mecontrola-o11y="ssh -N -L 3001:127.0.0.1:3000 root@187.77.45.48"
```

**Passo a passo:**

1. Em um terminal, abra o túnel e mantenha-o aberto:

   ```bash
   mecontrola-o11y
   ```

2. Acesse no browser: `http://localhost:3001`

   | Campo | Valor |
   |---|---|
   | User | `admin` |
   | Password | valor de `OTEL_LGTM_ADMIN_PASSWORD` no `.env` da VPS |

3. O túnel mapeia `localhost:3001` → Grafana da VPS (`127.0.0.1:3000`).

---

## skills-lock.json

O arquivo `skills-lock.json` na raiz controla as skills de IA externas usadas pelo projeto. Cada skill aponta para um `SKILL.md` versionado em repositório GitHub externo; o `computedHash` garante que o conteúdo carregado pelos agentes é exatamente o que foi auditado.

**Formato:**

```json
{
  "version": 1,
  "skills": {
    "<nome>": {
      "source": "owner/repo",
      "sourceType": "github",
      "skillPath": "skills/<nome>/SKILL.md",
      "computedHash": "<hash>"
    }
  }
}
```

**Skills registradas** (fonte: `JailtonJunior94/skills`)

| Nome | Descrição |
|---|---|
| `azure-devops-epic-stories` | Geração de épicos e histórias de usuário para Azure DevOps |
| `decision-brainstorming` | Brainstorming estruturado de decisões técnicas |
| `prompt-enricher` | Enriquecimento de prompts para LLMs |
| `taskfile-production` | Criação e manutenção de Taskfiles em produção |
| `technical-discovery-production` | Discovery técnico de projetos |
| `tracker-to-prd` | Conversão de issues/tickets para PRD |

Para atualizar: edite `skills-lock.json` com o hash correto do `SKILL.md` na fonte e abra PR para revisão.

---

## Contribuição

1. **Abra uma issue** antes de iniciar qualquer mudança de escopo maior para alinhar contexto e abordagem.
2. **Siga Conventional Commits** — o hook `commit-msg` instalado por `task setup` valida esse padrão localmente (`feat:`, `fix:`, `chore:`, etc.).
3. **Execute `task check`** antes de abrir PR — roda lint, testes unitários e vulncheck localmente.
4. **Execute `task setup`** ao clonar — instala pre-commit hooks, provisiona o `golangci-lint` pinado e configura gitsign para assinatura de commits.
5. **Não flexibilize regras de arquitetura** — as regras em `AGENTS.md` são inegociáveis e verificadas automaticamente no CI.

---

## Governance

Referências canônicas para regras de arquitetura, ADRs e especificações de produto.

| Artefato | Localização | Conteúdo |
|---|---|---|
| Regras e skills | `AGENTS.md` | Fonte canônica de arquitetura, ADRs e regras obrigatórias |
| PRDs e techspecs | `.specs/` | Especificações por módulo |
| Arquitetura completa | `docs/diagrams/architecture.md` | Visão textual consolidada da arquitetura, bootstrap e fluxos principais |
| Diagramas C4 | `docs/diagrams/` | PlantUML por módulo (container + fluxos) |
| Coleção Postman | `docs/postman/` | Endpoints + environment |
| Regras transversais | `.claude/rules/` | R-ADAPTER-001, R-WF-KERNEL-001, R-AGENT-WF-001, R-TXN-WORKFLOWS-001, R-TESTING-001, R-DTO-VALIDATE-001, R-GOV-001 |
| Runbooks operacionais | `deployment/runbooks/` | Deploy, PITR restore, rollback, rotação de secrets, firewall |
