# Runbook: Deploy MeControla

**Referências:** ADR-011 (Docker + VPS), ADR-012 (supply chain scan), ADR-013 (cosign keyless)

## Visão Geral

O deploy é executado automaticamente pelo workflow `.github/workflows/cd.yml` a cada push bem-sucedido em `main`.
Este runbook documenta o fluxo manual equivalente para execução em emergências ou validação local.

## Pipeline de Deploy (automático)

```
push main
  → CI (ci.yml): lint + unit + integration + security + build-image + scan-and-attest
  → CD (cd.yml): gate → deploy VPS SSH → smoke
```

O job `gate` valida que o CI passou e extrai `image-tag` + `image-digest` do artefato `image-meta`.
O job `deploy` executa `deployment/scripts/deploy.sh` na VPS via SSH.
O job `smoke` executa `task auth:smoke` contra a URL de staging.

## Fluxo Manual (emergências)

### Pré-requisitos

| Ferramenta | Instalação |
|---|---|
| `docker` | https://docs.docker.com/get-docker/ |
| `trivy` | `brew install trivy` |
| `cosign` | `brew install cosign` |
| `task` | `brew install go-task` |
| `ssh` | nativo no sistema |

```sh
docker login ghcr.io -u <github-user> -p <github-pat>
```

### 1. Build da imagem

```sh
SHA=$(git rev-parse --short HEAD)
task build:docker:build IMAGE_TAG=${SHA}
```

### 2. Push para GHCR

```sh
docker push ghcr.io/limateixeiratecnologia/mecontrola:${SHA}
```

### 3. Scan de vulnerabilidades

```sh
task security:image-scan IMAGE_SHA=${SHA}
```

### 4. Gerar SBOM e provenance

```sh
task security:sbom IMAGE_SHA=${SHA}
```

### 5. Assinar com cosign (requer OIDC — apenas no CI)

```sh
task security:sign-image \
  IMAGE_REF=ghcr.io/limateixeiratecnologia/mecontrola:${SHA} \
  IMAGE_SHA=<digest-sha256>
```

### 6. Deploy na VPS

```sh
VPS_HOST=<host> VPS_USER=<user> VPS_DEPLOY_PATH=<path> \
  bash deployment/scripts/deploy.sh "${SHA}"
```

O script executa na VPS: `docker compose pull` → `migrate` → `up -d server worker` →
healthcheck `/health` com retry 12× (interval 5s) → rollback automático se falhar.

### 7. Smoke test pós-deploy

```sh
WEBHOOK_URL=<staging_url> META_APP_SECRET=<secret> task auth:smoke
```

## Verificar Assinatura

```sh
cosign verify \
  --certificate-identity-regexp '^https://github\.com/LimaTeixeiraTecnologia/mecontrola/' \
  --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' \
  ghcr.io/limateixeiratecnologia/mecontrola:<sha>
```

## Monitoramento Pós-Deploy

| Condição | Ação |
|---|---|
| Healthcheck `/health` falha após deploy | Script reverte automaticamente; verificar `ssh VPS docker compose logs server` |
| CVE HIGH/CRITICAL no scan | Adicionar `.trivyignore` + abrir issue urgente |
| `cosign verify` falha | NÃO fazer deploy manual; investigar pipeline CI |
| Worker não inicia | `ssh VPS docker compose logs worker` |
| Migração falha | `ssh VPS docker compose logs migrate`; reverter migration manualmente |
