# Tarefa 10.0: CI/CD workflows + `cmd_integration_test` + coverage PR comment + runbooks + README + branch protection

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Materializar o **pipeline completo** GitHub Actions (CI + CD + auto-merge) consumindo o Taskfile (RF-06), executar **`cmd_integration_test.go`** que compila o binário e valida os 3 subcomandos contra testcontainers (RF-18 + CS-21 + CS-22), wire-up do **`fgrosse/go-coverage-report`** para comentário automático de cobertura no PR (D-28, ADR-015), criar **runbooks operacionais** (RF-15) e **README** completo, configurar **branch protection** + `gitsign` no GitHub (ADR-013). Cobre **RF-06, RF-14, RF-15, RF-16 (CI check), RF-18, RF-21 (Action no CI)**.

<requirements>
- `.github/workflows/ci.yml`:
  - Triggers: PR contra `main` + push em `main`.
  - Concurrency: cancel-in-progress por PR.
  - Jobs: `setup` (cache go.mod + .task/ + mockery) → `lint`, `unit`, `integration` (com Docker), `security`, `governance` (ai-spec doctor/lint + validate-taskfile + conventional commits check), `coverage-comment`.
  - Cada job consome o Taskfile (`task lint`, `task test:unit`, `task test:integration`, `task security:vulncheck`, `task governance:check`).
  - `setup-ai-spec` Action via `JailtonJunior94/orchestrator/.github/actions/setup-ai-spec@setup-action-v1`.
  - `arduino/setup-task@v2` com versão lida de `taskfiles/vars.yml`.
  - Coverage report posta comentário via `fgrosse/go-coverage-report@v1` (D-28).
  - Branch protection na `main` (configurada manualmente via GitHub UI ou `gh api` script): require CODEOWNER review + status checks obrigatórios (todos os jobs acima) + linear history + require signed commits.
- `.github/workflows/cd.yml`:
  - Trigger: push para `main` (após PR mergeado).
  - Steps: build imagem (Dockerfile) → push GHCR (`ghcr.io/limateixeiratecnologia/mecontrola:<sha>` + `:latest` quando tag semver) → `trivy image` → `cosign sign --yes` → `cosign attest` (SBOM SPDX + provenance SLSA) → `flyctl deploy --strategy=rolling`.
  - Permissions: `id-token: write` (OIDC para cosign), `packages: write` (GHCR), `contents: write` (release).
  - Smoke pós-deploy: `fly status -a mecontrola | grep started` em ambos processes.
- `.github/workflows/auto-merge.yml`:
  - Trigger: PR aberto por Dependabot com label `dependencies`.
  - Aprovação automática + merge se CI verde + grupo é `minor`/`patch` (major exige review manual).
- `cmd_integration_test.go` (na raiz `cmd/` com tag `//go:build integration`):
  - Compila o binário (`go build -o /tmp/mecontrola ./cmd`).
  - Executa `mecontrola --help` + `mecontrola server --help` + `mecontrola worker --help` + `mecontrola migrate --help` — assert exit 0 + stdout contém usage esperado.
  - Sobe `postgres:16-alpine` via testcontainers + executa `mecontrola migrate` real — assert migration aplicada (consulta `health_probe`).
  - Executa `mecontrola server` em background, `curl /health` 200, `curl /ready` 200, kill — assert exit 0.
  - Executa `mecontrola worker` em background — assert log "worker idle, no jobs registered" + kill graceful.
- `docs/runbooks/`:
  - `deploy.md` — fluxo `task docker:build` → push → cosign sign → `fly deploy`.
  - `rollback.md` — `fly releases rollback` + `fly deploy --image <prev>`.
  - `restore-pitr.md` — Fly Postgres PITR + smoke test.
  - `rotate-secret.md` — `fly secrets set` + smoke (`/ready` 200).
  - `upgrade-ai-spec.md` — `ai-spec upgrade --check` + revalidação `doctor/lint`.
  - `disclosure.md` — fluxo de triage para CVE recebida via `SECURITY.md`.
  - `setup-gitsign.md` — configurar gitsign localmente para novo dev.
