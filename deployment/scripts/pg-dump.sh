#!/usr/bin/env bash
set -euo pipefail

# pg-dump.sh — backup lógico diário + upload offsite criptografado
#
# Dependências no host: docker, age, rclone
# Configurar rclone com remote B2 ou R2 antes do primeiro uso:
#   rclone config  # crie um remote chamado "backup"
#
# Variáveis de ambiente obrigatórias (arquivo /etc/pg-dump.env ou .env):
#   POSTGRES_CONTAINER   — nome do container postgres (padrão: mecontrola-postgres-1)
#   POSTGRES_USER        — usuário do banco (padrão: mecontrola)
#   POSTGRES_DB          — nome do banco (padrão: mecontrola_db)
#   BACKUP_REMOTE        — rclone remote:bucket (ex: backup:mecontrola-backups)
#   AGE_RECIPIENT        — chave pública age para criptografia
#   RETENTION_DAYS       — dias de retenção no bucket (padrão: 30)
#   BACKUP_DIR           — diretório temporário local (padrão: /tmp/pg-backups)

ENV_FILE="${PG_DUMP_ENV_FILE:-/etc/pg-dump.env}"
if [[ -f "$ENV_FILE" ]]; then
  # shellcheck disable=SC1090
  source "$ENV_FILE"
fi

POSTGRES_CONTAINER="${POSTGRES_CONTAINER:-mecontrola-postgres-1}"
POSTGRES_USER="${POSTGRES_USER:-mecontrola}"
POSTGRES_DB="${POSTGRES_DB:-mecontrola_db}"
BACKUP_REMOTE="${BACKUP_REMOTE:?BACKUP_REMOTE is required (ex: backup:mecontrola-backups)}"
AGE_RECIPIENT="${AGE_RECIPIENT:?AGE_RECIPIENT is required (chave pública age)}"
RETENTION_DAYS="${RETENTION_DAYS:-30}"
BACKUP_DIR="${BACKUP_DIR:-/tmp/pg-backups}"

TIMESTAMP=$(date -u +"%Y%m%dT%H%M%SZ")
DUMP_FILE="${BACKUP_DIR}/${POSTGRES_DB}_${TIMESTAMP}.sql.gz"
ENCRYPTED_FILE="${DUMP_FILE}.age"

log() { echo "[$(date -u +"%Y-%m-%dT%H:%M:%SZ")] $*"; }

cleanup() {
  rm -f "$DUMP_FILE" "$ENCRYPTED_FILE"
}
trap cleanup EXIT

mkdir -p "$BACKUP_DIR"

log "Iniciando pg_dump de ${POSTGRES_DB} no container ${POSTGRES_CONTAINER}"
docker exec "$POSTGRES_CONTAINER" \
  pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" --no-password \
  | gzip -9 > "$DUMP_FILE"

DUMP_SIZE=$(du -sh "$DUMP_FILE" | cut -f1)
log "Dump concluído: ${DUMP_FILE} (${DUMP_SIZE})"

log "Criptografando com age"
age -r "$AGE_RECIPIENT" -o "$ENCRYPTED_FILE" "$DUMP_FILE"
rm -f "$DUMP_FILE"

log "Enviando para ${BACKUP_REMOTE}"
rclone copy "$ENCRYPTED_FILE" "${BACKUP_REMOTE}/" \
  --log-level INFO

log "Removendo backups com mais de ${RETENTION_DAYS} dias no bucket"
rclone delete "${BACKUP_REMOTE}/" \
  --min-age "${RETENTION_DAYS}d" \
  --log-level INFO

log "Backup concluído com sucesso: $(basename "$ENCRYPTED_FILE")"
