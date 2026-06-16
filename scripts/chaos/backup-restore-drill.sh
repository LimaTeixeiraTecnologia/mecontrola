#!/usr/bin/env bash
# Backup/restore drill local — independente de rclone/age.
#
# Diferente de `task security:backup-restore-smoke` (que pega o ultimo dump
# cifrado do remoto), este script:
#   1. Captura counts em users, cards, transactions, budget_alerts_sent.
#   2. pg_dump direto da DB atual em arquivo plano.
#   3. Cria DB efemera, restaura nela.
#   4. Recoleta counts na DB restaurada e compara.
#   5. Reporta diff por tabela.
#
# Envs:
#   DATABASE_URL    required (source)
#   PG_CONTAINER    default mecontrola-postgres (para criar DB sandbox)
#   SANDBOX_DB      default mecontrola_restore_drill
#   DUMP_FILE       default /tmp/mecontrola-drill-$(date +%s).sql
#   KEEP_DUMP       default 0 (1 mantem dump apos run)

set -euo pipefail

DATABASE_URL="${DATABASE_URL:?DATABASE_URL nao definido}"
PG_CONTAINER="${PG_CONTAINER:-mecontrola-postgres}"
SANDBOX_DB="${SANDBOX_DB:-mecontrola_restore_drill}"
DUMP_FILE="${DUMP_FILE:-/tmp/mecontrola-drill-$(date +%s).sql}"
KEEP_DUMP="${KEEP_DUMP:-0}"
SCHEMA="${SCHEMA:-mecontrola}"

log() { printf '[bkp-drill %s] %s\n' "$(date +%H:%M:%S)" "$*"; }
fail() { log "FAIL: $*"; cleanup; exit 1; }
pass() { log "PASS: $*"; }

cleanup() {
  if [ "$KEEP_DUMP" != "1" ] && [ -f "$DUMP_FILE" ]; then
    rm -f "$DUMP_FILE"
  fi
}
trap cleanup EXIT

command -v psql >/dev/null || fail "psql ausente"
command -v pg_dump >/dev/null || fail "pg_dump ausente"

TABLES=("users" "cards" "transactions" "budget_alerts_sent")

declare -A BEFORE_COUNTS
log "==> 1/5 contagens na DB origem"
for t in "${TABLES[@]}"; do
  if ! psql "$DATABASE_URL" -tAc "SELECT 1 FROM information_schema.tables WHERE table_schema='${SCHEMA}' AND table_name='${t}'" | grep -q '^1$'; then
    log "    warn: tabela ${SCHEMA}.${t} ausente — ignorando"
    BEFORE_COUNTS[$t]="N/A"
    continue
  fi
  n=$(psql "$DATABASE_URL" -tAc "SELECT count(*) FROM ${SCHEMA}.${t}")
  BEFORE_COUNTS[$t]=$n
  log "    ${t} = ${n}"
done

log "==> 2/5 pg_dump -> ${DUMP_FILE}"
pg_dump --no-owner --no-privileges --schema="$SCHEMA" "$DATABASE_URL" > "$DUMP_FILE"
DUMP_SIZE=$(wc -c < "$DUMP_FILE" | tr -d ' ')
log "    dump size = ${DUMP_SIZE} bytes"
[ "$DUMP_SIZE" -gt 1000 ] || fail "dump suspeitosamente pequeno"

log "==> 3/5 criando DB sandbox ${SANDBOX_DB}"
ADMIN_URL=$(echo "$DATABASE_URL" | sed -E 's|/[^/?]+(\?|$)|/postgres\1|')
psql "$ADMIN_URL" -v ON_ERROR_STOP=1 -c "DROP DATABASE IF EXISTS ${SANDBOX_DB}"
psql "$ADMIN_URL" -v ON_ERROR_STOP=1 -c "CREATE DATABASE ${SANDBOX_DB}"

SANDBOX_URL=$(echo "$DATABASE_URL" | sed -E "s|/[^/?]+(\?|$)|/${SANDBOX_DB}\1|")
psql "$SANDBOX_URL" -v ON_ERROR_STOP=1 -c "CREATE SCHEMA IF NOT EXISTS ${SCHEMA}"

log "==> 4/5 restaurando dump no sandbox"
psql "$SANDBOX_URL" -v ON_ERROR_STOP=1 -f "$DUMP_FILE" >/dev/null

log "==> 5/5 contagens na DB restaurada e diff"
FAIL_COUNT=0
for t in "${TABLES[@]}"; do
  before="${BEFORE_COUNTS[$t]}"
  if [ "$before" = "N/A" ]; then
    continue
  fi
  if ! psql "$SANDBOX_URL" -tAc "SELECT 1 FROM information_schema.tables WHERE table_schema='${SCHEMA}' AND table_name='${t}'" | grep -q '^1$'; then
    log "    FAIL: ${t} ausente na DB restaurada"
    FAIL_COUNT=$((FAIL_COUNT + 1))
    continue
  fi
  after=$(psql "$SANDBOX_URL" -tAc "SELECT count(*) FROM ${SCHEMA}.${t}")
  if [ "$before" = "$after" ]; then
    log "    OK   ${t}: ${before} == ${after}"
  else
    log "    FAIL ${t}: ${before} -> ${after}"
    FAIL_COUNT=$((FAIL_COUNT + 1))
  fi
done

psql "$ADMIN_URL" -c "DROP DATABASE IF EXISTS ${SANDBOX_DB}" >/dev/null

[ "$FAIL_COUNT" = "0" ] || fail "${FAIL_COUNT} divergencias detectadas"
pass "backup/restore idempotente em todas as tabelas criticas"
