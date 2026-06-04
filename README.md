# MeControla

[![CI](https://github.com/LimaTeixeiraTecnologia/mecontrola/actions/workflows/ci.yml/badge.svg)](https://github.com/LimaTeixeiraTecnologia/mecontrola/actions/workflows/ci.yml)
![Signed Image](https://img.shields.io/badge/image-signed%20cosign-brightgreen)
![SBOM Available](https://img.shields.io/badge/SBOM-SPDX--JSON-blue)
![Governance](https://img.shields.io/badge/governance-ai--spec-purple)

Agente financeiro conversacional via WhatsApp — monolito Go com arquitetura hexagonal.

## Stack

| Componente | Versão / Detalhe |
|---|---|
| Go | 1.26.4 |
| devkit-go | v0.4.0 |
| PostgreSQL | 16 (Alpine) |
| Reverse proxy | Caddy 2 (HTTPS automático Let's Encrypt) |
| Deploy | VPS Hostinger KVM 2 — Ubuntu 24.04 LTS |
| Observabilidade | Grafana Cloud (OTel OTLP + Loki) |
| Assinatura | cosign keyless + gitsign (Sigstore) |
| Registry | GHCR (`ghcr.io/limateixeiratecnologia/mecontrola`) |

---

## Ambiente local

### Pré-requisitos

| Ferramenta | Instalação |
|---|---|
| Docker Engine + Compose v2 | https://docs.docker.com/get-docker/ |
| Task `v3.51.1` | `brew install go-task` ou https://taskfile.dev |
| Go `1.26.4` | https://go.dev/dl/ |

### Configuração inicial

```sh
# 1. Clone e entre no repositório
git clone https://github.com/LimaTeixeiraTecnologia/mecontrola.git
cd mecontrola

# 2. Instale ferramentas, pre-commit hooks e gitsign
task setup

# 3. Copie o arquivo de variáveis de ambiente
cp .env.example .env
```

Abra `.env` e preencha pelo menos:

```env
DB_PASSWORD=senha_local_qualquer
```

Os outros valores já têm defaults funcionais para desenvolvimento local.

### Subir o ambiente

```sh
# Sobe postgres + server + worker e aplica migrations
task local:seed
```

A API fica disponível em `http://localhost:8080`.
O banco Postgres fica exposto em `localhost:5432` para ferramentas locais (DBeaver, psql).

### Endpoints de saúde

| Endpoint | Descrição |
|---|---|
| `GET /health` | Liveness — app está no ar |
| `GET /ready` | Readiness — DB conectado e pronto |

```sh
curl http://localhost:8080/health
curl http://localhost:8080/ready
```

### Outros comandos locais

```sh
task local:up     # Sobe containers sem aplicar migrations
task local:down   # Para containers (preserva dados nos volumes)
task local:logs   # Tail dos logs de todos os containers
task local:ps     # Status dos containers
task local:reset  # Para containers e apaga volumes (DESTRÓI dados locais)
```

### Testes

```sh
# Testes unitários (sem Docker, com race detector)
task test:unit

# Testes de integração (requer postgres via task local:up)
task test:integration

# Relatório de cobertura em HTML
task test:coverage

# Validação rápida pré-commit (lint + unit + vulncheck)
task check
```

### Lint e formatação

```sh
task lint:run        # golangci-lint
task lint:fmt:check  # verifica formatação gofmt/goimports
```

---

## Desenvolvimento

### Pre-commit hooks

Após `task setup`, os seguintes hooks rodam automaticamente a cada commit:

| Hook | O que faz |
|---|---|
| `gofmt` / `goimports` | Formata o código Go |
| `golangci-lint --fast-only` | Lint estático rápido |
| `ai-spec lint` | Valida governança do repositório |
| `conventional-commits` | Valida formato da mensagem de commit |
| `detect-private-key` | Bloqueia commit de chaves privadas |
| `check-added-large-files` | Bloqueia arquivos >1 MB |

### Mensagens de commit

O projeto segue **Conventional Commits**:

```
feat(billing): adicionar cálculo de imposto sobre transação
fix(identity): corrigir validação de e-mail com subdomínio
refactor(platform): extrair helper de retry para devkit-go
docs(readme): atualizar instruções de deploy VPS
test(finance): adicionar caso de borda para valor negativo
chore(deps): atualizar devkit-go para v0.4.1
```

### Geração de mocks

```sh
task mocks   # regenera via mockery (configurado em .mockery.yaml)
```

### Pipeline completa local

```sh
task ci   # lint + fmt + unit + integration + vulncheck + build
```

Equivalente ao que roda no GitHub Actions antes do merge.

---

## Build e imagem Docker

### Binário local

```sh
task build          # compila bin/mecontrola para o SO atual
task build:build    # mesmo que acima (target direto)
```

### Cross-compile

```sh
task build:all   # linux/darwin/windows × amd64/arm64
```

### Imagem Docker

```sh
# Build local da imagem multi-stage (alpine builder + distroless runtime)
SHA=$(git rev-parse --short HEAD)
task build:docker:build IMAGE_TAG=${SHA}

# Scan de vulnerabilidades na imagem
task security:image-scan IMAGE_SHA=${SHA}

# Gerar SBOM (spdx-json)
task security:sbom IMAGE_SHA=${SHA}
```

A imagem final é `≤30 MB`, roda como UID `65532` (nonroot) e não contém shell.

---

## Deploy em produção (VPS Hostinger)

### Visão geral do fluxo

```
git push → CI (lint + test + vulncheck) → CD (build + trivy + cosign + push GHCR)
  → GitHub Actions workflow_dispatch  →  deploy.sh na VPS  →  smoke test /health
```

### Pré-requisitos na VPS

1. **VPS Hostinger KVM 2** com template Ubuntu 24.04 LTS.
2. **Docker Engine + Compose v2** instalados (template Docker da Hostinger já inclui).
3. **Usuário `deploy`** com acesso ao grupo `docker`; root SSH desabilitado.
4. **Repo clonado** em `/opt/mecontrola`:
   ```sh
   git clone https://github.com/LimaTeixeiraTecnologia/mecontrola.git /opt/mecontrola
   ```
5. **DNS** do domínio apontando para o IP da VPS (necessário para emissão TLS automática do Caddy).

### Primeiro deploy — passo a passo

**Na VPS**, como usuário `deploy`:

```sh
cd /opt/mecontrola

# 1. Copie e edite o .env de produção
cp .env.example .env
chmod 600 .env

# Variáveis obrigatórias (substitua os CHANGE_ME_*):
#   APP_DOMAIN     — domínio do serviço (ex: app.mecontrola.com.br)
#   CADDY_EMAIL    — e-mail para Let's Encrypt
#   DB_PASSWORD    — senha forte do postgres
#   ENVIRONMENT    — production
# Opcionais mas recomendados:
#   OTEL_EXPORTER_OTLP_ENDPOINT / OTEL_EXPORTER_OTLP_HEADERS — Grafana Cloud OTLP
#   LOKI_URL / LOKI_USER_ID / LOKI_API_KEY — logs no Grafana Cloud Loki

# 2. Suba todos os serviços (postgres, server, worker, caddy)
#    Caddy emite o certificado TLS automaticamente na primeira vez
IMAGE_TAG=<sha-ou-semver> docker compose \
  -f deployment/compose/compose.yml \
  -f deployment/compose/compose.prod.yml \
  up -d

# 3. Verifique que tudo subiu
docker compose \
  -f deployment/compose/compose.yml \
  -f deployment/compose/compose.prod.yml \
  ps

# 4. Smoke test
curl -s https://${APP_DOMAIN}/health
curl -s https://${APP_DOMAIN}/ready
```

### Deploy de nova versão

**Via GitHub Actions (recomendado):**

1. Acesse `Actions → CD → Run workflow`.
2. Informe o `IMAGE_TAG` (short-SHA ou semver, ex: `abc12345` ou `v1.2.0`).
3. Clique em **Run workflow**.

O workflow executa automaticamente:
- Pull da nova imagem no GHCR
- `docker compose run --rm migrate` (migrations)
- `docker compose up -d --no-deps server worker`
- Poll em `/health` por 60s; em falha reinicia com a imagem anterior

**Manual (emergência, via SSH):**

```sh
# Localmente, com chave SSH configurada
VPS_HOST=<ip-da-vps> VPS_USER=deploy \
  bash deployment/scripts/deploy.sh <image-tag>
```

### Secrets necessários no GitHub

Configure em `Settings → Secrets and variables → Actions`:

| Secret | Valor |
|---|---|
| `VPS_HOST` | IP ou hostname da VPS |
| `VPS_USER` | Usuário SSH (ex: `deploy`) |
| `VPS_SSH_KEY` | Chave SSH privada ed25519 do usuário deploy |
| `VPS_DEPLOY_PATH` | Caminho do repo na VPS (ex: `/opt/mecontrola`) |

### Ativar observabilidade (Loki + Prometheus)

Preencha no `.env` da VPS:

```env
LOKI_URL=https://logs-prod-xxx.grafana.net/loki/api/v1/push
LOKI_USER_ID=<user-id>
LOKI_API_KEY=<api-key>
```

Suba adicionando o profile `observability`:

```sh
IMAGE_TAG=<sha> docker compose \
  -f deployment/compose/compose.yml \
  -f deployment/compose/compose.prod.yml \
  --profile observability \
  up -d
```

O Promtail coleta logs dos containers Docker e envia para Grafana Cloud Loki.

### Backup automático

Configure o cron na VPS (como root):

```sh
# Edite /etc/pg-dump.env com as variáveis obrigatórias:
#   POSTGRES_CONTAINER, BACKUP_REMOTE, AGE_RECIPIENT

# Adicione ao crontab root:
crontab -e
# Linha a adicionar:
0 3 * * * /opt/mecontrola/deployment/scripts/pg-dump.sh >> /var/log/pg-dump.log 2>&1
```

O script realiza: `pg_dump` → gzip → `age` encrypt → upload para B2/R2 via `rclone` → retenção 30 dias.

Para gerar a chave `age`:

```sh
age-keygen -o /root/age-key.txt   # chave privada fora do servidor
# copie o recipient (chave pública) para AGE_RECIPIENT no /etc/pg-dump.env
```

### Rollback

```sh
# Via deploy.sh com a tag anterior
VPS_HOST=<ip> VPS_USER=deploy \
  bash deployment/scripts/deploy.sh <tag-anterior>
```

O script detecta falha de healthcheck e faz o rollback automaticamente.
Veja detalhes em [rollback.md](deployment/runbooks/rollback.md).

---

## Subcomandos mecontrola

```
mecontrola server    Inicia o servidor HTTP na porta 8080 + health endpoints
mecontrola worker    Inicia o worker de módulos em background
mecontrola migrate   Aplica migrations pendentes do PostgreSQL e termina (exit 0)
```

---

## Arquitetura

O projeto segue **SDD (Spec-Driven Development)** — toda funcionalidade começa com um PRD e uma especificação técnica antes da implementação.

- PRD: [`.specs/prd-mecontrola-foundation/prd.md`](.specs/prd-mecontrola-foundation/prd.md)
- Especificação técnica: [`.specs/prd-mecontrola-foundation/techspec.md`](.specs/prd-mecontrola-foundation/techspec.md)
- ADRs: [`.specs/prd-mecontrola-foundation/`](.specs/prd-mecontrola-foundation/) (ADR-001 a ADR-015)

### Módulos de domínio

```
internal/
  identity/        Identidade e autenticação
  conversation/    Conversas e sessões WhatsApp
  agent/           Agente LLM conversacional
  finance/         Transações e categorização financeira
  notifications/   Notificações e alertas
  telemetry/       Métricas e eventos de negócio
```

### Infraestrutura

```
internal/platform/
  database/        Manager + UnitOfWork[T] + migrations embed
  http/            Servidor Chi + middlewares + health endpoints
  observability/   OTel traces/metrics/logs + redaction PII
  worker/          WorkerManager + JobAdapter + ConsumerAdapter + Registry agnóstico
    job/           Scheduler via robfig/cron/v3 + OverlapPolicy (Skip|Allow)
    consumer/      Registry, Runner, Adapter e subpacote database (outbox/banco)
```

### Diagrama de infraestrutura (produção)

```
Internet ──TLS──▶ Caddy 2 :80/:443 ──http──▶ server:8080
                                                  │ pgx
                                          postgres:16 (volume isolado)
                                                  ▲
                                          mecontrola-worker

Promtail ──▶ Grafana Cloud Loki
node_exporter ──▶ Grafana Cloud Prometheus
```

---

## Segurança

Toda imagem publicada no GHCR é assinada com `cosign` keyless via GitHub OIDC e acompanhada de SBOM (SPDX-JSON) e atestado de provenance (SLSA).

### Verificar assinatura

```sh
cosign verify \
  --certificate-identity-regexp '^https://github\.com/LimaTeixeiraTecnologia/mecontrola/' \
  --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' \
  ghcr.io/limateixeiratecnologia/mecontrola:<sha>
```

### Verificar SBOM

```sh
# Listar atestados disponíveis
cosign verify-attestation \
  --certificate-identity-regexp '^https://github\.com/LimaTeixeiraTecnologia/mecontrola/' \
  --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' \
  --type spdxjson \
  ghcr.io/limateixeiratecnologia/mecontrola:<sha> \
  | jq '.payload | @base64d | fromjson'
```

### Scan de segurança local

```sh
task security:vulncheck   # govulncheck + trivy fs (sem Docker)
task security:scan        # vulncheck + audit de módulos
```

Para reportar vulnerabilidades: consulte [SECURITY.md](SECURITY.md).

- [ADR-013: cosign + gitsign + disclosure](.specs/prd-mecontrola-foundation/adr-013-signing-attestation-disclosure.md)
- Sigstore: https://www.sigstore.dev/

---

## Runbooks Operacionais

| Runbook | Quando usar |
|---|---|
| [deploy.md](deployment/runbooks/deploy.md) | Deploy manual ou emergencial |
| [rollback.md](deployment/runbooks/rollback.md) | Reverter para release anterior |
| [restore-pitr.md](deployment/runbooks/restore-pitr.md) | Restore do banco via backup |
| [rotate-secret.md](deployment/runbooks/rotate-secret.md) | Rotacionar credenciais (trimestral ou incidente) |
| [upgrade-ai-spec.md](deployment/runbooks/upgrade-ai-spec.md) | Upgrade do harness ai-spec |
| [disclosure.md](deployment/runbooks/disclosure.md) | Triage de CVE / responsible disclosure |
| [setup-gitsign.md](deployment/runbooks/setup-gitsign.md) | Configurar gitsign para novo desenvolvedor |
