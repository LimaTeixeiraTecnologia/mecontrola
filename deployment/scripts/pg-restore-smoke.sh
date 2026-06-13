#!/usr/bin/env bash
# pg-restore-smoke.sh — baixa dump cifrado, restaura em Postgres efemero e valida smoke queries
#
# Dependencias no host: rclone, age, docker, psql
# Variaveis de ambiente obrigatorias (arquivo /etc/pg-restore.env ou PG_RESTORE_ENV_FILE):
#   BACKUP_REMOTE        — rclone remote:bucket (ex: backup:mecontrola-backups)
#   AGE_KEY_FILE         — caminho da chave privada age (ex: /etc/age/key.txt)
#   POSTGRES_DB          — nome do banco (padrao: mecontrola_db)
#   POSTGRES_USER        — usuario do banco (padrao: mecontrola)
#   RESTORE_PORT         — porta local efemera para o container (padrao: 15432)
#   SMOKE_SCHEMA         — schema das tabelas criticas (padrao: mecontrola)
#   POSTGRES_IMAGE       — imagem docker postgres (padrao: postgres:16-alpine)
#   ALERT_CMD            — comando de alerta em caso de falha (opcional, ex: curl -s ...)
set -euo pipefail

ENV_FILE="${PG_RESTORE_ENV_FILE:-/etc/pg-restore.env}"
if [[ -f "$ENV_FILE" ]]; then
  # shellcheck disable=SC1090
  source "$ENV_FILE"
fi

BACKUP_REMOTE="${BACKUP_REMOTE:?BACKUP_REMOTE e obrigatorio (ex: backup:mecontrola-backups)}"
AGE_KEY_FILE="${AGE_KEY_FILE:?AGE_KEY_FILE e obrigatorio (caminho da chave privada age)}"
POSTGRES_DB="${POSTGRES_DB:-mecontrola_db}"
POSTGRES_USER="${POSTGRES_USER:-mecontrola}"
RESTORE_PORT="${RESTORE_PORT:-15432}"
SMOKE_SCHEMA="${SMOKE_SCHEMA:-mecontrola}"
POSTGRES_IMAGE="${POSTGRES_IMAGE:-postgres:16-alpine}"
RESTORE_PASSWORD="restore_smoke_$(date +%s)"
CONTAINER_NAME="pg-restore-smoke-$$"
WORK_DIR="${WORK_DIR:-/tmp/pg-restore-smoke}"
ALERT_CMD="${ALERT_CMD:-}"

log()   { echo "[$(date -u +"%Y-%m-%dT%H:%M:%SZ")] $*"; }
error() { echo "[$(date -u +"%Y-%m-%dT%H:%M:%SZ")] ERROR: $*" >&2; }

cleanup() {
  local exit_code=$?
  log "Iniciando cleanup..."
  docker rm -f "$CONTAINER_NAME" >/dev/null 2>&1 || true
  rm -f "${WORK_DIR}/dump.sql" "${WORK_DIR}/latest.age"
  if [[ $exit_code -ne 0 ]]; then
    error "Restore falhou com exit code $exit_code"
    if [[ -n "$ALERT_CMD" ]]; then
      log "Enviando alerta de falha..."
      eval "$ALERT_CMD" || true
    fi
  fi
  exit "$exit_code"
}
trap cleanup EXIT

check_prereqs() {
  local missing=0
  for cmd in rclone age docker psql; do
    if ! command -v "$cmd" >/dev/null 2>&1; then
      error "Binario ausente: $cmd"
      missing=1
    fi
  done
  [[ $missing -eq 0 ]] || { error "Pre-requisitos ausentes. Consulte docs/runbooks/backup-restore.md"; exit 1; }
  if [[ ! -f "$AGE_KEY_FILE" ]]; then
    error "Chave age nao encontrada: $AGE_KEY_FILE"
    exit 1
  fi
  log "Pre-requisitos OK: rclone, age, docker, psql"
}

find_latest_backup() {
  log "Localizando ultimo dump em ${BACKUP_REMOTE}..."
  local latest
  latest=$(rclone lsf "${BACKUP_REMOTE}/" --format "tp" 2>/dev/null \
    | grep "\.sql\.gz\.age$" \
    | sort -k2 -r \
    | head -1 \
    | awk '{print $2}')
  if [[ -z "$latest" ]]; then
    error "Nenhum dump .sql.gz.age encontrado em ${BACKUP_REMOTE}"
    exit 1
  fi
  echo "$latest"
}

