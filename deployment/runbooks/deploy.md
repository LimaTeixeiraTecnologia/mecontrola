# Runbook: Deploy MeControla

**Referências:** ADR-011 (Docker + VPS), ADR-012 (supply chain scan), ADR-013 (cosign keyless)

## Visão Geral

O deploy é executado automaticamente pelo workflow `.github/workflows/ci-cd.yml` a cada push bem-sucedido em `main`.
Este runbook documenta o fluxo manual equivalente para execução em emergências ou validação local.

A gestão de configuração e secrets segue o modelo **SOPS + age + Git + Docker Swarm secrets**:

- `deployment/config/prod.env` — configuração não-secreta, versionada em texto no Git.
- `deployment/config/prod.secrets.env` — secrets criptografados com SOPS + age, versionados no Git.
- **Não existe `.env` persistente na VPS.** O CI descriptografa os secrets no runner efêmero,
  cria/atualiza os `docker secret` na VPS via SSH e faz o deploy.
- Aplicação em produção lê secrets de `/run/secrets/<NOME>`.

## Pipeline de Deploy (automático)

```
push main
  → build + lint + unit + integration + vulncheck + agent-data-boundary
  → build-image + scan-image + sign-image
  → deploy (self-hosted staging):
      - instala sops/age
      - descriptografa deployment/config/prod.secrets.env com AGE_PRIVATE_KEY
      - SSH → cria/atualiza docker secrets
      - SSH → migrations → docker stack deploy
  → healthcheck
  → notify
```

## Pré-requisitos para deploy manual

| Ferramenta | Instalação |
|---|---|
| `docker` | https://docs.docker.com/get-docker/ |
| `trivy` | `brew install trivy` |
| `cosign` | `brew install cosign` |
| `task` | `brew install go-task` |
| `sops` | https://github.com/getsops/sops |
| `age` | https://age-encryption.org |
| `ssh` | nativo no sistema |

Configure a chave privada age localmente:

```sh
mkdir -p ~/.config/sops/age
cp /caminho/seguro/keys.txt ~/.config/sops/age/keys.txt
chmod 600 ~/.config/sops/age/keys.txt
```

## 1. Build da imagem

```sh
SHA=$(git rev-parse --short HEAD)
task build:docker:build IMAGE_TAG=${SHA}
```

## 2. Push para GHCR

```sh
docker push ghcr.io/limateixeiratecnologia/mecontrola:${SHA}
```

## 3. Scan de vulnerabilidades

```sh
task security:image-scan IMAGE_SHA=${SHA}
```

## 4. Gerar SBOM e provenance

```sh
task security:sbom IMAGE_SHA=${SHA}
```

## 5. Assinar com cosign (requer OIDC — apenas no CI)

```sh
task security:sign-image \
  IMAGE_REF=ghcr.io/limateixeiratecnologia/mecontrola:${SHA} \
  IMAGE_SHA=<digest-sha256>
```

## 6. Deploy manual na VPS (Swarm)

### 6.1 Descriptografar secrets

```sh
sops --decrypt deployment/config/prod.secrets.env > /tmp/mecontrola-secrets.env
chmod 600 /tmp/mecontrola-secrets.env
```

### 6.2 Executar deploy

```sh
export VPS_HOST=<host>
export VPS_USER=<user>
export VPS_DEPLOY_PATH=/opt/mecontrola
bash deployment/scripts/deploy-swarm.sh "${SHA}" /tmp/mecontrola-secrets.env
```

O script executa na VPS:

1. `git pull --ff-only`
2. Cria/atualiza Docker secrets a partir de `/tmp/mecontrola-secrets.env`
3. Renderiza provisioning de alertas do Grafana
4. Executa migrations via `docker run --rm`
5. Renderiza o stack Swarm e executa `docker stack deploy`
6. Aguarda health checks de `server-1`, `server-2`, `worker-1`, `worker-2`
7. Rollback automático para a tag anterior em caso de falha

### 6.3 Limpar secrets locais

```sh
rm -f /tmp/mecontrola-secrets.env
```

### 6.4 Deploy completo em um comando

O script `deployment/scripts/deploy-full.sh` descriptografa os secrets, atualiza config e secrets na VPS, executa migrations, deploy e health checks:

```sh
export VPS_HOST=<host>
export VPS_USER=<user>
export VPS_DEPLOY_PATH=/opt/mecontrola
export AGE_PRIVATE_KEY="$(cat key.txt)"

bash deployment/scripts/deploy-full.sh "${SHA}"
```

Para build local + transferência direta (sem GHCR):

```sh
bash deployment/scripts/deploy-full.sh --local "${SHA}"
```

Equivalente via Task:

```sh
task -t taskfiles/swarm.yml prod:deploy:full IMAGE_TAG="${SHA}"
task -t taskfiles/swarm.yml prod:deploy:full:local IMAGE_TAG="${SHA}"
```

## Docker Swarm Single-Node (Produção)

A stack de produção roda em Docker Swarm single-node. O arquivo canônico é `deployment/compose/compose.swarm.yml`.

### Deploy da stack Swarm (comando direto — avançado)

```sh
export IMAGE_TAG=<sha>
export VPS_HOST=<host>
export VPS_USER=<user>
export VPS_DEPLOY_PATH=/opt/mecontrola
sops --decrypt deployment/config/prod.secrets.env > /tmp/mecontrola-secrets.env
bash deployment/scripts/deploy-swarm.sh "${IMAGE_TAG}" /tmp/mecontrola-secrets.env
rm -f /tmp/mecontrola-secrets.env
```

Verificar saúde dos services:

```sh
docker service ls
docker stack ps mecontrola --no-trunc
```

### Migração de Compose para Swarm

A migração ocorre em uma única janela de manutenção. Não há snapshot/rollback
formal; mitigue o risco com backup S3 e configs versionadas no Git.

1. Notificar usuários pelo canal oficial.
2. Realizar backup do banco e dos arquivos de config (`deployment/config/prod.env` e `deployment/config/prod.secrets.env`).
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
6. Fazer deploy da stack conforme seção 6.
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
- Secrets da aplicação são montados em `/run/secrets/<NOME>`; o código Go os
  lê quando `ENVIRONMENT=production`.
- Serviços de infra (postgres, pgbouncer, otel-lgtm) recebem secrets via
  variáveis de ambiente injetadas durante o render do stack.

## Validações em Staging (antes de produção)

Antes de promover para produção, executar em ambiente de staging com dados anonimizados:

1. Subir stack Swarm com `bash deployment/scripts/deploy-swarm.sh <sha> /tmp/secrets-staging.env`.
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

## Backup de Configuração

O backup dos arquivos de configuração (não-secreto + secrets criptografados) é
feito pelo script `deployment/scripts/backup-env-s3.sh`:

```sh
bash deployment/scripts/backup-env-s3.sh
```

Requisitos: `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY` e bucket configurados
(como env vars ou via `PGBACKREST_S3_*`).

## Monitoramento Pós-Deploy

| Condição | Ação |
|---|---|
| Healthcheck falha após deploy | Script reverte automaticamente para tag anterior; verificar `docker service logs mecontrola_server-1` |
| CVE HIGH/CRITICAL no scan | Adicionar `.trivyignore` + abrir issue urgente |
| `cosign verify` falha | NÃO fazer deploy manual; investigar pipeline CI |
| Worker não inicia | `docker service logs mecontrola_worker-1` |
| Migração falha | `docker logs <container-migrate>`; seguir `restore-pitr.md` se necessário |
| Secret vazado em prod.env | CI bloqueia via `deployment/scripts/lint-secrets-in-config.sh` |
