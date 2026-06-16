#!/usr/bin/env bash
# Chaos: pausa o container Postgres por N segundos e verifica que o server recupera.
#
# Etapas:
#   1. Confere /healthz antes (200).
#   2. Verifica pool: psql `SELECT 1`.
#   3. `docker pause` no container postgres.
#   4. Durante a pausa, faz alguns hits em /healthz para confirmar degradacao
#      (esperado: /healthz volta 503 OU 200 dependendo da semantica do health
#      check do projeto — registra ambos).
#   5. Espera PAUSE_SEC (default 30).
#   6. `docker unpause`.
#   7. Poll /healthz ate retornar 200 (deadline 60s).
#   8. Confere psql `SELECT 1` voltou.
#
# Envs:
#   BACKEND          default http://localhost:8080
#   PG_CONTAINER     default mecontrola-postgres
#   PAUSE_SEC        default 30
#   DATABASE_URL     required para validar pool

set -euo pipefail

BACKEND="${BACKEND:-http://localhost:8080}"
PG_CONTAINER="${PG_CONTAINER:-mecontrola-postgres}"
PAUSE_SEC="${PAUSE_SEC:-30}"
DATABASE_URL="${DATABASE_URL:?DATABASE_URL nao definido}"
RECOVER_SEC="${RECOVER_SEC:-60}"

log() { printf '[chaos-pg %s] %s\n' "$(date +%H:%M:%S)" "$*"; }
fail() { log "FAIL: $*"; resume_safe; exit 1; }
pass() { log "PASS: $*"; }

resume_safe() {
  if docker inspect -f '{{.State.Status}}' "$PG_CONTAINER" 2>/dev/null | grep -q paused; then
    log "    cleanup: unpausing ${PG_CONTAINER}"
    docker unpause "$PG_CONTAINER" >/dev/null || true
  fi
}
trap resume_safe EXIT

command -v docker >/dev/null || fail "docker ausente"
command -v psql >/dev/null || fail "psql ausente"
command -v curl >/dev/null || fail "curl ausente"

docker inspect "$PG_CONTAINER" >/dev/null 2>&1 || fail "container ${PG_CONTAINER} nao existe"

log "==> 1/8 /healthz antes"
HC=$(curl -sS -o /dev/null -w "%{http_code}" "${BACKEND}/healthz" || echo 000)
[ "$HC" = "200" ] || fail "/healthz=${HC} antes do chaos"

log "==> 2/8 psql SELECT 1 antes"
psql "$DATABASE_URL" -tAc "SELECT 1" | grep -q '^1$' || fail "psql falhou antes do chaos"

log "==> 3/8 pausando ${PG_CONTAINER}"
docker pause "$PG_CONTAINER" >/dev/null

log "==> 4/8 amostrando /healthz durante pausa (~5 hits)"
for _ in 1 2 3 4 5; do
  HC=$(curl -sS -o /dev/null -w "%{http_code}" --max-time 3 "${BACKEND}/healthz" || echo TIMEOUT)
  log "    /healthz durante pausa = ${HC}"
  sleep 1
done

log "==> 5/8 aguardando ${PAUSE_SEC}s totais de pausa"
SLEPT=5
while [ "$SLEPT" -lt "$PAUSE_SEC" ]; do
  sleep 5
  SLEPT=$((SLEPT + 5))
done

log "==> 6/8 unpausing"
docker unpause "$PG_CONTAINER" >/dev/null

log "==> 7/8 aguardando /healthz=200 (deadline ${RECOVER_SEC}s)"
DEADLINE=$(( $(date +%s) + RECOVER_SEC ))
while :; do
  HC=$(curl -sS -o /dev/null -w "%{http_code}" --max-time 3 "${BACKEND}/healthz" || echo 000)
  if [ "$HC" = "200" ]; then
    break
  fi
  if [ "$(date +%s)" -ge "$DEADLINE" ]; then
    fail "/healthz nao voltou em ${RECOVER_SEC}s (ultimo=${HC})"
  fi
  sleep 2
done

log "==> 8/8 psql SELECT 1 apos recovery"
psql "$DATABASE_URL" -tAc "SELECT 1" | grep -q '^1$' || fail "psql falhou apos recovery"

pass "server recuperou ${PAUSE_SEC}s de Postgres indisponivel"
