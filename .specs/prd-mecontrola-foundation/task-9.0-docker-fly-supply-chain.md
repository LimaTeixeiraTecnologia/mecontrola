# Tarefa 9.0: Dockerfile distroless + `fly.toml` 2 processes + cosign keyless + Dependabot + supply chain scan tarefas

<critical>Ler prd.md e techspec.md desta pasta — sua tarefa será invalidada se você pular</critical>

## Visão Geral

Materializar o **runtime de produção**: `Dockerfile` multi-stage (builder `golang:1.26.3-alpine` + runtime `gcr.io/distroless/static-debian12:nonroot` — D-23, ADR-011), `fly.toml` declarando **2 processes** (`app=mecontrola server` + `worker=mecontrola worker` — D-24), `.github/dependabot.yml` com grupos (D-26), tarefas de **supply chain scan** no `taskfiles/security.yml` (`govulncheck` + `trivy fs` + `trivy image` + SBOM + cosign sign — D-25/D-27, ADR-012/013). Cobre **RF-07** integralmente.

<requirements>
- `Dockerfile` multi-stage:
  - **Builder**: `golang:1.26.3-alpine` (referenciar imagem oficial), `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w -X main.version=${VERSION}"`.
  - **Runtime**: `gcr.io/distroless/static-debian12:nonroot` — copia `mecontrola` + roda como UID 65532.
  - `ENTRYPOINT ["/mecontrola"]`, `CMD ["server"]` (default).
  - Imagem final ≤ 30 MB; validar via `docker image inspect`.
- `fly.toml`:
  - `app = "mecontrola"`, `primary_region = "gru"`.
  - `[build]` referencia `Dockerfile`.
  - `[[vm]]` shared-cpu-1x 256MB.
  - `[processes]` `app = "/mecontrola server"`, `worker = "/mecontrola worker"`.
  - `[[services]]` HTTP em `app` apenas; `worker` sem ingress.
  - Health checks em `app` apontando para `/ready`.
- `.github/dependabot.yml`:
  - 3 ecossistemas: `gomod`, `github-actions`, `docker`.
  - Grupos: `go-deps` (minor/patch), `ci-actions`, `docker-base`.
  - Schedule: semanal terça 06:00 UTC.
  - Limite de PRs abertos: 10.
- `taskfiles/security.yml`:
  - `task security:vulncheck` = `govulncheck ./...` + `trivy fs --severity HIGH,CRITICAL --exit-code 1 .`.
  - `task security:image-scan` = `trivy image --severity HIGH,CRITICAL --exit-code 1 ghcr.io/...:<sha>`.
  - `task security:sbom` = `trivy image --format spdx-json --output sbom.spdx.json ghcr.io/...:<sha>`.
  - `task security:sign-image` = `cosign sign --yes ghcr.io/...@<digest>` + `cosign attest --predicate sbom.spdx.json --type spdxjson` + `cosign attest --predicate provenance.json --type slsaprovenance`.
  - `task security:verify-image` = `cosign verify --certificate-identity-regexp '...' --certificate-oidc-issuer '...' ghcr.io/...:<sha>`.
- `.trivyignore` vazio inicial com header documentando processo de supressão (data + CVE + revisão 7d).
- Tarefas testáveis localmente sem GHCR (skip cosign sign se sem OIDC; só `vulncheck` e `fs` rodam local).
</requirements>

## Subtarefas

- [ ] 9.1 Criar `Dockerfile` multi-stage seguindo spec.
- [ ] 9.2 Criar `fly.toml` com 2 processes + health checks no `app`.
- [ ] 9.3 Criar `.github/dependabot.yml` com grupos e schedule.
- [ ] 9.4 Criar `.trivyignore` vazio com header.
- [ ] 9.5 Atualizar `taskfiles/security.yml` com as 5 tarefas listadas.
- [ ] 9.6 Atualizar `taskfiles/vars.yml` se versões adicionais forem necessárias (`COSIGN_VERSION`, `TRIVY_VERSION` já presentes em task 1.0).
- [ ] 9.7 Adicionar `taskfiles/scripts/build-image.sh` (helper cross-platform para `docker build`).
- [ ] 9.8 Documentar em `docs/runbooks/deploy.md` o fluxo: `task docker:build` → `trivy image` → `cosign sign` → `fly deploy`.
- [ ] 9.9 Smoke local: `task docker:build` cria imagem ≤ 30 MB; `docker image inspect mecontrola:dev | jq '.[].Config.User'` retorna `nonroot`.

## Detalhes de Implementação

Ver techspec §"Plano de Rollout" (passo 9, M5) + ADR-011 (Docker + Fly) + ADR-012 (supply chain) + ADR-013 (cosign + SBOM).

## Critérios de Sucesso

- `docker build -t mecontrola:dev .` cria imagem; `docker image inspect mecontrola:dev` mostra `User: nonroot` e size ≤ 30 MB.
- `docker run --rm mecontrola:dev --help` mostra 3 subcomandos.
- `fly validate` (ou `flyctl deploy --build-only`) valida `fly.toml` sem erro.
- `task security:vulncheck` executa localmente e termina com exit 0 (sem CVE HIGH/CRITICAL em repo limpo).
- `cat .github/dependabot.yml` cobre 3 ecossistemas com grupos.
- Cobre RF-07 integralmente.

## Skills Necessárias

<!-- MANDATÓRIO: preenchido por `create-tasks` Etapa 4.1 via descoberta agnóstica em `.agents/skills/`. -->

Nenhuma além das auto-carregadas (governance + linguagem).

## Testes da Tarefa

- [ ] Testes unitários: n/a (assets de infra; sem código Go).
- [ ] Testes de integração: smoke `docker build` + `docker run --help` + `task security:vulncheck` + `fly validate`; deploy real em staging fica para task 10.0.

<critical>SEMPRE CRIAR E EXECUTAR TESTES DA TAREFA ANTES DE CONSIDERAR A TAREFA COMO `done`</critical>

## Arquivos Relevantes

- `Dockerfile`
- `fly.toml`
- `.github/dependabot.yml`
- `.trivyignore`
- `taskfiles/security.yml`
- `taskfiles/scripts/build-image.sh`
- `docs/runbooks/deploy.md`
- `.dockerignore`
