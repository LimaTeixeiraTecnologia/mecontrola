# Análise: Deployment, Ambientes, O11y — Limpeza MVP Production-Ready

**Data:** 2026-06-15
**Foco:** MVP robusto, production-proof, pronto para produção, sem falso positivo
**Plataforma de produção confirmada:** VPS Hostinger (SSH + Docker Compose)

---

## Estado Atual do Working Tree

O repositório passou por uma refatoração de CI/CD onde o pipeline unificado `ci.yml` (chamado
"CI/CD") foi separado em dois arquivos distintos. As mudanças estão no working tree mas **não
commitadas** — isso é o primeiro bloqueante de produção.

| Arquivo | Estado git | Descrição |
|---------|-----------|-----------|
| `.github/workflows/ci.yml` | Modificado (unstaged) | "CI/CD" → "CI"; removeu `workflow_dispatch` e `if != workflow_dispatch` |
| `.github/workflows/cd.yml` | Untracked (novo) | Pipeline CD completo: gate → deploy VPS SSH → smoke |
| `.env.example` | Staged (index) | Mudanças pendentes de commit |
| `docs/` (inteiro) | Untracked (novo) | runbooks, diagrams, postman, prompts |

**Risco imediato:** sem commitar `cd.yml`, o deploy automatizado **não existe no repositório**.
Qualquer clone fresh do repo perde o pipeline de CD por completo.

---

## Mapa Completo de Deployment

### Plataforma definitiva: VPS Hostinger

```
push main
  → CI (ci.yml): lint + unit + integration + security + governance + card-audit + build-image + scan-and-attest
  → CD (cd.yml): gate (valida CI passed) → deploy VPS SSH → smoke (auth HMAC-SHA256)
```

**Script de deploy:** `deployment/scripts/deploy.sh`
- `docker compose pull server worker`
- `docker compose run --rm migrate`
- `docker compose up -d --no-deps server worker`
- Healthcheck `/health` com retry 12× (interval 5s)
- Rollback automático se healthcheck falhar

**Imagem:** `ghcr.io/limateixeiratecnologia/mecontrola:<sha-curto>`
**Registry:** GitHub Container Registry (GHCR)
**Runtime:** `gcr.io/distroless/static-debian12:nonroot` (user 65532, no shell)

### Ambiente local

```
task local:up → postgres + otel-lgtm + migrate + server + worker
```

- Compose override: `deployment/compose/compose.local.yml`
- O11y: `grafana/otel-lgtm:0.7.5` na porta 127.0.0.1:3000 (Grafana), 4317 (OTLP gRPC)
- Postgres exposto em `0.0.0.0:5432` (para ferramentas locais)

---

## Observabilidade (Stack Saudável)

A stack de o11y está production-ready. Nenhuma remoção necessária.

| Componente | Local | Produção | Status |
|------------|-------|----------|--------|
| Traces (OTLP gRPC) | otel-lgtm:4317 | Configurável via `OTEL_EXPORTER_OTLP_ENDPOINT` | Ativo em todos módulos |
| Métricas Prometheus | otel-lgtm → Prometheus | Endpoint `/metrics` + OTLP export | 41+ arquivos com métricas |
| Logs (slog + bridge) | JSON → otel-lgtm Loki | JSON → Promtail → Grafana Cloud Loki | 82+ arquivos |
| Dashboard Grafana | `deployment/grafana/mecontrola-platform.json` | Idem | Commitado, production-ready |
| Health checks | `/health` (liveness) + `/ready` (readiness + db.Ping) | Idem | Fly.io health check apontava certo |
| Promtail (log shipper) | Profile `observability` (opcional) | Ativo (sem profile) | config em `deployment/promtail/` |

