#!/usr/bin/env bash
set -euo pipefail

# setup-github-secrets.sh — configura os secrets do environment 'staging' no GitHub.
#
# Uso:
#   bash deployment/scripts/setup-github-secrets.sh
#
# Pré-requisitos:
#   - gh CLI autenticado com escopo 'repo' + 'workflow': gh auth status
#   - SSH access à VPS (chave privada local configurada)
#   - sops e age instalados, com AGE_PRIVATE_KEY configurado
#   - VPS rodando com deployment/config/prod.secrets.env criptografado no repo
#
# O script lê DB_PASSWORD do arquivo de secrets SOPS (ou do .env legado na VPS)
# e configura os secrets essenciais para o CI/CD.

REPO="${REPO:-LimaTeixeiraTecnologia/mecontrola}"
ENV="staging"
VPS_HOST="${VPS_HOST:-}"
VPS_USER="${VPS_USER:-root}"
VPS_DEPLOY_PATH="${VPS_DEPLOY_PATH:-/opt/mecontrola}"
VPS_SSH_KEY_PATH="${VPS_SSH_KEY_PATH:-$HOME/.ssh/id_ed25519}"
STAGING_WEBHOOK_URL="${STAGING_WEBHOOK_URL:-}"
STAGING_HEALTH_URL="${STAGING_HEALTH_URL:-}"

log() { echo "[setup-secrets] $*"; }
err() { echo "[setup-secrets] ERROR: $*" >&2; exit 1; }

log "Repositório: $REPO | Environment: $ENV"

command -v gh >/dev/null 2>&1 || err "gh CLI não encontrado. Instale: https://cli.github.com"
gh auth status >/dev/null 2>&1 || err "gh CLI não autenticado. Execute: gh auth login"

SECRETS_FILE=""
if [[ -f "deployment/config/prod.secrets.env" ]]; then
  command -v sops >/dev/null 2>&1 || err "sops não encontrado. Instale: https://github.com/getsops/sops"
  log "Usando secrets do arquivo SOPS"
  SECRETS_FILE=$(mktemp)
  trap 'rm -f "$SECRETS_FILE"' EXIT
  sops --decrypt deployment/config/prod.secrets.env > "$SECRETS_FILE"
  chmod 600 "$SECRETS_FILE"
elif [[ -f ".env" ]]; then
  log "AVISO: usando .env local (legado)"
  SECRETS_FILE=".env"
fi

env_value() {
  local var="$1"
  if [[ -n "$SECRETS_FILE" && -f "$SECRETS_FILE" ]]; then
    grep -E "^${var}=" "$SECRETS_FILE" | cut -d= -f2- | tail -n1
  else
    echo ""
  fi
}

if [[ -z "${VPS_HOST}" ]]; then
  err "VPS_HOST não configurado. Defina VPS_HOST=<host-da-vps>"
fi

if [[ ! -f "$VPS_SSH_KEY_PATH" ]]; then
  err "Chave SSH não encontrada: $VPS_SSH_KEY_PATH
Defina VPS_SSH_KEY_PATH=<caminho-da-chave-privada>"
fi

log "Lendo chave SSH privada de $VPS_SSH_KEY_PATH"
VPS_SSH_KEY_CONTENT=$(cat "$VPS_SSH_KEY_PATH")

DB_PASSWORD=$(env_value DB_PASSWORD)
if [[ -z "$DB_PASSWORD" ]]; then
  log "DB_PASSWORD não encontrado localmente — tentando obter da VPS (legado)"
  DB_PASSWORD=$(ssh \
    -i "$VPS_SSH_KEY_PATH" \
    -o StrictHostKeyChecking=accept-new \
    -o BatchMode=yes \
    -o ConnectTimeout=10 \
    "${VPS_USER}@${VPS_HOST}" \
    "grep '^DB_PASSWORD=' ${VPS_DEPLOY_PATH}/.env | cut -d= -f2-" 2>/dev/null) \
    || err "Não foi possível conectar à VPS ou obter DB_PASSWORD"
fi

[[ -z "$DB_PASSWORD" ]] && err "DB_PASSWORD não encontrado"
log "DB_PASSWORD obtido com sucesso."

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
set_secret "STAGING_DB_URL"         "$STAGING_DB_URL"

[[ -n "$STAGING_WEBHOOK_URL" ]] && set_secret "STAGING_WEBHOOK_URL" "$STAGING_WEBHOOK_URL"
[[ -n "$STAGING_HEALTH_URL" ]] && set_secret "STAGING_HEALTH_URL" "$STAGING_HEALTH_URL"

log ""
log "Todos os secrets configurados. Verificando..."
gh secret list --env "$ENV" --repo "$REPO"

log ""
log "Próximos passos:"
log "  1. Gerar par de chaves age e adicionar AGE_PRIVATE_KEY como secret do environment '$ENV'."
log "     bash deployment/scripts/setup-sops-age.sh"
log "  2. Preencher e criptografar deployment/config/prod.secrets.env."
log "  3. Verificar se a imagem GHCR é pública:"
log "     gh browse --repo $REPO  → Packages → mecontrola → Package settings"
log "     Se privada, ver: deployment/scripts/setup-ghcr-login.sh"
log "  4. Disparar CI/CD manualmente:"
log "     gh workflow run ci-cd.yml --repo $REPO --field image_tag=<SHA>"
log "     ou empurrar um commit para main"
