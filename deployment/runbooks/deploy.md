# Runbook: Deploy MeControla

**Referências:** ADR-011 (Docker + VPS), ADR-012 (supply chain scan), ADR-013 (cosign keyless)

## Visão Geral

O deploy é executado automaticamente pelo workflow `.github/workflows/ci-cd.yml` a cada push bem-sucedido em `main`.
Este runbook documenta o fluxo manual equivalente para execução em emergências ou validação local.

## Pipeline de Deploy (automático)

```
push main
  → build + lint + unit + integration + vulncheck + agent-data-boundary
  → build-image + scan-image + sign-image
  → deploy (self-hosted staging): SSH → docker stack deploy
  → healthcheck
  → notify
```

O job `deploy` executa `deployment/scripts/deploy-swarm.sh` na VPS via SSH, realizando `docker stack deploy` na stack Swarm.

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

## Docker Swarm Single-Node (Produção)

A stack de produção foi migrada de Docker Compose para Docker Swarm single-node para
suportar réplicas nomeadas de `server` e `worker`, rolling updates e service discovery.
O arquivo canônico é `deployment/compose/compose.swarm.yml`.

### Deploy da stack Swarm (script)

```sh
export VPS_HOST=<host>
export VPS_USER=<user>
export VPS_DEPLOY_PATH=/opt/mecontrola
bash deployment/scripts/deploy-swarm.sh <sha>
```

O script executa: `git pull` → `create-secrets.sh` → `backup-env-s3.sh` → `docker run --rm migrate` → `docker stack deploy` → health checks de `server-1`, `server-2`, `worker-1`, `worker-2` → rollback automático para a tag anterior se algum health check falhar.

### Deploy da stack Swarm (comando direto)

```sh
IMAGE_TAG=<sha> \
  docker stack deploy -c deployment/compose/compose.swarm.yml mecontrola
```

Variáveis obrigatórias no `.env` da VPS:

| Variável | Descrição |
|---|---|
| `DB_PASSWORD` | Senha do PostgreSQL |
| `OTEL_LGTM_ADMIN_PASSWORD` | Senha do admin do Grafana |
| `IMAGE_TAG` | Tag imutável da imagem da aplicação (digest ou SHA) |

Verificar saúde dos services:

```sh
docker service ls
docker stack ps mecontrola --no-trunc
```

### Migração de Compose para Swarm

A migração ocorre em uma única janela de manutenção. Não há snapshot/rollback
formal; mitigue o risco com backup S3 e configs versionadas no Git.

1. Notificar usuários pelo canal oficial.
2. Realizar backup do banco e do `.env`.
3. Parar a stack Compose atual:
   ```sh
   cd <deploy-path>
   docker compose -f deployment/compose/compose.yml \
     -f deployment/compose/compose.prod.yml down
   ```
4. Inicializar o Swarm:
   ```sh
   docker swarm init --advertise-addr <ip-da-vps>
   ```
5. Garantir que a imagem da aplicação está publicada em GHCR com tag imutável.
6. Fazer deploy da stack:
   ```sh
   IMAGE_TAG=<sha> docker stack deploy \
     -c deployment/compose/compose.swarm.yml mecontrola
   ```
7. Acompanhar a ordem de startup: `postgres` → `pgbouncer` → `migrate` →
   `server-1`/`server-2`/`worker-1`/`worker-2` → `caddy`.
8. Validar health checks e métricas no Grafana.
9. Em caso de falha grave, derrubar a stack e recriar a partir do último
   backup S3 + configs do Git.

### Notas sobre o compose.swarm.yml

- Cada réplica é um service nomeado (`server-1`, `server-2`, `worker-1`,
  `worker-2`) para permitir health checks individuais no Caddy.
- A rede `backend` é overlay criptografada; `frontend` é overlay e só expõe
  Caddy nas portas 80/443.
- PostgreSQL não expõe a porta 5432 externamente.
- `depends_on` no Swarm não suporta `condition: service_healthy`; a ordem de
  startup é garantida pelos healthchecks e pelas políticas de restart dos
  services downstream.

## Validações em Staging (antes de produção)

Antes de promover para produção, executar em ambiente de staging com dados anonimizados:

1. Subir stack Swarm com `docker stack deploy -c deployment/compose/compose.swarm.yml mecontrola-staging`.
2. Validar health checks de `server-1`, `server-2`, `worker-1`, `worker-2`, `caddy`, `postgres`, `pgbouncer`.
3. Verificar métricas e logs no Grafana (`http://<staging>:3000`).
4. Confirmar que alertas de infraestrutura estão carregados (`deployment/telemetry/grafana/provisioning/alerting/rules.yaml`).
5. Testar idempotência: reprocessar jobs de outbox e verificar ausência de duplicatas.
6. Testar restore PITR em instância isolada seguindo `restore-pitr.md`.
7. Sanity check de carga com k6/locust (local).

> A tarefa 6.0 (PostgreSQL/pgBouncer/pgBackRest) deve estar `done` para que os testes de restore PITR e restore de VPS sejam considerados comprovados.

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
| Healthcheck falha após deploy | Script reverte automaticamente para tag anterior; verificar `docker service logs mecontrola_server-1` |
| CVE HIGH/CRITICAL no scan | Adicionar `.trivyignore` + abrir issue urgente |
| `cosign verify` falha | NÃO fazer deploy manual; investigar pipeline CI |
| Worker não inicia | `docker service logs mecontrola_worker-1` |
| Migração falha | `docker logs <container-migrate>`; seguir `restore-pitr.md` se necessário |
