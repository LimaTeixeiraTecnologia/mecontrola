#!/usr/bin/env bash
set -euo pipefail

# lint-secrets-in-config.sh — Valida que secrets não vazaram para arquivos não-seguros.
#
# Uso:
#   bash deployment/scripts/lint-secrets-in-config.sh

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
PROD_ENV="${REPO_ROOT}/deployment/config/prod.env"
SECRETS_ENV="${REPO_ROOT}/deployment/config/prod.secrets.env"
COMPOSE_SWARM="${REPO_ROOT}/deployment/compose/compose.swarm.yml"

log() { echo "[lint-secrets] $*"; }
die() { echo "[lint-secrets] ERRO: $*" >&2; exit 1; }

errors=0

if [[ ! -f "$PROD_ENV" ]]; then
  die "arquivo não encontrado: $PROD_ENV"
fi

if [[ ! -f "$SECRETS_ENV" ]]; then
  log "AVISO: $SECRETS_ENV não encontrado — rode deployment/scripts/setup-sops-age.sh e criptografe os secrets"
  exit 0
fi

# 1. Verifica se prod.env contém chaves que parecem secrets.
forbidden_keys=(
  "PASSWORD"
  "SECRET"
  "TOKEN"
  "API_KEY"
  "PRIVATE_KEY"
  "CIPHER_PASS"
  "AGE_RECIPIENT"
)

# Chaves seguras que contêm palavras acima mas são configuração pública.
allowed_keys=(
  "KIWIFY_WEBHOOK_TOKEN_HEADER"
  "KIWIFY_OAUTH_TOKEN_SAFETY_MARGIN"
  "ONBOARDING_TOKEN_TTL_DAYS"
  "ONBOARDING_TOKEN_EXPIRATION_SCHEDULE"
  "ONBOARDING_MAX_TOKEN_LOOKUP_ATTEMPTS"
  "AGENT_LLM_MAX_TOKENS"
  "AGENT_LLM_CONV_MAX_TOKENS"
  "AGENT_LLM_PARSE_MAX_TOKENS"
  "AGENT_LLM_PROSE_MAX_TOKENS"
  "AGENT_ONBOARDING_LLM_MAX_TOKENS"
  "AGENT_MECONTROLA_MAX_TOKENS"
  "AGE_RECIPIENT"
)

is_allowed() {
  local target="$1"
  for allowed in "${allowed_keys[@]}"; do
    if [[ "$target" == "$allowed" ]]; then
      return 0
    fi
  done
  return 1
}

for key in "${forbidden_keys[@]}"; do
  while IFS= read -r line; do
    line_key="${line%%=*}"
    line_key="${line_key// /}"
    if is_allowed "$line_key"; then
      continue
    fi
    log "linha suspeita em prod.env (contém '$key'):"
    echo "$line"
    errors=$((errors + 1))
  done < <(grep -Ei "^[[:space:]]*[A-Z_]*${key}[A-Z_]*[[:space:]]*=" "$PROD_ENV" || true)
done

# 2. Verifica se prod.env contém placeholders conhecidos de secrets.
if grep -Eiq 'CHANGE_ME_(USE_STRONG_PASSWORD|GENERATE_SECURE|OPENROUTER|META_|AGE1|YOUR_)' "$PROD_ENV"; then
  log "placeholder de secret encontrado em prod.env:"
  grep -Ein 'CHANGE_ME_(USE_STRONG_PASSWORD|GENERATE_SECURE|OPENROUTER|META_|AGE1|YOUR_)' "$PROD_ENV" || true
  errors=$((errors + 1))
fi

# 3. Verifica se prod.secrets.env está criptografado pelo SOPS.
if ! grep -q 'sops' "$SECRETS_ENV"; then
  die "$SECRETS_ENV não parece estar criptografado pelo SOPS"
fi

# 4. Verifica se compose.swarm.yml ainda referencia o .env legado.
if grep -Eq 'env_file.*(^|/)\.env([[:space:]]|$)' "$COMPOSE_SWARM"; then
  log "compose.swarm.yml ainda referencia .env legado:"
  grep -En 'env_file.*(^|/)\.env([[:space:]]|$)' "$COMPOSE_SWARM" || true
  errors=$((errors + 1))
fi

if [[ "$errors" -gt 0 ]]; then
  die "encontrados $errors problema(s) de vazamento de secrets"
fi

log "OK: nenhum vazamento de secrets detectado"
