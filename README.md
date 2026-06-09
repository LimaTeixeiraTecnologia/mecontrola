# MeControla

[![CI](https://github.com/LimaTeixeiraTecnologia/mecontrola/actions/workflows/ci.yml/badge.svg)](https://github.com/LimaTeixeiraTecnologia/mecontrola/actions/workflows/ci.yml)
![Signed Image](https://img.shields.io/badge/image-signed%20cosign-brightgreen)
![SBOM Available](https://img.shields.io/badge/SBOM-SPDX--JSON-blue)
![Governance](https://img.shields.io/badge/governance-ai--spec-purple)

Monolito modular em Go para fluxos financeiros conversacionais via WhatsApp, com bootstrap separado para HTTP server, worker e migrações.

## Estado atual do projeto

- **Arquitetura:** monolito modular com bounded contexts em `internal/`
- **Módulos ativos no bootstrap:** `identity`, `billing`, `onboarding` e `platform`
- **Entrypoints principais:** `cmd/server`, `cmd/worker` e `cmd/migrate`
- **Mensageria interna:** outbox transacional + dispatcher no worker
- **HTTP inbound:** Chi
- **Deploy alvo:** VPS com Docker Compose, GHCR, cosign e Caddy

## Stack

| Componente | Versão / detalhe atual |
|---|---|
| Go | `1.26.4` |
| devkit-go | `v0.5.0` |
| Router HTTP | `github.com/go-chi/chi/v5 v5.3.0` |
| CLI | Cobra |
| Banco | PostgreSQL 16 (`postgres:16-alpine`) |
| Observabilidade local | `grafana/otel-lgtm:0.7.5` |
| Proxy de produção | Caddy 2 |
| Automação local | Task `3.51.1` |
| Registro de imagem | `ghcr.io/limateixeiratecnologia/mecontrola` |
| Supply chain | Trivy + cosign + SBOM SPDX-JSON |

## Módulos e responsabilidades

| Módulo | Responsabilidade atual |
|---|---|
| `internal/identity` | Usuários, principal/auth, projeções de assinatura, webhook WhatsApp inbound e housekeeping de `auth_events` |
| `internal/billing` | Webhook Kiwify, reconciliação de assinaturas, grace period, housekeeping de eventos e publicação de eventos de assinatura |
| `internal/onboarding` | Checkout/magic token, ativação via WhatsApp, outreach, expiração de tokens e limpeza de mensagens Meta processadas |
| `internal/platform` | Eventos, outbox, worker manager, HTTP client compartilhado, integrações transversais |

## Rotas HTTP atuais

| Método | Rota | Origem |
|---|---|---|
| `GET` | `/health` | health endpoint do servidor |
| `GET` | `/ready` | readiness com checagem de banco |
| `POST` | `/api/v1/identity/users/` | identity |
| `GET` | `/api/v1/whatsapp/verify` | webhook Meta |
| `POST` | `/api/v1/whatsapp/inbound` | webhook Meta inbound |
| `POST` | `/api/v1/billing/webhooks/kiwify` | webhook Kiwify |
| `POST` | `/api/v1/onboarding/checkout` | onboarding |
| `GET` | `/api/v1/onboarding/tokens/{token}/state` | onboarding |

## Estrutura do repositório

```text
cmd/
  main.go
  server/
  worker/
  migrate/
configs/
deployment/
  compose/
  docker/
  scripts/
internal/
  billing/
  identity/
  onboarding/
  platform/
migrations/
taskfiles/
```

## Ambiente local

### Pré-requisitos

| Ferramenta | Obrigatório |
|---|---|
| Docker Engine + Compose v2 | sim |
| Task `3.51.1` | sim |
| Go `1.26.4` | sim |
| `golangci-lint` | para `task lint:run` |
| `govulncheck` + `trivy` | para `task security:vulncheck` |

### Setup

```sh
git clone https://github.com/LimaTeixeiraTecnologia/mecontrola.git
cd mecontrola
cp .env.example .env
task setup
```

### Variáveis mínimas para bootstrap local

Hoje o bootstrap de `server` e `worker` instancia `billing` e `onboarding` logo na subida. Por isso, além do banco, o `.env` precisa ter valores para os pontos abaixo.

Use valores de desenvolvimento, por exemplo:

```env
DB_PASSWORD=mecontrola_local_password

ONBOARDING_TOKEN_ENCRYPTION_KEY=0123456789abcdef0123456789abcdef
META_PHONE_NUMBER_ID=local-phone-number-id
META_ACCESS_TOKEN=local-meta-access-token

KIWIFY_PRODUCT_ID_MONTHLY=local-monthly
KIWIFY_PRODUCT_ID_QUARTERLY=local-quarterly
KIWIFY_PRODUCT_ID_ANNUAL=local-annual

# Recomendado se voce for testar webhooks assinados localmente
META_APP_SECRET=local-meta-app-secret
KIWIFY_WEBHOOK_SECRET=local-kiwify-webhook-secret
```

Se quiser exercitar fluxos reais, complete tambem as credenciais de Meta Cloud API e Kiwify no `.env`.

### Subir o ambiente

```sh
task local:seed
```

Isso sobe:

- `postgres`
- `server`
- `worker`
- `otel-lgtm`

E depois executa `migrate`.

### Endpoints locais

| Serviço | Endereço |
|---|---|
| API | `http://localhost:8080` |
| Health | `http://localhost:8080/health` |
| Ready | `http://localhost:8080/ready` |
| Grafana local | `http://localhost:3000` |
| OTLP gRPC | `localhost:4317` |
| OTLP HTTP | `localhost:4318` |
| PostgreSQL | `localhost:5432` |

Credenciais padrão do LGTM local:

```text
admin / admin@dev
```

### Comandos locais úteis

```sh
task local:up
task local:down
task local:logs
task local:ps
task local:reset
task --list-all
```

## CLI da aplicação

```sh
go run ./cmd --help
```

Subcomandos atuais:

```text
mecontrola server
mecontrola worker
mecontrola migrate
mecontrola migrate-down --steps 1
```

## Desenvolvimento

### Rodar localmente fora do Docker Compose

```sh
task build
./bin/mecontrola server
./bin/mecontrola worker
./bin/mecontrola migrate
```

### Testes

```sh
task test:unit
task test:integration
task test:coverage
task test:coverage:identity
```

### Lint, segurança e validação rápida

```sh
task lint:run
task lint:fmt:check
task security:vulncheck
task check
```

### Mocks e benchmarks

```sh
task mocks
task bench:outbox
```

### Smokes disponíveis

```sh
task auth:smoke
task onboarding:smoke
```

## Worker atual

O `cmd/worker` monta um `worker.Manager` com jobs e handlers dos módulos. Hoje ele inclui:

- dispatcher, reaper e housekeeping do outbox
- housekeeping de `auth_events`
- reconciliação de billing
- housekeeping de eventos Kiwify
- expiração de grace period
- outreach de onboarding
- expiração de tokens de onboarding
- limpeza de mensagens Meta já processadas

## Build e imagem Docker

### Binário

```sh
task build
task build:all
```

### Imagem

```sh
SHA=$(git rev-parse --short HEAD)
task build:docker:build IMAGE_TAG=${SHA}
task security:image-scan IMAGE_SHA=${SHA}
task security:sbom IMAGE_SHA=${SHA}
```

## CI/CD atual

### CI

O workflow `.github/workflows/ci.yml` executa:

- lint
- formatação
- testes unitários
- testes de integração
- `govulncheck` + `trivy fs`
- governança (`ai-spec`, conventional commits, validação do Taskfile)
- `auth:smoke` em `main`

### CD

O workflow `.github/workflows/cd.yml` faz:

1. build e push da imagem para GHCR
2. scan Trivy da imagem
3. geração de SBOM
4. assinatura e attestations com cosign
5. deploy para VPS via `deployment/scripts/deploy.sh`

Em `workflow_dispatch`, o deploy aceita `image_tag` explícita.

## Deploy em produção

Os arquivos de compose atuais são:

- `deployment/compose/compose.yml`
- `deployment/compose/compose.prod.yml`

O script operacional atual é:

```sh
bash deployment/scripts/deploy.sh <image-tag>
```

Fluxo resumido do deploy:

```text
git push / workflow_dispatch
  -> build e push GHCR
  -> trivy image
  -> sbom
  -> cosign sign + attest
  -> SSH na VPS
  -> docker compose pull
  -> migrate
  -> up -d server worker
  -> smoke em /health
```

Em produção, `server` e `worker` rodam com:

- `read_only: true`
- `tmpfs` para `/tmp`
- `cap_drop: [ALL]`
- `no-new-privileges`
- `user 65532:65532`

## Segurança

### Verificar assinatura da imagem

```sh
cosign verify \
  --certificate-identity-regexp '^https://github\.com/LimaTeixeiraTecnologia/mecontrola/' \
  --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' \
  ghcr.io/limateixeiratecnologia/mecontrola:<sha>
```

### Scan local

```sh
task security:vulncheck
task security:scan
```

Para reporte de vulnerabilidades, consulte [SECURITY.md](SECURITY.md).

## Governança

As regras operacionais do repositório estão em:

- [AGENTS.md](AGENTS.md)
- [CLAUDE.md](CLAUDE.md)
- [GEMINI.md](GEMINI.md)

As automações de desenvolvimento usam:

- [Taskfile.yml](Taskfile.yml)
- [taskfiles/](taskfiles/)
- [migrations/](migrations/)
- [deployment/](deployment/)