**Lacunas documentadas (não bloqueantes para MVP):**
- Sem alertas Prometheus (rules/*.yml)
- Sem SLO/SLI definitions
- `internal/transactions/infrastructure/observability/` vazio (o módulo usa o11y via handlers)

---

## Artefatos Confirmados Mortos (remoção sem falso positivo)

### 1. `internal/platform/tenancy/` — REMOVER

**Evidência:**
- Diretório completamente vazio (0 arquivos)
- `grep -r "platform/tenancy" internal/` → zero resultados
- Nenhuma import em todo o codebase

**Ação:** `git rm -rf internal/platform/tenancy/`

---

### 2. `deployment/fly/fly.toml` — REMOVER (Fly.io confirmado abandonado)

**Evidência:**
- CD pipeline (`cd.yml`) faz deploy exclusivamente via SSH na VPS Hostinger
- Nenhum job em `ci.yml` ou `cd.yml` invoca `flyctl`
- `.env.example` documenta "Em producao (VPS Hostinger)"
- Usuário confirmou: VPS Hostinger é a plataforma definitiva

**Stale references a limpar junto:**
- `deployment/runbooks/deploy.md`: contém `flyctl deploy`, `mecontrola.fly.dev`, `flyctl status/logs`
- Referência `ADR-011 (Docker + Fly)` no runbook → atualizar para `ADR-011 (Docker + VPS)`

**Ação:** `git rm -r deployment/fly/` + reescrever `deployment/runbooks/deploy.md`

---

### 3. `coverage.out` e `coverage_vo.out` — REMOVER

**Evidência:**
- Ambos em `.gitignore` (`coverage.out` e `*.out`)
- `git ls-files --error-unmatch coverage.out` → NOT_TRACKED (não estão no índice git)
- Existem apenas no disco como resíduo de execução de testes locais

**Ação:** `rm coverage.out coverage_vo.out`

---

### 4. `docs/prompts/` — NÃO COMMITAR

**Evidência:**
- Artefatos de auditoria auto-gerados (2026-06-15-auditoria-*.md, 2026-06-15-production-readiness-audit.md)
- Não têm valor permanente como documentação viva
- `docs/` inteiro é untracked — controle granular possível no commit

**Ação:** Commitar `docs/runbooks/`, `docs/diagrams/`, `docs/postman/`, mas não `docs/prompts/` (exceto este arquivo)

---

## O Que NÃO Remover (sem falso positivo)

| Item | Por que manter |
|------|---------------|
| `internal/platform/ratelimit/` | Usado por `internal/card/module.go` (3 referências). Não é código morto — é isolado, mas ativo. |
| `internal/platform/channels/` | Usado por 4 arquivos (`cmd/server/`, `platform/telegram/`, `platform/whatsapp/`). |
| `docs/runbooks/` (8 módulos) | Documentação viva de domínio: agent, billing, budgets, card, categories, identity, onboarding, transactions |
| `docs/diagrams/` | Diagrama C4 container — referência arquitetural |
| `docs/postman/` | Coleção Postman — ferramenta de teste da API |
| `deployment/runbooks/` | Ops runbooks: deploy, rollback, restore-pitr, rotate-secret, setup-gitsign, disclosure |
| `deployment/compose/compose.prod.yml` | Override de produção com security hardening (read_only, cap_drop ALL, nonroot) |

---

## Plano de Execução (ordem obrigatória)

### Fase 1 — Commitar CI/CD (bloqueante)

```bash
# Verificar diff antes de commitar
git diff .github/workflows/ci.yml
git diff --cached .env.example

# Stage ci.yml
git add .github/workflows/ci.yml

# Commitar CI + CD juntos
git add .github/workflows/ci.yml .github/workflows/cd.yml .env.example
git commit -m "ci: separa ci/cd em workflows independentes, remove workflow_dispatch do ci"
```

### Fase 2 — Remover artefatos mortos

```bash
# Tenancy (dir vazio)
git rm -rf internal/platform/tenancy/

# Fly.io (plataforma abandonada)
git rm -r deployment/fly/

# Artefatos locais (não trackeados, só disco)
rm coverage.out coverage_vo.out
```

### Fase 3 — Atualizar runbook de deploy

Reescrever `deployment/runbooks/deploy.md` substituindo o fluxo `flyctl` pelo fluxo VPS:

```
CI (automático) → CD automático (workflow_run trigger) → deploy.sh VPS SSH
```

Seções a manter: build, push, scan, SBOM, sign (cosign keyless), verificar assinatura.
Seções a substituir: "Deploy no Fly.io" → "Deploy na VPS (automático via CD)".
Monitoramento: substituir `flyctl logs` por `ssh VPS docker compose logs`.

### Fase 4 — Commitar docs/ (seletivo)

```bash
# Commitar documentação permanente
git add docs/runbooks/ docs/diagrams/ docs/postman/
# NÃO adicionar docs/prompts/ (exceto este arquivo se desejado)
git commit -m "docs: adiciona runbooks de domínio, diagrama C4 e coleção Postman"
```

### Fase 5 — Commitar remoções + runbook

```bash
git add deployment/runbooks/deploy.md
git commit -m "chore: remove fly.toml (VPS Hostinger é prod definitivo), atualiza runbook deploy"
```

---

## Verificação de Produção (pós-execução)

```bash
# 1. Build não quebrou
task build:build

# 2. Lint passa (zero comentários, sem tenancy)
task lint:run

# 3. Testes unitários passam
task test:unit

# 4. Fly.io removido
ls deployment/fly/ 2>/dev/null && echo "FALHA: fly.toml ainda existe" || echo "OK"

# 5. Tenancy removido
ls internal/platform/tenancy/ 2>/dev/null && echo "FALHA: tenancy ainda existe" || echo "OK"

# 6. coverage.out limpos
ls coverage*.out 2>/dev/null && echo "PENDENTE: remover manualmente" || echo "OK"

# 7. cd.yml existe e é parseable
cat .github/workflows/cd.yml | python3 -c "import sys,yaml; yaml.safe_load(sys.stdin)" && echo "OK"
```

---

## Resumo Executivo

| Item | Ação | Risco |
|------|------|-------|
| Commitar `ci.yml` + `cd.yml` + `.env.example` | `git add + commit` | Baixo — mudanças já validadas |
| Remover `internal/platform/tenancy/` | `git rm -rf` | Zero — dir vazio, zero refs |
| Remover `deployment/fly/fly.toml` | `git rm -r` | Zero — Fly.io confirmado abandonado |
| Remover `coverage.out`, `coverage_vo.out` | `rm` local | Zero — não trackeados |
| Atualizar `deployment/runbooks/deploy.md` | Reescrever seção deploy | Baixo — documentação |
| Commitar `docs/runbooks/`, `docs/diagrams/`, `docs/postman/` | `git add` seletivo | Baixo — nova documentação |
| **NÃO remover** `platform/ratelimit/`, `platform/channels/` | — | Evita falso positivo |
