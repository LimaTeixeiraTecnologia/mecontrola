# MeControla

![Signed Image](https://img.shields.io/badge/image-signed%20cosign-brightgreen)
![SBOM Available](https://img.shields.io/badge/SBOM-SPDX--JSON-blue)
![Governance](https://img.shields.io/badge/governance-ai--spec-purple)

Agente financeiro conversacional via WhatsApp — monolito Go com arquitetura hexagonal.

## Stack

| Componente | Versão |
|---|---|
| Go | 1.26.3 |
| devkit-go | v0.4.0 |
| PostgreSQL | 16 (Alpine) |
| Deploy | Fly.io — região `gru` (São Paulo) |
| Observabilidade | Grafana Cloud (OTel OTLP) |
| Assinatura | cosign keyless + gitsign (Sigstore) |

## Comandos task

```sh
task setup               # Instala ferramentas, pre-commit hooks e gitsign
task build               # Compila o binário mecontrola
task test:unit           # Executa testes unitários com cobertura
task test:integration    # Executa testes de integração (requer Docker)
task ci                  # Pipeline completa: lint + unit + integration + security + build
task lint:run            # Executa golangci-lint
task security:vulncheck  # govulncheck + trivy fs
```

## Subcomandos mecontrola

```
mecontrola server    Sobe o servidor HTTP na porta 8080 + health endpoints
mecontrola worker    Sobe o runtime worker idle (jobs registrados em PRDs futuros)
mecontrola migrate   Aplica migrations pendentes do PostgreSQL e termina com exit 0
```

## Configuração

Copie `.env.example` para `.env` e preencha os valores:

```sh
cp .env.example .env
```

Consulte [`.env.example`](.env.example) para a lista completa de variáveis.
Variáveis obrigatórias em produção: `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`.

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
  events/          EventBus tipado via generics
  clock/           Abstração de tempo (testável)
  runtime/         Bootstrap de modos server/worker
```

## Segurança

Toda imagem publicada no GHCR é assinada com `cosign` keyless via GitHub OIDC.

Verificar assinatura:
```sh
cosign verify \
  --certificate-identity-regexp '^https://github\.com/LimaTeixeiraTecnologia/mecontrola/' \
  --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' \
  ghcr.io/limateixeiratecnologia/mecontrola:<sha>
```

Para reportar vulnerabilidades: consulte [SECURITY.md](SECURITY.md).

- [ADR-013: cosign + gitsign + disclosure](.specs/prd-mecontrola-foundation/adr-013-signing-attestation-disclosure.md)
- Sigstore: https://www.sigstore.dev/

## Runbooks Operacionais

| Runbook | Descrição |
|---|---|
| [deploy.md](docs/runbooks/deploy.md) | Deploy manual + pipeline CI/CD |
| [rollback.md](docs/runbooks/rollback.md) | Reverter para release anterior |
| [restore-pitr.md](docs/runbooks/restore-pitr.md) | Restore Fly Postgres via PITR |
| [rotate-secret.md](docs/runbooks/rotate-secret.md) | Rotacionar credenciais |
| [upgrade-ai-spec.md](docs/runbooks/upgrade-ai-spec.md) | Upgrade do harness ai-spec |
| [disclosure.md](docs/runbooks/disclosure.md) | Triage de CVE / responsible disclosure |
| [setup-gitsign.md](docs/runbooks/setup-gitsign.md) | Configurar gitsign para novo dev |
