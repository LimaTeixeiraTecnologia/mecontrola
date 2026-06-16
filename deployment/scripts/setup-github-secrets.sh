#!/usr/bin/env bash
set -euo pipefail

# setup-github-secrets.sh — configura todos os secrets do environment 'staging' no GitHub
#
# Pré-requisitos:
#   - gh CLI autenticado com escopo 'repo' + 'workflow': gh auth status
#   - SSH access à VPS (chave privada local configurada)
#   - VPS rodando com /opt/mecontrola/.env configurado

REPO="${REPO:-LimaTeixeiraTecnologia/mecontrola}"
ENV="staging"
VPS_HOST="${VPS_HOST:-187.77.45.48}"
VPS_USER="${VPS_USER:-root}"
VPS_DEPLOY_PATH="${VPS_DEPLOY_PATH:-/opt/mecontrola}"
VPS_SSH_KEY_PATH="${VPS_SSH_KEY_PATH:-$HOME/.ssh/id_ed25519}"
STAGING_WEBHOOK_URL="${STAGING_WEBHOOK_URL:-https://api.mecontrola.app.br/api/v1/whatsapp/inbound}"
STAGING_META_APP_SECRET="${STAGING_META_APP_SECRET:-fd8f6781034975836f51ea505b3b0a13}"

log() { echo "[setup-secrets] $*"; }
err() { echo "[setup-secrets] ERROR: $*" >&2; exit 1; }

log "Repositório: $REPO | Environment: $ENV"

command -v gh >/dev/null 2>&1 || err "gh CLI não encontrado. Instale: https://cli.github.com"
gh auth status >/dev/null 2>&1 || err "gh CLI não autenticado. Execute: gh auth login"

if [[ ! -f "$VPS_SSH_KEY_PATH" ]]; then
  err "Chave SSH não encontrada: $VPS_SSH_KEY_PATH
Defina VPS_SSH_KEY_PATH=<caminho-da-chave-privada>"
fi

log "Lendo chave SSH privada de $VPS_SSH_KEY_PATH"
VPS_SSH_KEY_CONTENT=$(cat "$VPS_SSH_KEY_PATH")

log "Buscando DB_PASSWORD da VPS ($VPS_HOST)..."
DB_PASSWORD=$(ssh \
  -i "$VPS_SSH_KEY_PATH" \
  -o StrictHostKeyChecking=accept-new \
  -o BatchMode=yes \
  -o ConnectTimeout=10 \
  "${VPS_USER}@${VPS_HOST}" \
  "grep '^DB_PASSWORD=' ${VPS_DEPLOY_PATH}/.env | cut -d= -f2-" 2>/dev/null) \
  || err "Não foi possível conectar à VPS. Verifique: ssh -i $VPS_SSH_KEY_PATH ${VPS_USER}@${VPS_HOST}"

[[ -z "$DB_PASSWORD" ]] && err "DB_PASSWORD não encontrado em ${VPS_DEPLOY_PATH}/.env na VPS"
log "DB_PASSWORD obtido com sucesso."

if [[ -z "${STAGING_SMOKE_WA:-}" ]]; then
  printf "[setup-secrets] STAGING_SMOKE_WA (número WhatsApp de teste — dígitos, ex: 5511912345678): "
  read -r STAGING_SMOKE_WA
fi
[[ -z "$STAGING_SMOKE_WA" ]] && err "STAGING_SMOKE_WA é obrigatório"

STAGING_SMOKE_WA="${STAGING_SMOKE_WA#+}"

STAGING_DB_URL="postgres://mecontrola:${DB_PASSWORD}@${VPS_HOST}:5432/mecontrola_db"

log "Configurando secrets no environment '$ENV'..."

set_secret() {
  local name="$1"
  local value="$2"
  printf '%s' "$value" | gh secret set "$name" \
    --env "$ENV" \
    --repo "$REPO" \
    --body "$(cat -)"
  log "  ✓ $name"
}

set_secret "VPS_HOST"               "$VPS_HOST"
set_secret "VPS_USER"               "$VPS_USER"
set_secret "VPS_DEPLOY_PATH"        "$VPS_DEPLOY_PATH"
set_secret "VPS_SSH_KEY"            "$VPS_SSH_KEY_CONTENT"
set_secret "STAGING_WEBHOOK_URL"    "$STAGING_WEBHOOK_URL"
set_secret "STAGING_META_APP_SECRET" "$STAGING_META_APP_SECRET"
set_secret "STAGING_SMOKE_WA"       "$STAGING_SMOKE_WA"
set_secret "STAGING_DB_URL"         "$STAGING_DB_URL"

log ""
log "Todos os secrets configurados. Verificando..."
gh secret list --env "$ENV" --repo "$REPO"

log ""
log "Próximos passos:"
log "  1. Verificar se a imagem GHCR é pública:"
log "     gh browse --repo $REPO  → Packages → mecontrola → Package settings"
log "     Se privada, ver: deployment/scripts/setup-ghcr-login.sh"
log "  2. Disparar CI/CD manualmente:"
log "     gh workflow run cd.yml --repo $REPO --field image_tag=<SHA>"
log "     ou empurrar um commit para main"
