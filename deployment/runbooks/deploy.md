# Runbook: Deploy MeControla

**Referências:** ADR-011 (Docker + Fly), ADR-012 (supply chain scan), ADR-013 (cosign keyless)

## Visão Geral

O deploy é executado automaticamente pelo workflow `.github/workflows/cd.yml` a cada push em `main`.
Este runbook documenta o fluxo manual equivalente para execução em emergências ou validação local.

## Pipeline de Deploy

```
task build:docker:build  →  task security:image-scan  →  task security:sbom
  →  task security:sign-image  →  flyctl deploy --strategy=rolling
```

## Pré-requisitos

| Ferramenta | Instalação |
|---|---|
| `docker` | https://docs.docker.com/get-docker/ |
| `flyctl` | `brew install flyctl` |
| `trivy` | `brew install trivy` |
| `cosign` | `brew install cosign` |
| `task` | `brew install go-task` |

```sh
flyctl auth login
docker login ghcr.io -u <github-user> -p <github-pat>
```

## Passo a Passo

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
# Gera: sbom.spdx.json
```

### 5. Assinar com cosign (requer OIDC — apenas no CI)

```sh
# Executado automaticamente pelo cd.yml com id-token: write
task security:sign-image \
  IMAGE_REF=ghcr.io/limateixeiratecnologia/mecontrola:${SHA} \
  IMAGE_SHA=<digest-sha256>
```

### 6. Deploy no Fly.io

```sh
flyctl deploy \
  --strategy rolling \
  --image ghcr.io/limateixeiratecnologia/mecontrola:${SHA} \
  --app mecontrola
```

### 7. Smoke test pós-deploy

```sh
flyctl status -a mecontrola | grep started
curl -s https://mecontrola.fly.dev/health | jq .
curl -s https://mecontrola.fly.dev/ready | jq .
flyctl logs -a mecontrola -p app
flyctl logs -a mecontrola -p worker
```

## Verificar Assinatura

```sh
cosign verify \
  --certificate-identity-regexp '^https://github\.com/LimaTeixeiraTecnologia/mecontrola/' \
  --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' \
  ghcr.io/limateixeiratecnologia/mecontrola:<sha>
```

## Monitoramento

| Condição | Ação |
|---|---|
| `state ≠ started` após 5 min | `flyctl logs -a mecontrola` |
| CVE HIGH/CRITICAL no scan | Adicionar `.trivyignore` + abrir issue urgente |
| `cosign verify` falha | NÃO fazer deploy; investigar pipeline CI |
