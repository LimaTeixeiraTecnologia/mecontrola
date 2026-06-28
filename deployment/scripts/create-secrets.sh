#!/usr/bin/env bash
set -euo pipefail

ENV_FILE="${1:-.env}"
STACK="${STACK:-mecontrola}"
MODE="${MODE:-create}"

SECRETS=(
  DB_PASSWORD
  META_ACCESS_TOKEN
  META_APP_SECRET
  KIWIFY_WEBHOOK_SECRET
  KIWIFY_CLIENT_SECRET
  OPENROUTER_API_KEY
  ONBOARDING_TOKEN_ENCRYPTION_KEY
  IDENTITY_GATEWAY_SHARED_SECRET_CURRENT
  IDENTITY_GATEWAY_SHARED_SECRET_NEXT
)

log() { echo "[$(date -u +"%Y-%m-%dT%H:%M:%SZ")] $*"; }

command -v docker >/dev/null || { log "ERRO: docker não encontrado"; exit 1; }

SWARM_STATE=$(docker info --format '{{.Swarm.LocalNodeState}}' 2>/dev/null || echo "unknown")
if [[ "$SWARM_STATE" != "active" ]]; then
  log "ERRO: Docker Swarm não está ativo (estado: $SWARM_STATE)"
  exit 1
fi

[[ -f "$ENV_FILE" ]] || { log "ERRO: $ENV_FILE não encontrado"; exit 1; }

chmod 600 "$ENV_FILE"

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
