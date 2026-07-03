#!/usr/bin/env bash
set -euo pipefail

# deploy-swarm.sh — Deploy da stack Docker Swarm na VPS via SSH.
#
# Uso:
#   bash deployment/scripts/deploy-swarm.sh <IMAGE_TAG> [SECRETS_ENV_FILE]
#
# Variáveis de ambiente obrigatórias:
#   VPS_HOST, VPS_USER, VPS_DEPLOY_PATH
# Variável opcional:
#   VPS_SSH_KEY — caminho para chave SSH
#
# Fluxo:
#   1. git pull --ff-only na VPS
#   2. Copia deployment/config/prod.env e o arquivo de secrets descriptografado
#      para /tmp na VPS (removidos ao final)
#   3. Criar/atualizar Docker secrets
#   4. Renderizar provisioning de alertas do Grafana
#   5. Executar migrations via docker run --rm (advisory lock)
#   6. Renderizar stack Swarm e docker stack deploy
#   7. Aguardar health checks de server-1/2 e worker-1/2
#   8. Rollback manual para tag anterior em caso de falha
#
# Segurança:
#   - Não há .env persistente na VPS. Secrets trafegam apenas por /tmp
#     durante o deploy e são removidos imediatamente.
#   - O arquivo de secrets pode ser o prod.secrets.env descriptografado pelo CI.

IMAGE_TAG="${1:-${IMAGE_TAG:?IMAGE_TAG obrigatorio}}"
SECRETS_ENV_FILE="${2:-${SECRETS_ENV_FILE:-}}"
LEGACY_ENV_FILE="${VPS_DEPLOY_PATH:-/opt/mecontrola}/.env"

IMAGE_NAME="${IMAGE_NAME:-ghcr.io/limateixeiratecnologia/mecontrola}"
STACK="${STACK:-mecontrola}"
VPS_HOST="${VPS_HOST:?VPS_HOST obrigatorio}"
VPS_USER="${VPS_USER:?VPS_USER obrigatorio}"
VPS_DEPLOY_PATH="${VPS_DEPLOY_PATH:?VPS_DEPLOY_PATH obrigatorio}"
VPS_SSH_KEY="${VPS_SSH_KEY:-}"

PROD_ENV_FILE="${PROD_ENV_FILE:-deployment/config/prod.env}"

HEALTH_RETRIES="${HEALTH_RETRIES:-24}"
HEALTH_INTERVAL="${HEALTH_INTERVAL:-5}"
MIGRATE_TIMEOUT="${MIGRATE_TIMEOUT:-300}"

SSH_OPTS=(-o BatchMode=yes -o StrictHostKeyChecking=accept-new -o ConnectTimeout=10)
[[ -n "$VPS_SSH_KEY" ]] && SSH_OPTS+=(-i "$VPS_SSH_KEY")

log() { echo "[$(date -u +%Y-%m-%dT%H:%M:%SZ)] $*"; }
ssh_exec() { ssh "${SSH_OPTS[@]}" "${VPS_USER}@${VPS_HOST}" "$@"; }

upload_file() {
  local src="$1" dst="$2"
  if [[ "${LOCAL_DEPLOY:-false}" == "true" ]]; then
    cp "$src" "$dst"
    chmod 600 "$dst"
  else
    scp "${SSH_OPTS[@]}" "$src" "${VPS_USER}@${VPS_HOST}:${dst}"
    ssh "${SSH_OPTS[@]}" "${VPS_USER}@${VPS_HOST}" "chmod 600 '${dst}'"
  fi
}

log "Iniciando deploy Swarm — tag: ${IMAGE_TAG}"

if [[ -z "$SECRETS_ENV_FILE" ]]; then
  if [[ -f ".env" ]]; then
    log "AVISO: usando .env local (legado). Prefira passar o arquivo de secrets descriptografado."
    SECRETS_ENV_FILE=".env"
  elif [[ -f "$LEGACY_ENV_FILE" ]]; then
    log "AVISO: usando ${LEGACY_ENV_FILE} na VPS (legado)."
    SECRETS_ENV_FILE="$LEGACY_ENV_FILE"
  else
    log "ERRO: arquivo de secrets não informado e nenhum .env encontrado"
    exit 1
  fi
fi

[[ -f "$SECRETS_ENV_FILE" ]] || { log "ERRO: arquivo de secrets não encontrado: $SECRETS_ENV_FILE"; exit 1; }
[[ -f "$PROD_ENV_FILE" ]] || { log "ERRO: arquivo de config não encontrado: $PROD_ENV_FILE"; exit 1; }

