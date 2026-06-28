# Runbook: Restore Completo da VPS a partir de Backups S3

**Referências:** `deployment/pgbackrest/pgbackrest.conf`, `deployment/runbooks/restore-pitr.md`

## Quando Usar

- Falha catastrófica da VPS (host inacessível, volume corrompido, ransomware).
- Necessidade de recriar toda a stack em um novo servidor.
- PITR local não é suficiente ou o host original está comprometido.

## RTO Alvo

| Operação | Duração Estimada |
|----------|-----------------|
| Provisionar nova VPS (Ubuntu 24.04, Docker, AWS CLI) | 15–30 min |
| Restaurar `.env` a partir do S3 | 5 min |
| Recriar Docker secrets e Swarm | 5 min |
| Restore do PostgreSQL via pgBackRest | 10–30 min |
| Subir stack Swarm e validar health checks | 10 min |
| **RTO total** | **< 4 horas** |

> Atualizar após primeiro restore real em staging.

## Pré-requisitos

- Bucket S3 `mecontrola-backups` acessível com as credenciais da conta.
- Bucket S3 com backup do `.env` no prefixo `mecontrola-env-backups/`.
- Imagem da aplicação disponível em `ghcr.io/limateixeiratecnologia/mecontrola:<tag>`.
- Imagem `mecontrola-postgres:<tag>` disponível.
- Acesso ao GitHub Actions ou a uma cópia local do repositório.
- Domínio e DNS apontados para a nova VPS.

## Variáveis de ambiente (na nova VPS)

```bash
export STACK=mecontrola
export STANZA=mecontrola
export VPS_DEPLOY_PATH=/opt/mecontrola
export PGBACKREST_CONF=/etc/pgbackrest/pgbackrest.conf
export IMAGE_TAG=<tag-da-imagem>
export S3_ENV_BUCKET=mecontrola-backups
```

## Passo a Passo

### 1. Preparar a nova VPS

```bash
# Ubuntu 24.04
sudo apt-get update && sudo apt-get install -y docker.io docker-compose-plugin awscli git fail2ban
sudo usermod -aG docker $USER
newgrp docker

# Hardening básico
sudo systemctl enable --now docker
sudo systemctl enable --now fail2ban
sudo ufw allow 22/tcp
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
sudo ufw --force enable
```

### 2. Inicializar Docker Swarm

```bash
docker swarm init --advertise-addr $(hostname -I | awk '{print $1}')
```

### 3. Recuperar `.env` do S3

```bash
mkdir -p "$VPS_DEPLOY_PATH"
cd "$VPS_DEPLOY_PATH"

# Listar backups disponíveis
aws s3 ls "s3://${S3_ENV_BUCKET}/mecontrola-env-backups/"

# Baixar o mais recente
aws s3 cp "s3://${S3_ENV_BUCKET}/mecontrola-env-backups/.env-<mais-recente>" "$VPS_DEPLOY_PATH/.env"
chmod 600 "$VPS_DEPLOY_PATH/.env"
```

### 4. Clonar repositório e checkout na tag

```bash
cd "$VPS_DEPLOY_PATH"
git clone <repo-url> .
git checkout "${IMAGE_TAG}"
```

### 5. Criar Docker secrets

```bash
cd "$VPS_DEPLOY_PATH"
bash deployment/scripts/create-secrets.sh "$VPS_DEPLOY_PATH/.env"
```

### 6. Subir PostgreSQL sem aplicação

Primeiro subimos apenas postgres para poder restaurar o data dir.