- `README.md` na raiz:
  - Stack (Go 1.26.3, devkit-go v0.4.0, Postgres 16, Fly.io `gru`, Grafana Cloud).
  - Comandos `task` principais (`task setup`, `task build`, `task test:unit`, `task test:integration`, `task ci`).
  - Mandato SDD (link p/ PRD/techspec/ADRs).
  - Subcomandos `mecontrola` (`server`, `worker`, `migrate`).
  - Seção "Configuração" (link `.env.example`).
  - Seção "Segurança" (link `SECURITY.md` + cosign verify + Sigstore).
  - Badges: signed image, SBOM available, governance ai-spec.
</requirements>

## Subtarefas

- [ ] 10.1 Criar `.github/workflows/ci.yml` com jobs listados.
- [ ] 10.2 Criar `.github/workflows/cd.yml` com build + push + scan + sign + deploy.
- [ ] 10.3 Criar `.github/workflows/auto-merge.yml` para Dependabot.
- [ ] 10.4 Criar `cmd_integration_test.go` em `cmd/` com os 5 cenários listados.
- [ ] 10.5 Criar runbooks em `docs/runbooks/*.md` (7 documentos).
- [ ] 10.6 Criar `README.md` raiz com 7 seções listadas.
- [ ] 10.7 Configurar branch protection via `gh api` (script em `taskfiles/scripts/configure-branch-protection.sh`) — opcional automatizar; pode ser manual via UI documentado em runbook.
- [ ] 10.8 Validar pipeline com PR de exemplo: criar branch `chore/foundation-smoke`, abrir PR, validar todos os jobs verdes + comentário de cobertura postado.
- [ ] 10.9 Validar deploy real em staging Fly (`mecontrola-staging`): `fly status` mostra `app` + `worker` em `started`; `cosign verify` `verified=true`.
- [ ] 10.10 Validar `gitsign` end-to-end: commit no PR é assinado; `git log --show-signature` mostra "Good signature".

## Detalhes de Implementação

Ver techspec §"Plano de Rollout" passos 9–10 (M5–M7) + ADR-010/011/012/013/015. Não duplicar.

## Critérios de Sucesso

- PR de exemplo verde com TODOS os jobs do CI (`lint`, `unit`, `integration`, `security`, `governance`, `coverage-comment`).
- Comentário automático de cobertura postado no PR (CS-28).
- `cosign verify --certificate-identity-regexp '^https://github.com/LimaTeixeiraTecnologia/mecontrola/' --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' ghcr.io/limateixeiratecnologia/mecontrola:<sha>` retorna `verified=true` (CS-27).
- Deploy em Fly staging: `fly status -a mecontrola-staging` mostra `app` e `worker` em estado `started` (CS-24).
- `mecontrola --help` em prod responde com 3 subcomandos listados (CS-21).
- `git log --show-signature` mostra "Good signature" em commits do CODEOWNER (CS-30).
- 7 runbooks presentes em `docs/runbooks/`.
- `README.md` cobre 7 seções listadas.
- Cobre RF-06, RF-14, RF-15, RF-16 (CI check), RF-18, RF-21 parcial (CI).

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`. -->

- `pull-request` — abrir e validar o PR de smoke fim-a-fim contra `main` com descrição estruturada e checklist de aceite do deploy
- `semantic-commit` — emitir commits e tags (release `v0.1.0` no primeiro deploy de produção, conforme D-05) seguindo conventional commits para alimentar `ai-spec changelog` e `ai-spec semver-next`

## Testes da Tarefa

- [ ] Testes unitários: n/a (workflows + integration tests).
- [ ] Testes de integração: `cmd_integration_test.go` com 5 cenários (testcontainers + binário compilado + 3 subcomandos + health endpoints); execução via `task test:integration` no CI.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `.github/workflows/ci.yml`
- `.github/workflows/cd.yml`
- `.github/workflows/auto-merge.yml`
- `cmd/cmd_integration_test.go`
- `docs/runbooks/deploy.md`
- `docs/runbooks/rollback.md`
- `docs/runbooks/restore-pitr.md`
- `docs/runbooks/rotate-secret.md`
- `docs/runbooks/upgrade-ai-spec.md`
- `docs/runbooks/disclosure.md`
- `docs/runbooks/setup-gitsign.md`
- `README.md`
- `taskfiles/scripts/configure-branch-protection.sh` (opcional)