log "Verificando estado do Swarm na VPS"
SWARM_STATE=$(ssh_exec "docker info --format '{{.Swarm.LocalNodeState}}' 2>/dev/null || echo unknown")
if [[ "$SWARM_STATE" != "active" ]]; then
  log "ERRO: Docker Swarm não está ativo (estado: $SWARM_STATE)"
  exit 1
fi

log "Capturando imagem anterior para rollback"
PREVIOUS_TAG=$(ssh_exec "docker service inspect ${STACK}_server-1 --format '{{.Spec.TaskTemplate.ContainerSpec.Image}}' 2>/dev/null | sed 's/.*://' || echo ''")
log "Imagem anterior: ${PREVIOUS_TAG:-<nenhuma>}"

log "Atualizando código na VPS"
ssh_exec "git config --global --add safe.directory ${VPS_DEPLOY_PATH} 2>/dev/null || true"
if [[ "${SKIP_GIT_PULL:-0}" == "1" ]]; then
  log "SKIP_GIT_PULL=1 — pulando git pull (assume código já sincronizado)"
elif ! ssh_exec "cd ${VPS_DEPLOY_PATH} && git pull --ff-only"; then
  log "ERRO: git pull falhou — abortando deploy (use SKIP_GIT_PULL=1 se o código já foi sincronizado)"
  exit 1
fi

REMOTE_PROD_ENV="/tmp/mecontrola-prod.env.$$"
REMOTE_SECRETS_ENV="/tmp/mecontrola-secrets.env.$$"
REMOTE_RENDERED_STACK="/tmp/mecontrola-stack-rendered.yml.$$"
trap 'ssh_exec "rm -f '\''"$REMOTE_PROD_ENV"'\'' '\''"$REMOTE_SECRETS_ENV"'\'' '\''"$REMOTE_RENDERED_STACK"'\''" >/dev/null 2>&1 || true' EXIT

log "Enviando arquivos de configuração para /tmp na VPS"
upload_file "$PROD_ENV_FILE" "$REMOTE_PROD_ENV"
upload_file "$SECRETS_ENV_FILE" "$REMOTE_SECRETS_ENV"

log "Criando/atualizando Docker secrets"
ssh_exec "cd ${VPS_DEPLOY_PATH} && bash deployment/scripts/create-secrets.sh '$REMOTE_SECRETS_ENV'"

log "Renderizando provisioning de alertas do Grafana"
ssh_exec "cd ${VPS_DEPLOY_PATH} && bash deployment/scripts/setup-grafana-alerts.sh '$REMOTE_SECRETS_ENV'"

log "Executando migrations (docker run --rm com advisory lock)"
ssh_exec "
set -euo pipefail
trap 'rm -f /tmp/mecontrola-migrate.env.$$' EXIT
secrets_path='$REMOTE_SECRETS_ENV'
prod_path='$REMOTE_PROD_ENV'
migrate_env='/tmp/mecontrola-migrate.env.$$'
cat \"\$secrets_path\" > \"\$migrate_env\"
cat \"\$prod_path\" >> \"\$migrate_env\"
docker run --rm \\
  --network ${STACK}_backend \\
  --env-file \"\$migrate_env\" \\
  -e ENVIRONMENT=production \\
  -e DB_HOST=postgres \\
  -e DB_PORT=5432 \\
  -e OTEL_EXPORTER_OTLP_ENDPOINT=otel-lgtm:4317 \\
  -e OTEL_EXPORTER_OTLP_PROTOCOL=grpc \\
  -e OTEL_EXPORTER_OTLP_INSECURE=true \\
  --name ${STACK}-migrate-${IMAGE_TAG} \\
  ${IMAGE_NAME}:${IMAGE_TAG} \\
  migrate" || {
  log "ERRO: migrações falharam — abortando deploy"
  ssh_exec "rm -f '$REMOTE_PROD_ENV' '$REMOTE_SECRETS_ENV' '$REMOTE_RENDERED_STACK'"
  exit 1
}

log "Renderizando stack Swarm a partir do compose"
ssh_exec "cd ${VPS_DEPLOY_PATH} && python3 deployment/scripts/render-stack.py deployment/compose/compose.swarm.yml --env-file '$REMOTE_PROD_ENV' --secrets-env-file '$REMOTE_SECRETS_ENV' > '$REMOTE_RENDERED_STACK'"

log "Fazendo deploy da stack Swarm"
ssh_exec "docker stack deploy -c '$REMOTE_RENDERED_STACK' ${STACK}"

