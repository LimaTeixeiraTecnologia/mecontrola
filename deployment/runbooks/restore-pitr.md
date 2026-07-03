# Runbook: Restore PITR com pgBackRest

**Referências:** `deployment/pgbackrest/pgbackrest.conf`, `deployment/postgres/postgresql.conf`, `deployment/runbooks/restore-vps.md`

**SLO de Recuperação — Envelope B (10k ativos/dia, single-node):**
- RPO: ≤ 10 min (derivado da configuração `archive_timeout = 600 s` em `deployment/postgres/postgresql.conf`)
- RTO: ≤ 45 min (estimado — projeção analítica sobre DB near-empty, pendente de restore real; ver `docs/runs/2026-07-03-evidencia-restore.md`)

**Base da estimativa de RPO:** `archive_timeout = 600` força a rotação e push de segmento WAL a cada 10 min mesmo sem atividade de escrita. Com WAL arquivado de forma assíncrona (`archive-async=y`), a janela máxima de perda é 10 min. Sob carga (escritas frequentes), segmentos de 16 MB são rotacionados antes do timeout e o RPO efetivo é menor.

## Quando Usar

- Corrupção de dados causada por bug ou operação humana incorreta.
- Necessidade de restaurar o banco para um ponto específico no tempo.
- Aplicação ainda roda na VPS atual; deseja-se recuperar apenas o PostgreSQL.

## RTO Alvo

Valores abaixo são **estimativas (projeção analítica), não medições** — a confirmar em restore real.

| Operação | Duração Estimada | Base da estimativa |
|----------|-----------------|-----------|
| Stop da aplicação e containers dependentes | < 2 min | projeção analítica |
| Listar backups e escolher ponto de restore | 2 min | projeção analítica |
| Restore full + aplicar WAL até o PITR | 10–25 min | DB near-empty; escala com tamanho |
| Start do postgres e validações | 3 min | projeção analítica |
| Migrations pós-restore e reativação da stack | 5 min | projeção analítica |
| **RTO total** | **< 45 min** | estimado, não medido |

**Gatilho de revisão do RTO:** quando o backup full exceder 5 GiB, re-medir e atualizar esta tabela.
Enquanto não houver restore real, os valores permanecem estimativas a confirmar.

## Pré-requisitos

- Docker Swarm ativo na VPS.
- Acesso SSH com usuário não-root e chave.
- Credenciais S3 disponíveis em `deployment/config/prod.secrets.env` (SOPS + age).
  - `PGBACKREST_S3_KEY`
  - `PGBACKREST_S3_KEY_SECRET`
  - `PGBACKREST_S3_BUCKET`
  - `PGBACKREST_S3_REGION`
- Imagem `mecontrola-postgres` disponível (com pgBackRest embutido).
- `sops` e `age` instalados na VPS.

```bash
export STACK=mecontrola
export STANZA=mecontrola
export VPS_DEPLOY_PATH=/opt/mecontrola
export PGBACKREST_CONF=/etc/pgbackrest/pgbackrest.conf

# Descriptografar secrets
sops --decrypt "${VPS_DEPLOY_PATH}/deployment/config/prod.secrets.env" > /tmp/mecontrola-secrets.env
chmod 600 /tmp/mecontrola-secrets.env
export $(grep -E '^(DB_PASSWORD|PGBACKREST_S3_KEY|PGBACKREST_S3_KEY_SECRET|PGBACKREST_S3_BUCKET|PGBACKREST_S3_REGION)=' /tmp/mecontrola-secrets.env | xargs)
```

## Passo a Passo

### 1. Notificar e isolar a aplicação

```bash
ssh <user>@<vps>
cd "$VPS_DEPLOY_PATH"

# Para não receber tráfego novo durante o restore
docker service scale ${STACK}_server-1=0 ${STACK}_server-2=0 ${STACK}_worker-1=0 ${STACK}_worker-2=0
```

### 2. Listar backups disponíveis

```bash
docker exec "${STACK}_postgres.1.$(docker service ps ${STACK}_postgres -q | head -n1)" \
  pgbackrest --config="$PGBACKREST_CONF" --stanza="$STANZA" info
```

Anotar o `label` do backup full mais recente anterior ao ponto desejado.

### 3. Parar o PostgreSQL e preservar o data dir atual

```bash
docker service scale ${STACK}_postgres=0

PGDATA="$(docker volume inspect ${STACK}_postgres-data --format '{{.Mountpoint}}')"
mv "$PGDATA" "${PGDATA}.bak.$(date +%Y%m%d%H%M%S)"
mkdir -p "$PGDATA"
chown 999:999 "$PGDATA"
chmod 700 "$PGDATA"
```

