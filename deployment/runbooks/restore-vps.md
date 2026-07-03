# Runbook: Restore Completo da VPS a partir de Backups

**Referências:** `deployment/pgbackrest/pgbackrest.conf`, `deployment/runbooks/restore-pitr.md`

## Quando Usar

- Falha catastrófica da VPS (host inacessível, volume corrompido, ransomware).
- Necessidade de recriar toda a stack em um novo servidor.
- PITR local não é suficiente ou o host original está comprometido.

## RTO Alvo

| Operação | Duração Estimada |
|----------|-----------------|
| Provisionar nova VPS (Ubuntu 24.04, Docker, AWS CLI) | 15–30 min |
| Clonar repositório e configurar SOPS + age | 5 min |
| Recriar Docker secrets e Swarm | 5 min |
| Restore do PostgreSQL via pgBackRest | 10–30 min |
| Subir stack Swarm e validar health checks | 10 min |
| **RTO total** | **< 4 horas** |

> Atualizar após primeiro restore real em staging.

## Pré-requisitos

- Acesso ao repositório Git (com `deployment/config/prod.env` e `deployment/config/prod.secrets.env` criptografado).
- Chave privada age (`AGE_PRIVATE_KEY`) disponível (GitHub secret ou cofre seguro).
- `sops` e `age` instalados na nova VPS.
- Bucket S3 `mecontrola-backups` acessível com as credenciais da conta (para restore do banco).
- Imagem da aplicação disponível em `ghcr.io/limateixeiratecnologia/mecontrola:<tag>`.
- Imagem `mecontrola-postgres:<tag>` disponível.
- Domínio e DNS apontados para a nova VPS.

## Variáveis de ambiente (na nova VPS)

```bash
export STACK=mecontrola
export STANZA=mecontrola
export VPS_DEPLOY_PATH=/opt/mecontrola
export PGBACKREST_CONF=/etc/pgbackrest/pgbackrest.conf
export IMAGE_TAG=<tag-da-imagem>
```

## Passo a Passo

### 1. Preparar a nova VPS

```bash
# Ubuntu 24.04
sudo apt-get update && sudo apt-get install -y docker.io docker-compose-plugin awscli git fail2ban age
sudo usermod -aG docker $USER
newgrp docker

# Instalar sops (versão atualizada em https://github.com/getsops/sops/releases)
curl -Lo sops.deb "https://github.com/getsops/sops/releases/download/v3.9.0/sops_3.9.0_amd64.deb"
sudo dpkg -i sops.deb
rm -f sops.deb

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

### 3. Clonar repositório

```bash
mkdir -p "$VPS_DEPLOY_PATH"
cd "$VPS_DEPLOY_PATH"
git clone <repo-url> .
git checkout "${IMAGE_TAG}"
```

### 4. Configurar SOPS + age

```bash
mkdir -p ~/.config/sops/age
# Copie a chave privada age para o arquivo abaixo (conteúdo de AGE_PRIVATE_KEY)
cat > ~/.config/sops/age/keys.txt <<'EOF'
# created: ...
# public key: age1...
AGE-SECRET-KEY-...
EOF
chmod 600 ~/.config/sops/age/keys.txt

# Testar descriptografia
sops --decrypt deployment/config/prod.secrets.env > /tmp/mecontrola-secrets.env
chmod 600 /tmp/mecontrola-secrets.env
```

### 5. Criar Docker secrets

```bash
cd "$VPS_DEPLOY_PATH"
bash deployment/scripts/create-secrets.sh /tmp/mecontrola-secrets.env
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

> As variáveis `DB_PASSWORD`, `PGBACKREST_S3_KEY` e `PGBACKREST_S3_KEY_SECRET`
> devem estar exportadas do `/tmp/mecontrola-secrets.env`:
> `export $(grep -E '^(DB_PASSWORD|PGBACKREST_S3_KEY|PGBACKREST_S3_KEY_SECRET)=' /tmp/mecontrola-secrets.env | xargs)`

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
bash deployment/scripts/deploy-swarm.sh "${IMAGE_TAG}" /tmp/mecontrola-secrets.env
```

Ou, se preferir o comando direto:

```bash
cd "$VPS_DEPLOY_PATH"
python3 deployment/scripts/render-stack.py deployment/compose/compose.swarm.yml \
  --env-file deployment/config/prod.env \
  --secrets-env-file /tmp/mecontrola-secrets.env > /tmp/stack-rendered.yml
docker stack deploy -c /tmp/stack-rendered.yml ${STACK}
rm -f /tmp/stack-rendered.yml
```

### 9. Limpar secrets temporários

```bash
rm -f /tmp/mecontrola-secrets.env
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
- Sincronizar nova config de volta para o S3 usando `deployment/scripts/backup-env-s3.sh`.

## Rollback

Se a nova VPS não funcionar, mantenha o backup do data dir original (se houver) e repita o restore com outro ponto no tempo, ou restaure o DNS para a VPS anterior enquanto ela ainda estiver acessível.
