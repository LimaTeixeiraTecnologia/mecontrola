#!/usr/bin/env bash
set -euo pipefail

# backup-config-s3.sh — Faz backup dos arquivos de configuração versionados para S3.
#
# Uso:
#   bash deployment/scripts/backup-config-s3.sh [repo-path]
#
# Backups:
#   - deployment/config/prod.env (texto, contém apenas configuração não-secreta)
#   - deployment/config/prod.secrets.env (já criptografado com SOPS + age)
#
# Nunca faz backup de secrets em texto plano. A fonte canonica dos secrets continua
# sendo o repositório Git (arquivo criptografado) e a chave privada age no GitHub secret.

REPO_ROOT="${1:-$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)}"
BUCKET="${ENV_BACKUP_S3_BUCKET:-${PGBACKREST_S3_BUCKET:-}}"
REGION="${ENV_BACKUP_S3_REGION:-${PGBACKREST_S3_REGION:-us-east-1}}"
SSE="${ENV_BACKUP_SSE:-AES256}"
STORAGE_CLASS="${ENV_BACKUP_STORAGE_CLASS:-STANDARD}"
PREFIX="${ENV_BACKUP_PREFIX:-mecontrola-env-backups}"

log() { echo "[$(date -u +"%Y-%m-%dT%H:%M:%SZ")] $*"; }

command -v aws >/dev/null || { log "ERRO: AWS CLI não encontrada"; exit 1; }

PROD_ENV="${REPO_ROOT}/deployment/config/prod.env"
SECRETS_ENV="${REPO_ROOT}/deployment/config/prod.secrets.env"

[[ -f "$PROD_ENV" ]] || { log "ERRO: $PROD_ENV não encontrado"; exit 1; }
[[ -f "$SECRETS_ENV" ]] || { log "ERRO: $SECRETS_ENV não encontrado"; exit 1; }

if grep -Eq '^[A-Z_]+(PASSWORD|SECRET|TOKEN|KEY)=.+CHANGE_ME' "$PROD_ENV" 2>/dev/null; then
  log "AVISO: $PROD_ENV parece conter placeholders de secrets — abortando backup"
  exit 1
fi

if ! head -n1 "$SECRETS_ENV" | grep -q 'sops'; then
  log "AVISO: $SECRETS_ENV não parece estar criptografado pelo SOPS — abortando backup"
  exit 1
fi

AWS_ACCESS_KEY_ID="${AWS_ACCESS_KEY_ID:-}"
AWS_SECRET_ACCESS_KEY="${AWS_SECRET_ACCESS_KEY:-}"
BUCKET="${BUCKET:-}"
REGION="${REGION:-us-east-1}"

[[ -n "$AWS_ACCESS_KEY_ID" ]] || { log "ERRO: AWS_ACCESS_KEY_ID não configurado"; exit 1; }
[[ -n "$AWS_SECRET_ACCESS_KEY" ]] || { log "ERRO: AWS_SECRET_ACCESS_KEY não configurado"; exit 1; }
[[ -n "$BUCKET" ]] || { log "ERRO: bucket S3 não configurado (ENV_BACKUP_S3_BUCKET ou PGBACKREST_S3_BUCKET)"; exit 1; }

export AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY
export AWS_DEFAULT_REGION="$REGION"

timestamp=$(date -u +"%Y%m%d-%H%M%S")

log "Fazendo upload de $PROD_ENV para s3://${BUCKET}/${PREFIX}/prod.env-${timestamp}"
aws s3 cp "$PROD_ENV" "s3://${BUCKET}/${PREFIX}/prod.env-${timestamp}" --sse "$SSE" --storage-class "$STORAGE_CLASS"

log "Fazendo upload de $SECRETS_ENV para s3://${BUCKET}/${PREFIX}/prod.secrets.env-${timestamp}"
aws s3 cp "$SECRETS_ENV" "s3://${BUCKET}/${PREFIX}/prod.secrets.env-${timestamp}" --sse "$SSE" --storage-class "$STORAGE_CLASS"

log "Backup concluído: s3://${BUCKET}/${PREFIX}/*-${timestamp}"
