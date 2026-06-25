#!/usr/bin/env bash
set -euo pipefail

DSN="${DATABASE_URL:-}"

if [[ -z "$DSN" ]]; then
  DB_HOST="${DB_HOST:-localhost}"
  DB_PORT="${DB_PORT:-5432}"
  DB_USER="${DB_USER:-mecontrola}"
  DB_PASSWORD="${DB_PASSWORD:-}"
  DB_NAME="${DB_NAME:-mecontrola_db}"
  DSN="postgres://${DB_USER}:${DB_PASSWORD}@${DB_HOST}:${DB_PORT}/${DB_NAME}?sslmode=disable"
fi

run_query() {
  psql "$DSN" --no-psqlrc -t -A -c "$1"
}

check_table() {
  local table="$1"
  local count
  count=$(run_query "SELECT count(*) FROM mecontrola.${table} WHERE channel = 'telegram'" 2>/dev/null || echo "0")
  echo "$count"
}

echo "[pre-deploy-000020] Verificando premissa: zero usuarios Telegram em producao"

identities_count=$(check_table "user_identities")
sessions_count=$(check_table "onboarding_sessions")

echo "[pre-deploy-000020] user_identities com channel=telegram: ${identities_count}"
echo "[pre-deploy-000020] onboarding_sessions com channel=telegram: ${sessions_count}"

if [[ "${identities_count}" -gt 0 ]] || [[ "${sessions_count}" -gt 0 ]]; then
  echo "[pre-deploy-000020] ABORT: premissa violada — existem registros Telegram em producao."
  echo "[pre-deploy-000020] user_identities count=${identities_count}, onboarding_sessions count=${sessions_count}"
  echo "[pre-deploy-000020] NAO aplicar migration 000020. Escalar para o time."
  exit 1
fi

echo "[pre-deploy-000020] OK: count(*) telegram = 0 em user_identities e onboarding_sessions."
echo "[pre-deploy-000020] Premissa confirmada. Seguro aplicar migration 000020."
exit 0