download_backup() {
  local filename="$1"
  local dest="${WORK_DIR}/latest.age"
  log "Baixando ${filename} de ${BACKUP_REMOTE}..."
  mkdir -p "$WORK_DIR"
  rclone copy "${BACKUP_REMOTE}/${filename}" "${WORK_DIR}/" --log-level INFO
  mv "${WORK_DIR}/${filename}" "$dest"
  log "Download concluido: $dest ($(du -sh "$dest" | cut -f1))"
}

decrypt_backup() {
  local src="${WORK_DIR}/latest.age"
  local dest="${WORK_DIR}/dump.sql"
  log "Descriptografando com age..."
  age -d -i "$AGE_KEY_FILE" "$src" | gunzip -c > "$dest"
  log "Descriptografia concluida: $dest ($(du -sh "$dest" | cut -f1))"
}

start_postgres() {
  log "Subindo container Postgres efemero ${CONTAINER_NAME} na porta ${RESTORE_PORT}..."
  docker run --rm -d \
    --name "$CONTAINER_NAME" \
    -e POSTGRES_PASSWORD="$RESTORE_PASSWORD" \
    -e POSTGRES_USER="$POSTGRES_USER" \
    -e POSTGRES_DB="$POSTGRES_DB" \
    -p "${RESTORE_PORT}:5432" \
    "$POSTGRES_IMAGE" >/dev/null
  log "Container iniciado. Aguardando Postgres ficar pronto..."
  local max_attempts=30
  local attempt=0
  until PGPASSWORD="$RESTORE_PASSWORD" pg_isready -h 127.0.0.1 -p "$RESTORE_PORT" -U "$POSTGRES_USER" -q 2>/dev/null; do
    attempt=$((attempt + 1))
    if [[ $attempt -ge $max_attempts ]]; then
      error "Postgres nao ficou pronto em ${max_attempts}s"
      exit 1
    fi
    sleep 1
  done
  log "Postgres pronto apos ${attempt}s"
}

restore_dump() {
  local dump="${WORK_DIR}/dump.sql"
  log "Restaurando dump em ${POSTGRES_DB}..."
  PGPASSWORD="$RESTORE_PASSWORD" psql \
    -h 127.0.0.1 \
    -p "$RESTORE_PORT" \
    -U "$POSTGRES_USER" \
    -d "$POSTGRES_DB" \
    -f "$dump" \
    --quiet
  log "Restore concluido"
}

run_smoke_queries() {
  local schema="$SMOKE_SCHEMA"
  local all_ok=1
  local tables=("users" "cards" "transactions")

  log "Executando smoke queries no schema ${schema}..."
  for table in "${tables[@]}"; do
    local count
    count=$(PGPASSWORD="$RESTORE_PASSWORD" psql \
      -h 127.0.0.1 \
      -p "$RESTORE_PORT" \
      -U "$POSTGRES_USER" \
      -d "$POSTGRES_DB" \
      -t -A \
      -c "SELECT count(*) FROM ${schema}.${table};" 2>/dev/null || echo "-1")
    if [[ "$count" -lt 0 ]]; then
      error "Smoke query falhou: ${schema}.${table} — tabela nao encontrada ou query com erro"
      all_ok=0
    elif [[ "$count" -eq 0 ]]; then
      log "AVISO: ${schema}.${table} existe mas count=0 — verifique se o dump esta completo"
    else
      log "OK: ${schema}.${table} count=${count}"
    fi
  done

  if [[ $all_ok -eq 0 ]]; then
    error "Uma ou mais smoke queries falharam"
    exit 1
  fi
  log "Smoke queries OK: ${tables[*]}"
}

main() {
  log "=== pg-restore-smoke: inicio ==="
  check_prereqs
  local filename
  filename=$(find_latest_backup)
  download_backup "$filename"
  decrypt_backup
  start_postgres
  restore_dump
  run_smoke_queries
  log "=== pg-restore-smoke: SUCESSO — exit 0 ==="
}

main "$@"
