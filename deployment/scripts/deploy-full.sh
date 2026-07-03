#!/usr/bin/env bash
set -euo pipefail

# deploy-full.sh — Deploy completo com descriptografia automática de secrets.
#
# Faz tudo em um comando:
#   1. Descriptografa deployment/config/prod.secrets.env com SOPS + age.
#   2. Atualiza config (prod.env) e Docker Swarm secrets na VPS.
#   3. Executa migrations, deploy da stack e health checks.
#
# Uso (imagem já publicada no GHCR):
#   AGE_PRIVATE_KEY="$(cat key.txt)" bash deployment/scripts/deploy-full.sh [IMAGE_TAG]
#
# Uso (build local e transferência direta para a VPS, sem registry):
#   AGE_PRIVATE_KEY="$(cat key.txt)" bash deployment/scripts/deploy-full.sh --local [IMAGE_TAG]
#
# Variáveis de ambiente:
#   AGE_PRIVATE_KEY      — chave privada age (conteúdo de key.txt).
#   SOPS_AGE_KEY         — alternativa à AGE_PRIVATE_KEY.
#   SOPS_AGE_KEY_FILE    — caminho para arquivo contendo a chave privada.
#   ENCRYPTED_SECRETS    — default: deployment/config/prod.secrets.env
#   PROD_ENV_FILE        — default: deployment/config/prod.env
#   VPS_HOST, VPS_USER, VPS_DEPLOY_PATH, VPS_SSH_KEY

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

DEPLOY_MODE="registry"
if [[ "${1:-}" == "--local" ]]; then
  DEPLOY_MODE="local"
  shift
fi

IMAGE_TAG="${1:-${IMAGE_TAG:-$(git rev-parse --short HEAD)}}"
ENCRYPTED_SECRETS="${ENCRYPTED_SECRETS:-deployment/config/prod.secrets.env}"
PROD_ENV_FILE="${PROD_ENV_FILE:-deployment/config/prod.env}"

log() { echo "[$(date -u +%Y-%m-%dT%H:%M:%SZ)] $*"; }
die() { echo "[ERRO] $*" >&2; exit 1; }

if [[ -n "${AGE_PRIVATE_KEY:-}" ]]; then
  export SOPS_AGE_KEY="$AGE_PRIVATE_KEY"
elif [[ -n "${SOPS_AGE_KEY:-}" ]]; then
  : # already set
elif [[ -n "${SOPS_AGE_KEY_FILE:-}" && -f "$SOPS_AGE_KEY_FILE" ]]; then
  SOPS_AGE_KEY="$(cat "$SOPS_AGE_KEY_FILE")"
  export SOPS_AGE_KEY
else
  die "chave privada age não encontrada. Defina AGE_PRIVATE_KEY, SOPS_AGE_KEY ou SOPS_AGE_KEY_FILE"
fi

command -v sops >/dev/null || die "sops não encontrado. Instale: https://github.com/getsops/sops"
[[ -f "$ENCRYPTED_SECRETS" ]] || die "arquivo criptografado não encontrado: $ENCRYPTED_SECRETS"
[[ -f "$PROD_ENV_FILE" ]] || die "arquivo de config não encontrado: $PROD_ENV_FILE"

TMP_SECRETS="$(mktemp)"
chmod 600 "$TMP_SECRETS"
trap 'rm -f "$TMP_SECRETS"' EXIT

log "Descriptografando $ENCRYPTED_SECRETS"
sops --decrypt "$ENCRYPTED_SECRETS" > "$TMP_SECRETS"

log "Iniciando deploy completo (modo: $DEPLOY_MODE, tag: $IMAGE_TAG)"
if [[ "$DEPLOY_MODE" == "local" ]]; then
  bash deployment/scripts/deploy-local.sh "$IMAGE_TAG" "$TMP_SECRETS"
else
  bash deployment/scripts/deploy-swarm.sh "$IMAGE_TAG" "$TMP_SECRETS"
fi
