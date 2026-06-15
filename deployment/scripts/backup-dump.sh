#!/usr/bin/env bash
# deployment/scripts/backup-dump.sh
#
# Backup lógico via pg_dump dentro do container postgres.
# Alternativa ao pgBackRest quando postgres roda em Docker sem imagem customizada.
# Escreve métrica backup_last_success_timestamp_seconds no textfile do node_exporter.
#
# Uso:
#   BACKUP_REMOTE=b2:mecontrola-backups \
#   AGE_RECIPIENT=age1... \
#   ./backup-dump.sh
#
# Crontab sugerido (diário 02:00 UTC):
#   0 2 * * * root /repo/deployment/scripts/backup-dump.sh >> /var/log/backup-dump.log 2>&1

set -euo pipefail

: "${BACKUP_REMOTE:?BACKUP_REMOTE obrigatório (ex: b2:meu-bucket/backups)}"
: "${AGE_RECIPIENT:?AGE_RECIPIENT obrigatório (chave pública age)}"

DB_NAME="${DB_NAME:-mecontrola_db}"
DB_USER="${DB_USER:-mecontrola}"
RETENTION_DAYS="${RETENTION_DAYS:-30}"
TIMESTAMP="$(date -u '+%Y%m%d_%H%M%S')"
BACKUP_FILE="mecontrola_${TIMESTAMP}.sql.gz"
ENCRYPTED_FILE="${BACKUP_FILE}.age"
TMP_DIR="$(mktemp -d)"

cleanup() { rm -rf "$TMP_DIR"; }
trap cleanup EXIT

for cmd in age rclone docker; do
  if ! command -v "$cmd" >/dev/null 2>&1; then
    echo "ERRO: $cmd não encontrado." >&2
    exit 1
  fi
done

CONTAINER="$(docker ps \
  --filter "label=com.docker.compose.project=mecontrola" \
  --filter "label=com.docker.compose.service=postgres" \
  --format "{{.Names}}" | head -1)"

if [[ -z "$CONTAINER" ]]; then
  echo "ERRO: container postgres do projeto mecontrola não está rodando." >&2
  exit 1
fi

echo "==> pg_dump via container $CONTAINER..."
docker exec "$CONTAINER" \
  pg_dump -U "${DB_USER}" "${DB_NAME}" \
  | gzip -9 \
  > "${TMP_DIR}/${BACKUP_FILE}"

DUMP_SIZE="$(du -sh "${TMP_DIR}/${BACKUP_FILE}" | cut -f1)"
echo "    Dump: ${DUMP_SIZE}"

echo "==> Criptografando com age..."
age --recipient="${AGE_RECIPIENT}" \
    --output="${TMP_DIR}/${ENCRYPTED_FILE}" \
    "${TMP_DIR}/${BACKUP_FILE}"

rm "${TMP_DIR}/${BACKUP_FILE}"

echo "==> Enviando para ${BACKUP_REMOTE}..."
rclone copy "${TMP_DIR}/${ENCRYPTED_FILE}" "${BACKUP_REMOTE}/"

echo "==> Removendo backups com mais de ${RETENTION_DAYS} dias..."
rclone delete "${BACKUP_REMOTE}/" \
  --min-age "${RETENTION_DAYS}d" \
  --include "mecontrola_*.sql.gz.age"

TEXTFILE_DIR="$(docker volume inspect mecontrola_node-exporter-textfile \
  --format '{{.Mountpoint}}' 2>/dev/null || echo '')"

if [[ -n "$TEXTFILE_DIR" && -d "$TEXTFILE_DIR" ]]; then
  echo "backup_last_success_timestamp_seconds $(date +%s)" \
    > "${TEXTFILE_DIR}/backup-dump.prom.tmp"
  mv "${TEXTFILE_DIR}/backup-dump.prom.tmp" "${TEXTFILE_DIR}/backup-dump.prom"
  echo "==> Métrica backup_last_success_timestamp_seconds atualizada."
else
  echo "    AVISO: node-exporter textfile volume não encontrado; métrica não atualizada."
fi

echo "==> Backup concluído: ${ENCRYPTED_FILE} (${DUMP_SIZE})"
