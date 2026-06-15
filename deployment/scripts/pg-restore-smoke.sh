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
RESTORE_PORT="${RESTORE_PORT:-$((20000 + RANDOM % 5000))}"
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
  for cmd in rclone age docker psql pg_isready; do
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
  latest=$(rclone lsf "${BACKUP_REMOTE}/" --format "pt" 2>/dev/null \
    | grep "\.sql\.gz\.age$" \
    | sort -t ';' -k2,2r \
    | head -1 \
    | cut -d ';' -f1)
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

  log "Executando smoke queries no schema ${schema}..."

  exec_query() {
    local label="$1"
    local query="$2"
    local expect_min="${3:-1}"
    local result
    result=$(PGPASSWORD="$RESTORE_PASSWORD" psql \
      -h 127.0.0.1 -p "$RESTORE_PORT" -U "$POSTGRES_USER" -d "$POSTGRES_DB" \
      -t -A -c "$query" 2>/dev/null || echo "-1")
    if [[ "$result" == "-1" ]]; then
      error "${label}: query falhou"
      all_ok=0
      return
    fi
    if [[ -n "$expect_min" && "$result" =~ ^[0-9]+$ && "$result" -lt "$expect_min" ]]; then
      error "${label}: resultado=${result} < esperado_min=${expect_min}"
      all_ok=0
      return
    fi
    log "OK: ${label} -> ${result}"
  }

  for table in users cards transactions; do
    exec_query "${schema}.${table} count" \
      "SELECT count(*) FROM ${schema}.${table}" 1
  done

  exec_query "transactions FK -> users valida" \
    "SELECT count(*) FROM ${schema}.transactions t JOIN ${schema}.users u ON u.id = t.user_id" 1

  exec_query "transactions freshness (max created_at)" \
    "SELECT EXTRACT(EPOCH FROM (now() - max(created_at)))::int FROM ${schema}.transactions" ""

  exec_query "schema_migrations version" \
    "SELECT version FROM public.schema_migrations ORDER BY version DESC LIMIT 1" ""

  if [[ $all_ok -eq 0 ]]; then
    error "Uma ou mais smoke queries falharam"
    exit 1
  fi
  log "Smoke queries OK"
}

main() {
  log "=== pg-restore-smoke: inicio ==="
  check_prereqs
  log "Cleanup proativo de containers orfaos pg-restore-smoke-*..."
  docker ps -a --filter "name=pg-restore-smoke-" --format '{{.Names}}' \
    | xargs -r docker rm -f >/dev/null 2>&1 || true
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