### 4. Executar restore PITR

Substitua o timestamp pelo ponto desejado em UTC.

```bash
docker run --rm \
  --network ${STACK}_backend \
  -v ${STACK}_postgres-data:/var/lib/postgresql/data \
  -v ${VPS_DEPLOY_PATH}/deployment/pgbackrest/pgbackrest.conf:/etc/pgbackrest/pgbackrest.conf:ro \
  -e PGBACKREST_REPO1_S3_KEY="$PGBACKREST_S3_KEY" \
  -e PGBACKREST_REPO1_S3_KEY_SECRET="$PGBACKREST_S3_KEY_SECRET" \
  mecontrola-postgres:${IMAGE_TAG:-latest} \
  pgbackrest --config="$PGBACKREST_CONF" \
    --stanza="$STANZA" \
    --pg1-path=/var/lib/postgresql/data \
    --type=time \
    --target="2026-12-15 14:00:00 UTC" \
    --target-action=promote \
    restore
```

Para restore até o último ponto disponível (não PITR), omita `--type=time`, `--target` e `--target-action`.

### 5. Subir o PostgreSQL e validar

```bash
docker service scale ${STACK}_postgres=1

# Aguardar readiness
until docker exec "${STACK}_postgres.1.$(docker service ps ${STACK}_postgres -q | head -n1)" \
  pg_isready -U ${DB_USER:-mecontrola} -d ${DB_NAME:-mecontrola_db}; do
  echo "aguardando postgres..."; sleep 5
done
```

### 6. Executar migrations pós-restore

```bash
set -euo pipefail
trap 'rm -f /tmp/mecontrola-migrate.env' EXIT

# Monta um env temporário com config não-secreta + DB_PASSWORD do secrets
grep '^DB_PASSWORD=' /tmp/mecontrola-secrets.env > /tmp/mecontrola-migrate.env
cat "${VPS_DEPLOY_PATH}/deployment/config/prod.env" >> /tmp/mecontrola-migrate.env

docker run --rm \
  --network ${STACK}_backend \
  --env-file /tmp/mecontrola-migrate.env \
  -e ENVIRONMENT=production \
  -e DB_HOST=postgres \
  -e DB_PORT=5432 \
  -e OTEL_EXPORTER_OTLP_ENDPOINT=otel-lgtm:4317 \
  -e OTEL_EXPORTER_OTLP_PROTOCOL=grpc \
  -e OTEL_EXPORTER_OTLP_INSECURE=true \
  ghcr.io/limateixeiratecnologia/mecontrola:${IMAGE_TAG} \
  migrate

rm -f /tmp/mecontrola-secrets.env
```

### 7. Validar dados

```bash
docker exec "${STACK}_postgres.1.$(docker service ps ${STACK}_postgres -q | head -n1)" \
  psql -U ${DB_USER:-mecontrola} -d ${DB_NAME:-mecontrola_db} \
  -c "SELECT COUNT(*) FROM mecontrola.schema_migrations;"
```

Verifique também tabelas críticas conforme o incidente.

### 8. Reativar a aplicação

```bash
docker service scale ${STACK}_server-1=1 ${STACK}_server-2=1 ${STACK}_worker-1=1 ${STACK}_worker-2=1

# Validar health checks
for svc in server-1 server-2 worker-1 worker-2; do
  until docker ps --filter name=${STACK}_${svc} --filter health=healthy --format '{{.Names}}' | grep -q .; do
    echo "aguardando $svc..."; sleep 5
  done
  echo "$svc: OK"
done
```

### 9. Verificar logs

```bash
for svc in server-1 server-2 worker-1 worker-2 postgres; do
  echo "=== $svc ==="
  docker service logs --since 10m ${STACK}_${svc} | tail -n 30
done
```

## Pós-Restore

- Documentar: timestamp do restore, causa raiz, dados perdidos (se houver).
- Atualizar a tabela de RTO com o valor real medido, substituindo a estimativa atual.
- Criar novo backup full imediatamente após o restore:
  ```bash
  docker exec "${STACK}_postgres.1.$(docker service ps ${STACK}_postgres -q | head -n1)" \
    pgbackrest --stanza="$STANZA" --type=full backup
  ```
- Em caso de falha do restore, siga `restore-vps.md` para recuperação completa da VPS a partir do S3.
