# MeControla

[![CI](https://github.com/LimaTeixeiraTecnologia/mecontrola/actions/workflows/ci.yml/badge.svg)](https://github.com/LimaTeixeiraTecnologia/mecontrola/actions/workflows/ci.yml)
![Signed Image](https://img.shields.io/badge/image-signed%20cosign-brightgreen)
![SBOM Available](https://img.shields.io/badge/SBOM-SPDX--JSON-blue)
![Governance](https://img.shields.io/badge/governance-ai--spec-purple)

Monolito modular em Go para fluxos financeiros conversacionais via WhatsApp e Telegram, com bootstrap separado para server, worker e migrations.

---

## Índice

- [Pré-requisitos](#pré-requisitos)
- [Stack](#stack)
- [Módulos e responsabilidades](#módulos-e-responsabilidades)
- [Entrypoints](#entrypoints)
- [Configuração (.env)](#configuração-env)
- [Subir só a infra](#subir-só-a-infra)
- [Subir tudo (infra + migrate + server + worker)](#subir-tudo-infra--migrate--server--worker)
- [Debug no VS Code](#debug-no-vs-code)
- [Comandos Task](#comandos-task)
- [Sequências comuns](#sequências-comuns)
- [Reset do banco de produção](#reset-do-banco-de-produção)
- [CI/CD](#cicd)
- [Deploy da máquina local direto na VPS (deploy-local.sh)](#deploy-da-máquina-local-direto-na-vps-deploy-localsh)
- [Contribuição](#contribuição)
- [Governance](#governance)

---

## Pré-requisitos

Ferramentas necessárias para desenvolvimento local. Execute `task setup` após instalar para configurar hooks e assinatura de commits.

| Ferramenta | Versão mínima | Obrigatório | Instalação |
|---|---|---|---|
| Docker Engine + Compose v2 | Docker 24+ | Sim | [docs.docker.com](https://docs.docker.com/engine/install/) |
| Go | 1.26+ | Sim (desenvolvimento) | [go.dev/dl](https://go.dev/dl/) |
| Task | 3.51.1 | Sim | `go install github.com/go-task/task/v3/cmd/task@v3.51.1` |
| golangci-lint | v2.12.2 | Sim (lint) | instalado via `task setup` |
| mockery | v2.53.3 | Sim (mocks) | instalado via `task setup` |
| govulncheck | v1.1.4 | Sim (security) | instalado via `task setup` |
| trivy | v0.62.1 | Sim (security/CI) | instalado via `task setup` |
| cosign | v2.4.3 | Para assinar imagens | instalado via `task setup` |
| gitsign | v0.12.0 | Para assinar commits | instalado via `task setup` |
| ngrok | qualquer | Opcional (webhooks locais) | [ngrok.com/download](https://ngrok.com/download) |

---

## Stack

Componentes, versões e registros de imagem usados em produção.

| Componente | Versão / detalhe |
|---|---|
| Go | `1.26.4` |
| Router HTTP | `go-chi/chi v5.3.0` |
| Banco | PostgreSQL 16 (`postgres:16-alpine`) |
| Connection Pooler | pgBouncer (`edoburu/pgbouncer:v1.25.2-p0`, pool mode: transaction) |
| Observabilidade local | `grafana/otel-lgtm:0.7.5` |
| Proxy de produção | Caddy 2 |
| Automação | Task `3.51.1` |
| Registro de imagem | `ghcr.io/limateixeiratecnologia/mecontrola` |
| Supply chain | Trivy + cosign keyless + SBOM SPDX-JSON |

---

## Módulos e responsabilidades

Monolito modular com 9 bounded contexts em `internal/`. Cada módulo segue as camadas Domain → Application → Infrastructure e se registra no bootstrap via `module.go`. O módulo `platform` provê capacidades transversais (outbox, worker, canais de mensagem) consumidas pelos demais.

| Módulo | Responsabilidade |
|---|---|
| `internal/identity` | Usuários, principal/auth, entitlements, gateway HMAC-SHA256, housekeeping de `auth_events` |
| `internal/billing` | Webhook Kiwify, reconciliação de assinaturas, grace period PAST_DUE (3 dias), housekeeping de eventos |
| `internal/onboarding` | Magic token, ativação via WhatsApp/Telegram, outreach, expiração de tokens, limpeza de mensagens Meta |
| `internal/categories` | Catálogo de categorias, dicionário com busca HTTP e ETag cache |
| `internal/card` | CRUD de cartões, listagem paginada, fatura por competência, conformidade PCI RF-16 |
| `internal/budgets` | Orçamentos mensais, despesas, recorrência, resumo mensal, reaper/purge jobs |
| `internal/transactions` | Transações financeiras (DMMF/Decide\*), idempotência, resumo mensal, recorrência materializada |
| `internal/agent` | Integração LLM via OpenRouter; padrão canônico Workflow/Tool com WorkflowRegistry (intent kind → Workflow → Tool → binding → usecase); runtime Thread/Run auditável com métricas; circuit breaker; dispatch multicanal (WhatsApp/Telegram) |
| `internal/platform` | Outbox transacional, worker manager, WhatsApp Cloud API, Telegram Bot, idempotência, rate limit |

---

## Entrypoints

O binário expõe 4 subcomandos Cobra. `server` e `worker` rodam em paralelo; `migrate` é one-shot e sai após aplicar as mudanças.

```bash
mecontrola server          # HTTP server (Chi, porta configurada em PORT)
mecontrola worker          # Worker de background (outbox dispatcher, jobs agendados)
mecontrola migrate         # Aplica todas as migrations pendentes e sai
mecontrola migrate-down    # Reverte migrations (flag --steps N opcional)
```

---

## Configuração (.env)

Copie `.env.example` para `.env` e preencha os valores marcados com `CHANGE_ME_*`. Esses valores são rejeitados pelo `Config.Validate()` quando `ENVIRONMENT=production`. Em produção o arquivo fica em `chmod 600`, dono root, na raiz do repositório.

```bash
cp .env.example .env
```

### Aplicação

```env
ENVIRONMENT=local          # local | production
APP_MODE=server
```

### HTTP

```env
PORT=8080
SERVICE_NAME_API=mecontrola-api
SERVICE_NAME_WORKER=mecontrola-worker
CORS_ALLOWED_ORIGINS=http://localhost:3000,http://localhost:5173
AUTH_RATE_LIMIT_PER_USER_PER_MIN=120
AUTH_RATE_LIMIT_PER_USER_BURST=60
```

> Em `production`: lista explícita obrigatória. Wildcard `*` ou valor vazio causam erro de boot.

### Banco de dados

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

Stack local via `docker compose` sobe `grafana/otel-lgtm:0.7.5` em `localhost`. Em produção, apontar para Grafana Cloud ou instância dedicada.

```env
OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4317
OTEL_EXPORTER_OTLP_PROTOCOL=grpc
OTEL_EXPORTER_OTLP_INSECURE=true
OTEL_TRACE_SAMPLE_RATE=1.0
OTEL_SERVICE_VERSION=dev
LOG_LEVEL=debug
LOG_FORMAT=json
```

### Docker Compose local (otel-lgtm)

Variáveis do container `grafana/otel-lgtm` usado em dev local (`task local:infra` / `task local:up`):

```env
OTEL_LGTM_ADMIN_USER=admin
OTEL_LGTM_ADMIN_PASSWORD=admin@dev
```

### Stack de observabilidade completa (profile `observability`)

O serviço `grafana` (Grafana standalone) é ativado apenas com `--profile observability`. A variável abaixo tem default `admin@dev` para dev local, mas **deve ser definida explicitamente em produção**.

```env
GRAFANA_ADMIN_USER=admin
GRAFANA_ADMIN_PASSWORD=CHANGE_ME_use_strong_password
```

> Em produção: defina `GRAFANA_ADMIN_PASSWORD` com valor forte. O default `admin@dev` é aceito apenas em dev local.

### Outbox transacional (RF-26 / D-03)

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
BILLING_ANONYMIZATION_RETENTION_DAYS=365
BILLING_GRACE_EXPIRATION_SCHEDULE=@daily
```

### Budgets

```env
BUDGETS_PENDING_REAPER_INTERVAL=@every 30s
BUDGETS_PENDING_TTL_HOURS=24
BUDGETS_ABANDONED_DRAFT_CRON=0 3 * * *
BUDGETS_RETENTION_PURGE_CRON=0 4 1 * *
BUDGETS_RETENTION_PURGE_BATCH_SIZE=500
```

### Transactions

```env
TRANSACTIONS_ENABLED=false
TRANSACTIONS_IDEMPOTENCY_TTL=24h
TRANSACTIONS_MONTHLY_SUMMARY_DEBOUNCE_WINDOW=1500ms
TRANSACTIONS_RECURRING_MATERIALIZER_CRON=@daily
TRANSACTIONS_MONTHLY_SUMMARY_RECONCILER_CRON=@daily
TRANSACTIONS_BRAZIL_TIMEZONE=America/Sao_Paulo
```

### Onboarding

```env
ONBOARDING_TOKEN_TTL_DAYS=7
ONBOARDING_OUTREACH_GAP_HOURS=2
ONBOARDING_OUTREACH_ENABLED=false
ONBOARDING_CHECKOUT_RATE_LIMIT_PER_MIN=10
ONBOARDING_CHECKOUT_RATE_LIMIT_BURST=5
ONBOARDING_STATE_RATE_LIMIT_PER_MIN=30
ONBOARDING_TOKEN_ENCRYPTION_KEY=CHANGE_ME_32_byte_token_encryption_key
ONBOARDING_TOKEN_EXPIRATION_SCHEDULE=0 3 * * *
ONBOARDING_MAX_TOKEN_LOOKUP_ATTEMPTS=5
ONBOARDING_META_RETENTION_DAYS=30
ONBOARDING_META_CLEANUP_SCHEDULE=30 3 * * *
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

### Telegram

```env
TELEGRAM_ENABLED=false
TELEGRAM_BOT_TOKEN=CHANGE_ME_telegram_bot_token
TELEGRAM_BOT_ID=0
TELEGRAM_BOT_USERNAME=CHANGE_ME_bot_username
TELEGRAM_SECRET_TOKEN=CHANGE_ME_telegram_secret_token
TELEGRAM_SECRET_TOKEN_NEXT=
TELEGRAM_WEBHOOK_PATH=/api/v1/channels/telegram/webhook
TELEGRAM_WEBHOOK_RATE_LIMIT_PER_MIN=600
TELEGRAM_OUTBOUND_TIMEOUT=10s
```

### Agent / LLM (OpenRouter)

```env
OPENROUTER_API_KEY=CHANGE_ME_openrouter_api_key
AGENT_LLM_HTTP_REFERER=https://mecontrola.app
AGENT_LLM_PRIMARY_MODEL=google/gemini-2.5-flash-lite
AGENT_LLM_FALLBACK_MODELS=mistralai/mistral-small-3.2-24b-instruct
AGENT_LLM_MAX_TOKENS=256
AGENT_LLM_TEMPERATURE=0
AGENT_LLM_REQUEST_TIMEOUT=8s
AGENT_LLM_CIRCUIT_FAILURES=5
AGENT_LLM_CIRCUIT_WINDOW=30s
AGENT_LLM_CIRCUIT_COOLDOWN=60s
```

### Gateway Auth (HMAC-SHA256)

Autenticação interna entre o agent LLM e a API. O segredo deve ser gerado com `openssl rand -hex 32`. `NEXT` é opcional e usado durante rotação.

```env
IDENTITY_GATEWAY_SHARED_SECRET_CURRENT=CHANGE_ME_openssl_rand_hex_32
IDENTITY_GATEWAY_SHARED_SECRET_NEXT=
IDENTITY_GATEWAY_AUTH_WINDOW=60s
IDENTITY_AUTH_EVENTS_HOUSEKEEPING_SCHEDULE=@daily
IDENTITY_AUTH_EVENTS_HOUSEKEEPING_BATCH=500
IDENTITY_AUTH_EVENTS_RETENTION_DAYS=90
```

---

## Subir só a infra

Use quando quiser rodar o server/worker via `go run`, `task run` ou debug no VS Code — sem precisar dos containers da aplicação. Sobe PostgreSQL e Grafana LGTM (observabilidade) apenas.

```bash
task local:infra
```

Equivalente manual:

```bash
docker compose --env-file .env \
  -f deployment/compose/compose.yml \
  -f deployment/compose/compose.local.yml \
  up -d postgres otel-lgtm
```

Endpoints disponíveis após subir:

| Serviço | Endereço |
|---|---|
| PostgreSQL | `localhost:5432` |
| Grafana | `http://localhost:3000` (admin / admin@dev) |
| OTLP gRPC | `localhost:4317` |
| OTLP HTTP | `localhost:4318` |

---

## Subir tudo (infra + migrate + server + worker)

Sobe o ambiente completo em sequência determinística. Use no dia a dia quando não precisar de debug com breakpoints. O `migrate` roda como one-shot e sai; `server` e `worker` ficam em background.

```bash
task local:up
```

Sequência executada internamente:

1. `docker compose up -d postgres otel-lgtm` — aguarda healthcheck do postgres
2. `docker compose run --rm migrate` — aplica migrations pendentes e sai
3. `docker compose up -d server worker` — sobe e fica em background

Endpoints após subir:

| Serviço | Endereço |
|---|---|
| API | `http://localhost:8080` |
| Health | `http://localhost:8080/health` |
| Grafana | `http://localhost:3000` |

Outros comandos de gerenciamento do ambiente local:

```bash
task local:down       # para e remove containers (preserva volumes)
task local:destroy    # para + remove volumes (apaga dados) — pede confirmação
task local:logs       # tail de todos os containers (Ctrl+C para sair)
task local:ps         # status dos containers
```

---

## Debug no VS Code

O projeto vem com `.vscode/launch.json` configurado para debugar `server`, `worker` e `migrate` individualmente ou em conjunto. Não é necessário subir os containers da app — apenas a infra.

**Pré-requisitos:** extensão [Go for VS Code](https://marketplace.visualstudio.com/items?itemName=golang.go) instalada, `.env` preenchido, infra no ar.

```bash
task local:infra   # postgres + otel-lgtm
task migrate:up    # aplica migrations
# VS Code: F5 → selecionar configuração
```

Configurações disponíveis em `.vscode/launch.json`:

| Configuração | Comando | Quando usar |
|---|---|---|
| `migrate` | `cmd migrate` + `.env` | Aplicar migrations em debug |
| `server` | `cmd server` + `.env` | Debugar o HTTP server com breakpoints |
| `worker` | `cmd worker` + `.env` | Debugar jobs em background |
| `server + worker` | ambos simultâneos | Debugar fluxo completo; `stopAll: true` ao encerrar |

> Alternativa via CLI: `dlv debug ./cmd -- server`

---

## Comandos Task

O projeto usa [Task](https://taskfile.dev) `v3.51.1`. Execute `task --list-all` para ver todas as tasks disponíveis.

### Setup e inicialização

| Task | Objetivo | Quando executar |
|---|---|---|
| `task setup` | Instala pre-commit hooks, gitsign e configura assinatura de commits | Uma vez ao clonar |
| `task mocks:mocks` | Regenera mocks via mockery conforme `.mockery.yml` | Após alterar interfaces |
| `task mocks:clean` | Remove todos os mocks gerados | Antes de regenerar do zero |
| `task mocks:verify` | Falha se os mocks estiverem desatualizados (uso em CI) | — |

### Build

| Task | Objetivo |
|---|---|
| `task build:build` | Compila binário para o SO atual em `bin/mecontrola` |
| `task build:all` | Cross-compile linux/darwin/windows × amd64/arm64 em `bin/` |
| `task build:docker:build IMAGE_TAG=<tag>` | Build da imagem Docker multi-stage distroless (≤30 MB, USER 65532) |
| `task build:clean` | Remove `bin/` e `.task/` |
| `task run` | Compila e executa o server localmente — requer infra no ar |

### Desenvolvimento local

| Task | Objetivo |
|---|---|
| `task local:infra` | Sobe postgres + otel-lgtm sem aplicação |
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
| `task migrate:down` | Reverte todas as migrations |
| `task migrate:create -- <nome>` | Cria novo par de arquivos SQL numerado em `migrations/` |

### Testes

| Task | Objetivo | Depende de |
|---|---|---|
| `task test:all` | Unitários + integração | Docker (integração) |
| `task test:unit` | Unitários com `-race` e cobertura em `coverage/unit.out` | — |
| `task test:integration` | Integração com testcontainers | Docker disponível |
| `task test:coverage` | Relatório HTML em `coverage/coverage.html` | `test:unit` |
| `task test:coverage:identity` | Cobertura do módulo identity com validação de pontos críticos (RF-17) | `test:unit` |
| `task test:e2e` | Testes E2E BDD com Godog (agent + categories, requer Docker) | Docker disponível |
| `task test:watch` | Re-executa unitários ao salvar | — |
| `task card:test` | Unitários do módulo card com `-race` | — |
| `task card:integration` | Integração do módulo card | Docker disponível |

### Lint e qualidade

| Task | Objetivo |
|---|---|
| `task lint:run` | golangci-lint + gates: auth-bypass, outbox-user-id |
| `task lint:fix` | Aplica correções automáticas do linter |
| `task lint:fmt` | gofmt + goimports |
| `task lint:fmt:check` | Falha se arquivo não formatado (uso em CI) |
| `task lint:tidy` | `go mod tidy` + verifica drift em `go.mod`/`go.sum` |
| `task lint:pci` | Gate RF-16: bloqueia PAN/CVV/CVC/track/PIN em produção |
| `task lint:user-isolation` | Gate: UPDATE/DELETE sem `user_id` na WHERE em repos per-user |
| `task lint:auth-bypass` | Gate M-09: `RequireGatewayAuth` obrigatório antes de `InjectPrincipal` |
| `task lint:outbox-user-id` | Gate: `AggregateUserID` obrigatório em `EventInput` |
| `task lint:outbox-user-id:test` | Regressão do gate outbox-user-id com fixtures (missing field, empty, populated, allowlist) |
| `task card:lint` | golangci-lint escopo card (inclui regra forbidigo PCI) |
| `task card:audit` | Auditoria R0–R7: init, panic, clock, interface-assertion, zero-comentários, SQL em adapter, PCI |

### Validação rápida

| Task | Objetivo |
|---|---|
| `task check` | `lint:run` + `test:unit` + `security:vulncheck` — executar antes de abrir PR |
| `task ci:pipeline` | Pipeline CI completa (lint + testes + segurança + build) |
| `task ci:fast` | Subconjunto rápido para feedback em PR (lint + testes unitários) |

### Segurança

| Task | Objetivo | Requer |
|---|---|---|
| `task security:vulncheck` | govulncheck + trivy fs HIGH/CRITICAL | govulncheck, trivy |
| `task security:scan` | vulncheck + audit | govulncheck, trivy |
| `task security:audit` | `go list -m -u all` + `go mod verify` | — |
| `task security:image-scan IMAGE_SHA=<sha>` | Trivy na imagem do GHCR | trivy, acesso GHCR |
| `task security:sbom IMAGE_SHA=<sha>` | Gera `sbom.spdx.json` da imagem | trivy, acesso GHCR |
| `task security:sign-image IMAGE_REF=<ref> IMAGE_SHA=<sha>` | Assina imagem via cosign keyless + gera SBOM e provenance attestations | cosign, OIDC GitHub Actions |
| `task security:verify-image IMAGE_SHA=<sha>` | Verifica assinatura cosign keyless | cosign |
| `task security:vps:firewall VPS_HOST=<ip>` | Aplica regras ufw no VPS via SSH (22/80/443) — `--force-enable` ativa o ufw | SSH + sudo no VPS |

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
| `task deploy:local` | Deploy da máquina local direto na VPS, sem GHCR (build amd64 + `docker save\|load` + migrate + server/worker + healthcheck/rollback). Aceita `-- <tag>`. Ver [seção dedicada](#deploy-da-máquina-local-direto-na-vps-deploy-localsh) |

---

## Sequências comuns

Receitas prontas para os fluxos mais frequentes.

**Primeira vez (clone do zero):**

```bash
cp .env.example .env   # preencher CHANGE_ME_* e ajustar valores locais
task setup             # pre-commit + gitsign
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

> Para reset do banco de **produção**, ver a seção [Reset do banco de produção](#reset-do-banco-de-produção).

**Testar webhook com ngrok:**

```bash
task ngrok:server    # sobe ambiente completo + abre túnel
task ngrok:urls      # copia URLs → configurar no Meta/Kiwify Dashboard
# Ctrl+C para encerrar o túnel
task local:down      # para os containers
```

---

## Reset do banco de produção

Procedimento para apagar todos os dados e recriar o schema a partir das migrations. Usado quando o banco de produção precisa ser trazido ao estado inicial (novo banco limpo + seeds).

> ⚠️ **Operação destrutiva e irreversível.** Todos os dados de produção serão apagados permanentemente.

### Como funciona

O binário expõe `migrate-down --steps -1` (reverte todas via `migrator.Down()`) e `migrate` (reaplicar via `migrator.Up()`). Em produção, ambos são executados pelo serviço `migrate` do Compose, que conecta diretamente ao postgres (sem pgbouncer).

### Passo 1 — Execute na VPS via docker compose

```bash
ssh root@187.77.45.48
cd /opt/mecontrola

# Reverter TODAS as migrations (apaga dados e schema)
docker compose --env-file .env \
  -f deployment/compose/compose.yml \
  -f deployment/compose/compose.prod.yml \
  run --rm migrate migrate-down --steps -1

# Reaplicar TODAS as migrations (recria schema + seeds)
docker compose --env-file .env \
  -f deployment/compose/compose.yml \
  -f deployment/compose/compose.prod.yml \
  run --rm migrate migrate
```

Saída esperada: `migrations reverted (steps=-1)` seguido de `migrations applied`.

### Passo 2 — Verificar estado pós-reset

```bash
# Confirma versão mais alta e dirty=false
docker exec mecontrola-postgres-1 psql -U mecontrola -d mecontrola_db \
  -c "SELECT version, dirty FROM schema_migrations ORDER BY version;"

# Confirma seed: dicionário de categorias (≥544 entradas)
docker exec mecontrola-postgres-1 psql -U mecontrola -d mecontrola_db \
  -c "SELECT COUNT(*) FROM mecontrola.category_dictionary;"
```

Resultado esperado: versão `18`, `dirty = false`, contagem `544`.

### Via túnel SSH (alternativa local)

Se preferir executar localmente com o túnel ativo (`mecontrola-db` no `.zshrc`):

```bash
# Terminal 1 — abrir túnel
mecontrola-db   # ou: ssh -N -L 5433:172.18.0.2:5432 root@187.77.45.48

# Terminal 2 — reverter e reaplicar
DB_HOST=localhost DB_PORT=5433 \
  DB_USER=mecontrola DB_PASSWORD=<senha> DB_NAME=mecontrola_db DB_SSL_MODE=disable \
  go run ./cmd/... migrate-down --steps -1

DB_HOST=localhost DB_PORT=5433 \
  DB_USER=mecontrola DB_PASSWORD=<senha> DB_NAME=mecontrola_db DB_SSL_MODE=disable \
  go run ./cmd/... migrate
```

### Reset completo do banco local

```bash
task local:destroy   # remove volumes (pede confirmação)
task local:up        # recria schema + seeds do zero
```

---

## CI/CD

Dois workflows GitHub Actions independentes. CI valida qualidade e segurança a cada PR e na main; CD implanta na VPS somente após CI verde na main (ou via dispatch manual).

### CI (`.github/workflows/ci.yml`)

Ativado em `pull_request` (branches: main) e `push` (branch: main).

| Job | Quando | O que faz |
|---|---|---|
| `lint` | sempre | `lint:run` + `lint:fmt:check` + `lint:pci` |
| `unit` | sempre | `test:unit` + upload de artefato de cobertura |
| `integration` | sempre | `test:integration` com testcontainers |
| `security` | sempre | `security:vulncheck` (govulncheck + trivy fs) |
| `governance` | sempre | ai-spec doctor + lint, conventional commits, validação do Taskfile |
| `card-audit` | sempre | `card:audit` (gates R0–R7 + anti-PCI) |
| `coverage-comment` | apenas PR | Posta relatório de cobertura como comentário no PR |
| `build-image` | apenas main | Build + push da imagem para GHCR com tag = SHA curto |
| `scan-and-attest` | apenas main | Trivy image scan + SBOM SPDX-JSON + cosign sign + attestations |

### CD (`.github/workflows/cd.yml`)

Ativado automaticamente após CI verde na main, ou manualmente via `workflow_dispatch` com `image_tag` customizado.

```
Automático (workflow_run):
  gate (download image-meta do CI) → deploy VPS

Manual (workflow_dispatch com image_tag):
  deploy VPS
```

### Dependabot (`.github/workflows/auto-merge.yml`)

Dependabot atualiza semanalmente (gomod, github-actions, docker). PRs de minor/patch são aprovados e mergeados automaticamente via squash. PRs de major ficam abertos para revisão manual.

---

## Deploy da máquina local direto na VPS (`deploy-local.sh`)

Deploy de um único comando, **da sua máquina direto para a VPS, sem depender do GHCR nem da CI/CD**. Use quando a pipeline estiver indisponível ou quando precisar subir uma correção rápida gerando tudo localmente.

O script `deployment/scripts/deploy-local.sh` faz, em sequência:

1. **Build** da imagem `linux/amd64` localmente (arquitetura da VPS — a imagem não precisa casar com o Mac/arm64).
2. **Transferência** da imagem para a VPS via `docker save | ssh docker load` (sem `docker push`/GHCR).
3. **Sync** do repositório no host (`git pull --ff-only`, best-effort) + captura da imagem anterior para rollback.
4. **Migrations** (`compose run --rm migrate`) — aplicadas **antes** do app subir.
5. **server + worker** recriados com a nova tag + **healthcheck** com **rollback automático** para a imagem anterior se falhar.
6. **Alinhamento** do `IMAGE_TAG` no `.env` da VPS + **verificação pós-deploy** (`schema_migrations`, imagens em execução, HEAD do host).

### Pré-requisitos

| Requisito | Detalhe |
|---|---|
| Docker local | daemon ativo (build + `docker save`) |
| Acesso SSH por chave à VPS | sem senha (`BatchMode`); a chave padrão ou `VPS_SSH_KEY` |
| Árvore git limpa | a tag = short SHA do commit; suja é bloqueada (use `ALLOW_DIRTY=true` para burlar) |
| `.env` na VPS | já presente em `VPS_DEPLOY_PATH/.env` (o script não cria nem altera segredos, só o `IMAGE_TAG`) |

### Passo a passo

```bash
# 1. (recomendado) commit + push para manter a VPS em sync via git pull
git add -A && git commit -m "fix: ..." && git push

# 2. deploy — tag default = short SHA do HEAD
bash deployment/scripts/deploy-local.sh

# ou com uma tag explícita:
bash deployment/scripts/deploy-local.sh 1a2b3c4
```

Atalho via Task (equivalente):

```bash
task deploy:local              # tag = short SHA do HEAD
task deploy:local -- 1a2b3c4   # tag explícita
```

Saída esperada ao final (resumida):

```
[..] 1/5 build ghcr.io/limateixeiratecnologia/mecontrola:<tag>
[..] 2/5 transferindo imagem para a VPS (docker save | ssh docker load)
[..] 3/5 migrate + 4/5 server/worker + healthcheck (no host)
[vps] migrate
[vps] up server worker
[vps] healthy após 10s
[vps] === verificação pós-deploy ===
[vps] schema_migrations (version dirty): 16|f
[vps] mecontrola-server-1 ...:<tag> Up 5 seconds (healthy)
[vps] mecontrola-worker-1 ...:<tag> Up 5 seconds (healthy)
[vps] HEAD host: <tag>
[..] 5/5 deploy concluído — <tag> em produção e saudável
```

### Variáveis de override

Todas opcionais; defaults entre parênteses.

| Variável | Default | Uso |
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

> **Segurança:** o script aborta antes de tocar a VPS se a árvore git estiver suja (a tag não refletiria o commit) ou se o SSH falhar. Em falha de healthcheck, faz rollback automático para a imagem anterior. As migrations rodam **antes** do app; se falharem, o deploy aborta e os containers atuais permanecem intactos.

> **Quando usar a CI/CD em vez disto:** o caminho padrão de produção é a pipeline (build assinado por cosign + SBOM + scan Trivy). O `deploy-local.sh` é um atalho operacional para a VPS — ele **não** assina a imagem nem gera SBOM. Veja [CI/CD](#cicd) e o runbook `docs/runbooks/deploy-producao.md`.

---

## Contribuição

1. **Abra uma issue** antes de iniciar qualquer mudança de escopo maior para alinhar contexto e abordagem.
2. **Siga Conventional Commits** — o gate `governance` no CI rejeita commits que não seguem o padrão (`feat:`, `fix:`, `chore:`, etc.).
3. **Execute `task check`** antes de abrir PR — roda lint, testes unitários e vulncheck localmente.
4. **Execute `task setup`** ao clonar — instala pre-commit hooks e configura gitsign para assinatura de commits.
5. **Não flexibilize regras de arquitetura** — as regras em `AGENTS.md` são inegociáveis e verificadas automaticamente no CI.

---

## Governance

Referências canônicas para regras de arquitetura, ADRs e especificações de produto.

| Artefato | Localização | Conteúdo |
|---|---|---|
| Regras e skills | `AGENTS.md` | Fonte canônica de arquitetura, ADRs e regras obrigatórias |
| PRDs e techspecs | `.specs/` | Especificações por módulo |
| Diagramas C4 | `docs/diagrams/` | PlantUML por módulo (container + fluxos) |
| Coleção Postman | `docs/postman/` | Endpoints + environment |
