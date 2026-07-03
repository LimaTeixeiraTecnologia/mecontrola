#!/usr/bin/env bash
set -euo pipefail

# deploy.sh — Entrypoint de deploy compatível com a estratégia SOPS + age + Docker Swarm secrets.
#
# Uso:
#   bash deployment/scripts/deploy.sh <IMAGE_TAG> [SECRETS_ENV_FILE]
#
# Este script apenas verifica se o Docker Swarm está ativo na VPS e delega para
# deployment/scripts/deploy-swarm.sh. Não há mais deploy não-Swarm com .env
# persistente na VPS.
#
# Variáveis de ambiente:
#   VPS_HOST, VPS_USER, VPS_DEPLOY_PATH, VPS_SSH_KEY

IMAGE_TAG="${1:-${IMAGE_TAG:?IMAGE_TAG obrigatorio}}"
SECRETS_ENV_FILE="${2:-${SECRETS_ENV_FILE:-}}"

VPS_HOST="${VPS_HOST:?VPS_HOST obrigatorio}"
VPS_USER="${VPS_USER:?VPS_USER obrigatorio}"
VPS_DEPLOY_PATH="${VPS_DEPLOY_PATH:?VPS_DEPLOY_PATH obrigatorio}"
VPS_SSH_KEY="${VPS_SSH_KEY:-}"

SSH_OPTS=(-o BatchMode=yes -o StrictHostKeyChecking=accept-new -o ConnectTimeout=10)
[[ -n "$VPS_SSH_KEY" ]] && SSH_OPTS+=(-i "$VPS_SSH_KEY")

log() { echo "[$(date -u +%Y-%m-%dT%H:%M:%SZ)] $*"; }

log "Iniciando deploy — tag: ${IMAGE_TAG}"

SWARM_STATE=$(ssh "${SSH_OPTS[@]}" "${VPS_USER}@${VPS_HOST}" "docker info --format '{{.Swarm.LocalNodeState}}' 2>/dev/null || echo 'unknown'")
if [[ "$SWARM_STATE" != "active" ]]; then
  log "ERRO: Docker Swarm não está ativo (estado: ${SWARM_STATE}). O deploy atual requer Swarm."
  exit 1
fi

bash deployment/scripts/deploy-swarm.sh "$IMAGE_TAG" ${SECRETS_ENV_FILE:+"$SECRETS_ENV_FILE"}
