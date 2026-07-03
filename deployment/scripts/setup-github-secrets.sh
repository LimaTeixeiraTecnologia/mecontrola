#!/usr/bin/env bash
set -euo pipefail

# setup-github-secrets.sh — configura os secrets do environment 'production' no GitHub.
#
# Uso:
#   export VPS_HOST=<host>
#   export VPS_USER=<user>              # default: root
#   export VPS_DEPLOY_PATH=<path>       # default: /opt/mecontrola
#   export VPS_SSH_KEY_PATH=<caminho>   # default: $HOME/.ssh/id_ed25519
#   export VPS_HOST_KEY=<fingerprint>   # output de ssh-keyscan <host>
#   export AGE_PRIVATE_KEY=<chave-age>  # conteudo da chave privada age usada pelo SOPS
#   export HEALTH_URL=<url>             # ex.: https://api.exemplo.com
#   export TELEGRAM_BOT_TOKEN=<token>   # opcional
#   export TELEGRAM_CHAT_ID=<chat-id>   # opcional
#   bash deployment/scripts/setup-github-secrets.sh
#
# Pré-requisitos:
#   - gh CLI autenticado com escopo 'repo' + 'workflow': gh auth status
#   - Chave SSH privada configurada para acesso à VPS
#   - Chave age (AGE_PRIVATE_KEY) usada para descriptografar prod.secrets.env

REPO="${REPO:-LimaTeixeiraTecnologia/mecontrola}"
ENV="production"
VPS_HOST="${VPS_HOST:-}"
VPS_USER="${VPS_USER:-root}"
VPS_DEPLOY_PATH="${VPS_DEPLOY_PATH:-/opt/mecontrola}"
VPS_SSH_KEY_PATH="${VPS_SSH_KEY_PATH:-$HOME/.ssh/id_ed25519}"
VPS_HOST_KEY="${VPS_HOST_KEY:-}"
AGE_PRIVATE_KEY="${AGE_PRIVATE_KEY:-}"
HEALTH_URL="${HEALTH_URL:-}"
TELEGRAM_BOT_TOKEN="${TELEGRAM_BOT_TOKEN:-}"
TELEGRAM_CHAT_ID="${TELEGRAM_CHAT_ID:-}"

log() { echo "[setup-secrets] $*"; }
err() { echo "[setup-secrets] ERROR: $*" >&2; exit 1; }

log "Repositório: $REPO | Environment: $ENV"

command -v gh >/dev/null 2>&1 || err "gh CLI não encontrado. Instale: https://cli.github.com"
gh auth status >/dev/null 2>&1 || err "gh CLI não autenticado. Execute: gh auth login"

[[ -n "$VPS_HOST" ]] || err "VPS_HOST não configurado. Defina VPS_HOST=<host-da-vps>"
[[ -f "$VPS_SSH_KEY_PATH" ]] || err "Chave SSH não encontrada: $VPS_SSH_KEY_PATH"
[[ -n "$VPS_HOST_KEY" ]] || err "VPS_HOST_KEY não configurado. Obtenha com: ssh-keyscan $VPS_HOST"
[[ -n "$AGE_PRIVATE_KEY" ]] || err "AGE_PRIVATE_KEY não configurado. É necessário para descriptografar prod.secrets.env no CD"
[[ -n "$HEALTH_URL" ]] || err "HEALTH_URL não configurado. Ex.: https://api.exemplo.com"

log "Lendo chave SSH privada de $VPS_SSH_KEY_PATH"
DEPLOY_SSH_KEY=$(cat "$VPS_SSH_KEY_PATH")

set_secret() {
  local name="$1"
  local value="$2"
  gh secret set "$name" \
    --env "$ENV" \
    --repo "$REPO" \
    --body "$value"
  log "  ✓ $name"
}

log "Configurando secrets no environment '$ENV'..."

set_secret "VPS_HOST"          "$VPS_HOST"
set_secret "VPS_USER"          "$VPS_USER"
set_secret "VPS_DEPLOY_PATH"   "$VPS_DEPLOY_PATH"
set_secret "DEPLOY_SSH_KEY"    "$DEPLOY_SSH_KEY"
set_secret "VPS_HOST_KEY"      "$VPS_HOST_KEY"
set_secret "AGE_PRIVATE_KEY"   "$AGE_PRIVATE_KEY"
set_secret "HEALTH_URL"        "$HEALTH_URL"

[[ -n "$TELEGRAM_BOT_TOKEN" ]] && set_secret "TELEGRAM_BOT_TOKEN" "$TELEGRAM_BOT_TOKEN"
[[ -n "$TELEGRAM_CHAT_ID" ]] && set_secret "TELEGRAM_CHAT_ID" "$TELEGRAM_CHAT_ID"

log ""
log "Todos os secrets configurados. Verificando..."
gh secret list --env "$ENV" --repo "$REPO"

log ""
log "Próximos passos:"
log "  1. Garantir que deployment/config/prod.secrets.env esteja criptografado com a chage age configurada acima."
log "  2. Verificar se a imagem GHCR é pública (ou configure login na VPS):"
log "     gh browse --repo $REPO → Packages → mecontrola → Package settings"
log "  3. Disparar CD manualmente:"
log "     gh workflow run cd.yml --repo $REPO"
log "     ou empurrar um commit para main"
