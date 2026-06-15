# Runbook: Restore VPS — pgBackRest PITR

**Referências:** deployment/pgbackrest/pgbackrest.conf, deployment/pgbackrest/crontab.txt

## Quando Usar

- Corrupção de dados causada por bug ou operação humana.
- Necessidade de restaurar estado do banco a um ponto específico no tempo.
- Falha catastrófica do VPS (restore em novo servidor).

## RTO Alvo

| Operação | Duração Estimada |
|----------|-----------------|
| Stop dos containers | < 1 min |
| Restore full + diff (último backup ~1 GB) | 5–15 min |
| Start + health check | 2 min |
| Smoke test | 2 min |
| **RTO total** | **< 20 min** |

Medir e atualizar após primeiro restore real em staging.

## Pré-requisitos

```bash
export PGBACKREST_S3_KEY=...
export PGBACKREST_S3_KEY_SECRET=...
export STANZA=mecontrola
export PGBACKREST_CONF=/etc/pgbackrest/pgbackrest.conf
```

## Passo a Passo

### 1. Parar a aplicação

```bash
cd /repo
docker compose -f deployment/compose/compose.yml -f deployment/compose/compose.prod.yml \
  stop server worker caddy
```

Verificar que não há conexões ativas:
```bash
docker exec mecontrola-postgres-1 psql -U mecontrola -c \
  "SELECT count(*) FROM pg_stat_activity WHERE datname = 'mecontrola_db';"
```

### 2. Listar backups disponíveis

```bash
pgbackrest --config="$PGBACKREST_CONF" \
  --repo1-s3-key="$PGBACKREST_S3_KEY" \
  --repo1-s3-key-secret="$PGBACKREST_S3_KEY_SECRET" \
  --stanza="$STANZA" \
  info
```

Anotar o `label` do backup full mais recente.

### 3. Parar o postgres e limpar data dir

```bash
docker compose -f deployment/compose/compose.yml -f deployment/compose/compose.prod.yml \
  stop postgres

PGDATA="$(docker volume inspect mecontrola_postgres-data --format '{{.Mountpoint}}')"
mv "$PGDATA" "${PGDATA}.bak.$(date +%Y%m%d%H%M%S)"
mkdir -p "$PGDATA"
chown 999:999 "$PGDATA"
chmod 700 "$PGDATA"
```

### 4. Restore pgBackRest

Para restore até o último ponto disponível:
```bash
pgbackrest --config="$PGBACKREST_CONF" \
  --repo1-s3-key="$PGBACKREST_S3_KEY" \
  --repo1-s3-key-secret="$PGBACKREST_S3_KEY_SECRET" \
  --stanza="$STANZA" \
  --pg1-path="$PGDATA" \
  restore
```

Para PITR (ponto específico no tempo):
```bash
pgbackrest --config="$PGBACKREST_CONF" \
  --repo1-s3-key="$PGBACKREST_S3_KEY" \
  --repo1-s3-key-secret="$PGBACKREST_S3_KEY_SECRET" \
  --stanza="$STANZA" \
  --pg1-path="$PGDATA" \
  --type=time \
  --target="2026-06-15 14:00:00 UTC" \
  --target-action=promote \
  restore
```

### 5. Iniciar postgres e verificar

```bash
docker compose -f deployment/compose/compose.yml -f deployment/compose/compose.prod.yml \
  start postgres

docker compose -f deployment/compose/compose.yml -f deployment/compose/compose.prod.yml \
  exec postgres pg_isready -U mecontrola
```

### 6. Executar migrations pós-restore

```bash
docker compose -f deployment/compose/compose.yml -f deployment/compose/compose.prod.yml \
  run --rm migrate
```

### 7. Smoke test

```bash
docker exec mecontrola-postgres-1 psql -U mecontrola mecontrola_db \
  -c "SELECT COUNT(*) FROM users;"

curl -sf https://${APP_DOMAIN}/health | jq .
```

### 8. Iniciar aplicação

```bash
docker compose -f deployment/compose/compose.yml -f deployment/compose/compose.prod.yml \
  start server worker caddy
```

### 9. Verificar logs

```bash
docker compose -f deployment/compose/compose.yml -f deployment/compose/compose.prod.yml \
  logs --tail=50 server worker
```

## Restore via pg_dump (alternativa sem pgBackRest)

Se o backup disponível for um dump `.sql.gz.age`:

```bash
AGE_KEY_FILE=/etc/age/key.txt
DUMP_FILE=mecontrola_YYYYMMDD_HHMMSS.sql.gz.age

rclone copy "${BACKUP_REMOTE}/${DUMP_FILE}" /tmp/restore/

age --decrypt \
    --identity="$AGE_KEY_FILE" \
    --output=/tmp/restore/dump.sql.gz \
    "/tmp/restore/${DUMP_FILE}"

gunzip /tmp/restore/dump.sql.gz

docker exec -i mecontrola-postgres-1 \
  psql -U mecontrola mecontrola_db < /tmp/restore/dump.sql
```

## Pós-Restore

- Documentar: timestamp do restore, causa raiz, dados perdidos (se houver).
- Atualizar RTO medido na tabela acima.
- Criar novo backup full imediatamente após restore:
  ```bash
  pgbackrest --stanza="$STANZA" --type=full backup
  ```