```bash
cd "$VPS_DEPLOY_PATH"
cat > /tmp/postgres-only.yml <<EOF
version: "3.8"
services:
  postgres:
    image: mecontrola-postgres:${IMAGE_TAG}
    command: ["postgres", "-c", "config_file=/etc/postgresql/postgresql.conf"]
    environment:
      POSTGRES_USER: ${DB_USER:-mecontrola}
      POSTGRES_PASSWORD: ${DB_PASSWORD:?DB_PASSWORD is required}
      POSTGRES_DB: ${DB_NAME:-mecontrola_db}
      PGBACKREST_REPO1_S3_KEY: ${PGBACKREST_S3_KEY}
      PGBACKREST_REPO1_S3_KEY_SECRET: ${PGBACKREST_S3_KEY_SECRET}
    volumes:
      - ${STACK}_postgres-data:/var/lib/postgresql/data
      - ./deployment/postgres/postgresql.conf:/etc/postgresql/postgresql.conf:ro
      - ./deployment/pgbackrest/pgbackrest.conf:/etc/pgbackrest/pgbackrest.conf:ro
    networks:
      - ${STACK}_backend
    deploy:
      replicas: 1
volumes:
  ${STACK}_postgres-data:
networks:
  ${STACK}_backend:
    external: true
EOF

docker stack deploy -c /tmp/postgres-only.yml ${STACK}
```

### 7. Restaurar o banco via pgBackRest

```bash
# Aguardar postgres iniciar para poder parar
sleep 10
docker service scale ${STACK}_postgres=0

# Restore do backup mais recente (ou PITR)
docker run --rm \
  --network ${STACK}_backend \
  -v ${STACK}_postgres-data:/var/lib/postgresql/data \
  -v ${VPS_DEPLOY_PATH}/deployment/pgbackrest/pgbackrest.conf:/etc/pgbackrest/pgbackrest.conf:ro \
  -e PGBACKREST_REPO1_S3_KEY="$PGBACKREST_S3_KEY" \
  -e PGBACKREST_REPO1_S3_KEY_SECRET="$PGBACKREST_S3_KEY_SECRET" \
  mecontrola-postgres:${IMAGE_TAG} \
  pgbackrest --config="$PGBACKREST_CONF" \
    --stanza="$STANZA" \
    --pg1-path=/var/lib/postgresql/data \
    restore

# Para PITR, adicione:
#   --type=time --target="2026-12-15 14:00:00 UTC" --target-action=promote
```

### 8. Subir stack completa

```bash
cd "$VPS_DEPLOY_PATH"
IMAGE_TAG=${IMAGE_TAG} docker stack deploy -c deployment/compose/compose.swarm.yml ${STACK}
```

### 9. Executar migrations

```bash
docker run --rm \
  --network ${STACK}_backend \
  --env-file ${VPS_DEPLOY_PATH}/.env \
  -e ENVIRONMENT=production \
  -e DB_HOST=postgres \
  -e DB_PORT=5432 \
  -e OTEL_EXPORTER_OTLP_ENDPOINT=otel-lgtm:4317 \
  -e OTEL_EXPORTER_OTLP_PROTOCOL=grpc \
  -e OTEL_EXPORTER_OTLP_INSECURE=true \
  ghcr.io/limateixeiratecnologia/mecontrola:${IMAGE_TAG} \
  migrate
```

### 10. Validar health checks

```bash
for svc in server-1 server-2 worker-1 worker-2 caddy; do
  until docker ps --filter name=${STACK}_${svc} --filter health=healthy --format '{{.Names}}' | grep -q .; do
    echo "aguardando $svc..."; sleep 5
  done
  echo "$svc: OK"
done

curl -fsS https://${APP_DOMAIN}/healthz
curl -fsS https://${APP_DOMAIN}/readyz
```

### 11. Verificar logs

```bash
for svc in server-1 server-2 worker-1 worker-2 postgres caddy; do
  echo "=== $svc ==="
  docker service logs --since 10m ${STACK}_${svc} | tail -n 30
done
```

### 12. Criar novo backup full

```bash
docker exec "${STACK}_postgres.1.$(docker service ps ${STACK}_postgres -q | head -n1)" \
  pgbackrest --stanza="$STANZA" --type=full backup
```

## Pós-Restore

- Documentar: timestamp do restore, causa raiz, RTO real, dados perdidos (se houver).
- Verificar TLS/certs do Caddy e DNS.
- Validar webhooks externos (Kiwify, WhatsApp) apontados para o novo IP/domínio.
- Atualizar GitHub Actions/secrets se o IP da VPS mudou.
- Sincronizar novo `.env` de volta para o S3.

## Rollback

Se a nova VPS não funcionar, mantenha o backup do data dir original (se houver) e repita o restore com outro ponto no tempo, ou restaure o DNS para a VPS anterior enquanto ela ainda estiver acessível.