wait_service_running() {
  local svc="$1"
  local retries="${2:-$HEALTH_RETRIES}"
  local interval="${3:-$HEALTH_INTERVAL}"
  for i in $(seq 1 "$retries"); do
    local state
    state=$(ssh_exec "docker service ps ${STACK}_${svc} --format '{{.CurrentState}}' 2>/dev/null | head -n1 || echo unknown")
    if [[ "$state" == Running* ]]; then
      log "${svc} em execução após $((i * interval))s"
      return 0
    fi
    log "Aguardando ${svc}... (${i}/${retries}) estado: ${state}"
    sleep "$interval"
  done
  log "ERRO: ${svc} não ficou em execução após $((retries * interval))s"
  return 1
}

wait_service_healthy() {
  local svc="$1"
  local retries="${2:-$HEALTH_RETRIES}"
  local interval="${3:-$HEALTH_INTERVAL}"
  for i in $(seq 1 "$retries"); do
    local healthy
    healthy=$(ssh_exec "docker ps --filter name=${STACK}_${svc} --filter health=healthy --format '{{.Names}}' 2>/dev/null | head -n1 || true")
    if [[ -n "$healthy" ]]; then
      log "${svc} saudável (container: ${healthy})"
      return 0
    fi
    log "Aguardando health check ${svc}... (${i}/${retries})"
    sleep "$interval"
  done
  log "ERRO: ${svc} não ficou saudável após $((retries * interval))s"
  return 1
}

log "Aguardando services entrarem em execução"
for svc in server-1 server-2 worker-1 worker-2; do
  wait_service_running "$svc" || {
    log "ERRO: deploy falhou — iniciando rollback"
    if [[ -n "$PREVIOUS_TAG" && "$PREVIOUS_TAG" != "$IMAGE_TAG" ]]; then
      log "Revertendo para imagem anterior: ${PREVIOUS_TAG}"
      ssh_exec "cd ${VPS_DEPLOY_PATH} && IMAGE_TAG=${PREVIOUS_TAG} python3 deployment/scripts/render-stack.py deployment/compose/compose.swarm.yml --env-file '$REMOTE_PROD_ENV' --secrets-env-file '$REMOTE_SECRETS_ENV' > '$REMOTE_RENDERED_STACK' && docker stack deploy -c '$REMOTE_RENDERED_STACK' ${STACK}; rm -f '$REMOTE_RENDERED_STACK'"
    else
      log "AVISO: sem imagem anterior para rollback"
    fi
    exit 1
  }
done

log "Aguardando health checks dos app services"
for svc in server-1 server-2; do
  wait_service_healthy "$svc" || {
    log "ERRO: health check de ${svc} falhou — iniciando rollback"
    if [[ -n "$PREVIOUS_TAG" && "$PREVIOUS_TAG" != "$IMAGE_TAG" ]]; then
      log "Revertendo para imagem anterior: ${PREVIOUS_TAG}"
      ssh_exec "cd ${VPS_DEPLOY_PATH} && IMAGE_TAG=${PREVIOUS_TAG} python3 deployment/scripts/render-stack.py deployment/compose/compose.swarm.yml --env-file '$REMOTE_PROD_ENV' --secrets-env-file '$REMOTE_SECRETS_ENV' > '$REMOTE_RENDERED_STACK' && docker stack deploy -c '$REMOTE_RENDERED_STACK' ${STACK}; rm -f '$REMOTE_RENDERED_STACK'"
    else
      log "AVISO: sem imagem anterior para rollback"
    fi
    exit 1
  }
done

for svc in worker-1 worker-2; do
  wait_service_healthy "$svc" || {
    log "ERRO: health check de ${svc} falhou — iniciando rollback"
    if [[ -n "$PREVIOUS_TAG" && "$PREVIOUS_TAG" != "$IMAGE_TAG" ]]; then
      log "Revertendo para imagem anterior: ${PREVIOUS_TAG}"
      ssh_exec "cd ${VPS_DEPLOY_PATH} && IMAGE_TAG=${PREVIOUS_TAG} python3 deployment/scripts/render-stack.py deployment/compose/compose.swarm.yml --env-file '$REMOTE_PROD_ENV' --secrets-env-file '$REMOTE_SECRETS_ENV' > '$REMOTE_RENDERED_STACK' && docker stack deploy -c '$REMOTE_RENDERED_STACK' ${STACK}; rm -f '$REMOTE_RENDERED_STACK'"
    else
      log "AVISO: sem imagem anterior para rollback"
    fi
    exit 1
  }
done

log "Limpando imagens antigas"
ssh_exec "docker image prune -f --filter 'until=72h'" || true

log "Deploy Swarm concluído — ${IMAGE_TAG}"
