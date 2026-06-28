#!/usr/bin/env bash
set -euo pipefail

ENV_FILE="${1:-.env}"
BUCKET="${ENV_BACKUP_S3_BUCKET:-${PGBACKREST_S3_BUCKET:-}}"
REGION="${ENV_BACKUP_S3_REGION:-${PGBACKREST_S3_REGION:-us-east-1}}"
SSE="${ENV_BACKUP_SSE:-AES256}"
STORAGE_CLASS="${ENV_BACKUP_STORAGE_CLASS:-STANDARD}"
PREFIX="${ENV_BACKUP_PREFIX:-mecontrola-env-backups}"

log() { echo "[$(date -u +"%Y-%m-%dT%H:%M:%SZ")] $*"; }

command -v aws >/dev/null || { log "ERRO: AWS CLI não encontrada"; exit 1; }

[[ -f "$ENV_FILE" ]] || { log "ERRO: $ENV_FILE não encontrado"; exit 1; }

chmod 600 "$ENV_FILE"

env_value() {
  local var="$1"
  grep -E "^${var}=" "$ENV_FILE" 2>/dev/null | cut -d= -f2- | tail -n1 || true
}

AWS_ACCESS_KEY_ID="${AWS_ACCESS_KEY_ID:-$(env_value AWS_ACCESS_KEY_ID)}"
AWS_ACCESS_KEY_ID="${AWS_ACCESS_KEY_ID:-$(env_value PGBACKREST_S3_KEY)}"
AWS_SECRET_ACCESS_KEY="${AWS_SECRET_ACCESS_KEY:-$(env_value AWS_SECRET_ACCESS_KEY)}"
AWS_SECRET_ACCESS_KEY="${AWS_SECRET_ACCESS_KEY:-$(env_value PGBACKREST_S3_KEY_SECRET)}"
BUCKET="${BUCKET:-$(env_value PGBACKREST_S3_BUCKET)}"
REGION="${REGION:-$(env_value PGBACKREST_S3_REGION)}"
REGION="${REGION:-us-east-1}"

[[ -n "$AWS_ACCESS_KEY_ID" ]] || { log "ERRO: AWS_ACCESS_KEY_ID/PGBACKREST_S3_KEY não configurado"; exit 1; }
[[ -n "$AWS_SECRET_ACCESS_KEY" ]] || { log "ERRO: AWS_SECRET_ACCESS_KEY/PGBACKREST_S3_KEY_SECRET não configurado"; exit 1; }
[[ -n "$BUCKET" ]] || { log "ERRO: bucket S3 não configurado (ENV_BACKUP_S3_BUCKET ou PGBACKREST_S3_BUCKET)"; exit 1; }

export AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY
export AWS_DEFAULT_REGION="$REGION"

timestamp=$(date -u +"%Y%m%d-%H%M%S")
key="${PREFIX}/.env-${timestamp}"

log "Fazendo upload de $ENV_FILE para s3://$BUCKET/$key"
aws s3 cp "$ENV_FILE" "s3://${BUCKET}/${key}" --sse "$SSE" --storage-class "$STORAGE_CLASS"

log "Backup concluído: s3://${BUCKET}/${key}"
