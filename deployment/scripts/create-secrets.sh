#!/usr/bin/env bash
set -euo pipefail

# create-secrets.sh — Cria/atualiza Docker Swarm secrets a partir de um arquivo .env
# descriptografado ou de variáveis de ambiente.
#
# Uso:
#   bash deployment/scripts/create-secrets.sh <arquivo-.env>
#   MODE=rotate bash deployment/scripts/create-secrets.sh <arquivo-.env>
#   CREATE_FROM_ENV=1 bash deployment/scripts/create-secrets.sh
#
# Variáveis:
#   STACK         — nome da stack (padrão: mecontrola)
#   MODE          — create (padrão) ou rotate
#   CREATE_FROM_ENV — se "1", lê valores das variáveis de ambiente em vez de arquivo.

ENV_FILE="${1:-}"
STACK="${STACK:-mecontrola}"
MODE="${MODE:-create}"
CREATE_FROM_ENV="${CREATE_FROM_ENV:-0}"

SECRETS=(
  DB_PASSWORD
  META_ACCESS_TOKEN
  META_APP_SECRET
  META_APP_SECRET_NEXT
  META_VERIFY_TOKEN
  KIWIFY_CLIENT_ID
  KIWIFY_CLIENT_SECRET
  KIWIFY_ACCOUNT_ID
  KIWIFY_WEBHOOK_SECRET
  KIWIFY_WEBHOOK_SECRET_NEXT
  KIWIFY_PRODUCT_ID_MONTHLY
  KIWIFY_PRODUCT_ID_QUARTERLY
  KIWIFY_PRODUCT_ID_ANNUAL
  META_PHONE_NUMBER_ID
  OPENROUTER_API_KEY
  ONBOARDING_TOKEN_ENCRYPTION_KEY
  IDENTITY_GATEWAY_SHARED_SECRET_CURRENT
  IDENTITY_GATEWAY_SHARED_SECRET_NEXT
  SMTP_USERNAME
  SMTP_PASSWORD
  RESEND_API_KEY
)

log() { echo "[$(date -u +"%Y-%m-%dT%H:%M:%SZ")] $*"; }

command -v docker >/dev/null || { log "ERRO: docker não encontrado"; exit 1; }

SWARM_STATE=$(docker info --format '{{.Swarm.LocalNodeState}}' 2>/dev/null || echo "unknown")
if [[ "$SWARM_STATE" != "active" ]]; then
  log "ERRO: Docker Swarm não está ativo (estado: $SWARM_STATE)"
  exit 1
fi

if [[ "$CREATE_FROM_ENV" != "1" ]]; then
  [[ -f "$ENV_FILE" ]] || { log "ERRO: arquivo de secrets não encontrado: ${ENV_FILE:-<não informado>}"; exit 1; }
  chmod 600 "$ENV_FILE"
fi

sha256_value() {
  local value="$1"
  if command -v sha256sum >/dev/null; then
    printf '%s' "$value" | sha256sum | awk '{print $1}'
  else
    printf '%s' "$value" | shasum -a 256 | awk '{print $1}'
  fi
}

env_value() {
  local var="$1"
  if [[ "$CREATE_FROM_ENV" == "1" ]]; then
    printenv "$var" || true
    return 0
  fi
  grep -E "^${var}=" "$ENV_FILE" | cut -d= -f2- | tail -n1
}

secret_exists() {
  local name="$1"
  docker secret ls --format '{{.Name}}' | grep -qx "$name"
}

secret_hash() {
  local name="$1"
  docker secret inspect --format '{{index .Spec.Labels "hash"}}' "$name" 2>/dev/null || true
}

rotate_secret_in_services() {
  local name="$1" old="$2" new="$3"
  local svc failed=0
  while IFS= read -r svc; do
    [[ -n "$svc" ]] || continue
    if docker service inspect --format '{{range .Spec.TaskTemplate.ContainerSpec.Secrets}}{{.SecretName}} {{end}}' "$svc" 2>/dev/null | grep -qw "$old"; then
      log "Atualizando service $svc: secret $name"
      if ! docker service update --detach=false --secret-rm "$old" --secret-add "source=$new,target=$name" "$svc"; then
        log "ERRO: falha ao atualizar service $svc"
        failed=1
        break
      fi
    fi
  done < <(docker service ls --format '{{.Name}}' --filter name="${STACK}_")
  return "$failed"
}

for name in "${SECRETS[@]}"; do
  value=$(env_value "$name")
  if [[ -z "$value" ]]; then
    log "AVISO: $name está vazio — pulando"
    continue
  fi

  hash=$(sha256_value "$value")
  secret_name="${STACK}_${name}"

  if ! secret_exists "$secret_name"; then
    log "Criando secret $secret_name"
    printf '%s' "$value" | docker secret create --label "hash=$hash" "$secret_name" -
    continue
  fi

  current_hash=$(secret_hash "$secret_name")
  if [[ "$current_hash" == "$hash" ]]; then
    log "Secret $secret_name inalterado"
    continue
  fi

  log "AVISO: valor de $secret_name mudou"
  if [[ "$MODE" != "rotate" ]]; then
    log "  -> rode com MODE=rotate para rotacionar"
    continue
  fi

  new_secret_name="${secret_name}_$(date +%s)"
  log "Criando secret rotacionado $new_secret_name"
  printf '%s' "$value" | docker secret create --label "hash=$hash" "$new_secret_name" -

  if rotate_secret_in_services "$name" "$secret_name" "$new_secret_name"; then
    log "Removendo secret antigo $secret_name"
    docker secret rm "$secret_name"
  else
    log "ERRO: falha na rotação de $secret_name — removendo $new_secret_name"
    docker secret rm "$new_secret_name" || true
    exit 1
  fi
done

log "Concluído"
