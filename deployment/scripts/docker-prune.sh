#!/usr/bin/env bash
set -euo pipefail

MAX_DISK_PERCENT="${MAX_DISK_PERCENT:-80}"
PRUNE_UNTIL="${PRUNE_UNTIL:-72h}"

log() { echo "[$(date -u +%Y-%m-%dT%H:%M:%SZ)] $*"; }

DISK_USED=$(df / | awk 'NR==2{gsub(/%/,""); print $5}')
log "Uso atual do disco: ${DISK_USED}%"

if [[ "$DISK_USED" -lt "$MAX_DISK_PERCENT" ]]; then
  log "Uso abaixo de ${MAX_DISK_PERCENT}% — prune leve (imagens antigas)"
  docker image prune -f --filter "until=${PRUNE_UNTIL}" || true
  BUILDER_HELP="$(docker builder prune --help 2>&1 || true)"
  BUILDER_PRUNE_FLAGS=""
  if printf '%s' "$BUILDER_HELP" | grep -q -- '--reserved-space'; then
    BUILDER_PRUNE_FLAGS="--reserved-space 2GB"
  elif printf '%s' "$BUILDER_HELP" | grep -q -- '--keep-storage'; then
    BUILDER_PRUNE_FLAGS="--keep-storage 2GB"
  fi
  docker builder prune -f ${BUILDER_PRUNE_FLAGS} || true
else
  log "Uso em ${DISK_USED}% >= ${MAX_DISK_PERCENT}% — prune agressivo"
  docker image prune -af || true
  docker builder prune -af || true
  docker container prune -f || true
fi

log "Espaco em disco apos prune:"
df -h /
