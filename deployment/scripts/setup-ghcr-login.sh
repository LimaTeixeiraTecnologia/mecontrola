#!/usr/bin/env bash
set -euo pipefail

# setup-ghcr-login.sh — autentica a VPS no GHCR para pull de imagens privadas
#
# Só necessário se o pacote ghcr.io/limateixeiratecnologia/mecontrola for PRIVADO.
# Se o pacote for público, este script não é necessário.
#
# Pré-requisitos:
#   - Personal Access Token (classic) com escopo read:packages
#     Criar em: https://github.com/settings/tokens/new
#     Escopos: read:packages (mínimo)
#   - SSH access à VPS

VPS_HOST="${VPS_HOST:-187.77.45.48}"
VPS_USER="${VPS_USER:-root}"
VPS_SSH_KEY_PATH="${VPS_SSH_KEY_PATH:-$HOME/.ssh/id_ed25519}"
GHCR_USER="${GHCR_USER:-}"
GHCR_PAT="${GHCR_PAT:-}"

log() { echo "[setup-ghcr] $*"; }
err() { echo "[setup-ghcr] ERROR: $*" >&2; exit 1; }

[[ ! -f "$VPS_SSH_KEY_PATH" ]] && err "Chave SSH não encontrada: $VPS_SSH_KEY_PATH"

if [[ -z "$GHCR_USER" ]]; then
  printf "[setup-ghcr] GitHub username (ex: JailtonJunior94): "
  read -r GHCR_USER
fi

if [[ -z "$GHCR_PAT" ]]; then
  printf "[setup-ghcr] Personal Access Token com escopo read:packages: "
  read -rs GHCR_PAT
  echo
fi

[[ -z "$GHCR_USER" || -z "$GHCR_PAT" ]] && err "Username e PAT são obrigatórios"

log "Autenticando VPS ($VPS_HOST) no GHCR..."
ssh \
  -i "$VPS_SSH_KEY_PATH" \
  -o StrictHostKeyChecking=accept-new \
  -o BatchMode=no \
  "${VPS_USER}@${VPS_HOST}" \
  "echo '${GHCR_PAT}' | docker login ghcr.io -u '${GHCR_USER}' --password-stdin"

log "Login GHCR configurado com sucesso na VPS."
log "Teste: ssh ${VPS_USER}@${VPS_HOST} 'docker pull ghcr.io/limateixeiratecnologia/mecontrola:latest'"
