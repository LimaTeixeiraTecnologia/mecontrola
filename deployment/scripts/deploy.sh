#!/usr/bin/env bash
set -euo pipefail

IMAGE_TAG="${1:-${IMAGE_TAG:?IMAGE_TAG must be provided as argument or env var}}"
LOCAL_DEPLOY="${LOCAL_DEPLOY:-false}"
VPS_DEPLOY_PATH="${VPS_DEPLOY_PATH:-/opt/mecontrola}"
STAGING_SMOKE_WA="${STAGING_SMOKE_WA:-}"
GHCR_USER="${GHCR_USER:-}"
GHCR_TOKEN="${GHCR_TOKEN:-}"

if [[ "$LOCAL_DEPLOY" != "true" ]]; then
  VPS_HOST="${VPS_HOST:?VPS_HOST is required}"
  VPS_USER="${VPS_USER:-deploy}"
  VPS_SSH_KEY="${VPS_SSH_KEY:-}"
fi

HEALTHZ_RETRIES=24
HEALTHZ_INTERVAL=5
OTEL_RETRIES=18
OTEL_INTERVAL=5

COMPOSE_FILES="-f ${VPS_DEPLOY_PATH}/deployment/compose/compose.yml -f ${VPS_DEPLOY_PATH}/deployment/compose/compose.prod.yml"
COMPOSE_ENV="--env-file ${VPS_DEPLOY_PATH}/.env"

log() { echo "[$(date -u +"%Y-%m-%dT%H:%M:%SZ")] $*"; }

ssh_exec() {
  local key_args=()
  [[ -n "${VPS_SSH_KEY:-}" ]] && key_args=(-i "$VPS_SSH_KEY")
  ssh "${key_args[@]}" \
    -o StrictHostKeyChecking=accept-new \
    -o BatchMode=yes \
    "${VPS_USER}@${VPS_HOST}" "$@"
}

run_cmd() {
  if [[ "$LOCAL_DEPLOY" == "true" ]]; then
    bash -c "$*"
  else
    ssh_exec "$@"
  fi
}

log "Iniciando deploy — tag: ${IMAGE_TAG}"

log "Atualizando código no servidor"
run_cmd "git config --global --add safe.directory ${VPS_DEPLOY_PATH} 2>/dev/null || true"
run_cmd "cd ${VPS_DEPLOY_PATH} && git pull --ff-only"

if [[ -n "${GHCR_TOKEN}" ]]; then
  log "Autenticando no GHCR"
  run_cmd "echo '${GHCR_TOKEN}' | docker login ghcr.io -u '${GHCR_USER:-x-access-token}' --password-stdin"
fi

log "Capturando imagem anterior para rollback"
PREVIOUS_TAG=$(run_cmd "docker inspect mecontrola-server-1 --format '{{index .Config.Image}}' 2>/dev/null | sed 's/.*://' || echo ''")
log "Imagem anterior: ${PREVIOUS_TAG:-<nenhuma>}"

log "Fazendo pull da nova imagem"
run_cmd "IMAGE_TAG=${IMAGE_TAG} docker compose ${COMPOSE_ENV} ${COMPOSE_FILES} pull server worker"

log "Garantindo otel-lgtm ativo (remove containers de observabilidade legados)"
run_cmd "docker compose ${COMPOSE_ENV} ${COMPOSE_FILES} up -d --remove-orphans otel-lgtm"
for i in $(seq 1 $OTEL_RETRIES); do
  OTEL_HEALTH=$(run_cmd "docker inspect --format='{{.State.Health.Status}}' mecontrola-otel-lgtm-1 2>/dev/null || echo 'unknown'")
  if [[ "$OTEL_HEALTH" == "healthy" ]]; then
    log "otel-lgtm saudável após $((i * OTEL_INTERVAL))s"
    break
  fi
  if [[ "$i" -eq "$OTEL_RETRIES" ]]; then
    log "AVISO: otel-lgtm não ficou saudável após $((OTEL_RETRIES * OTEL_INTERVAL))s (status: ${OTEL_HEALTH}) — continuando deploy"
    break
  fi
  log "Aguardando otel-lgtm... (${i}/${OTEL_RETRIES}) status: ${OTEL_HEALTH}"
  sleep "$OTEL_INTERVAL"
done

log "Configurando alertas Telegram (idempotente; pula se ALERT_TELEGRAM_* vazios)"
run_cmd "cd ${VPS_DEPLOY_PATH} && set -a && . ./.env && set +a && if [ -n \"\${ALERT_TELEGRAM_BOT_TOKEN:-}\" ] && [ -n \"\${ALERT_TELEGRAM_CHAT_ID:-}\" ]; then GRAFANA_ADMIN_PASSWORD=\"\${OTEL_LGTM_ADMIN_PASSWORD}\" bash deployment/telemetry/grafana/setup-alerting-telegram.sh; else echo 'ALERT_TELEGRAM_* nao definidos — alertas so no painel'; fi" || log "AVISO: setup de alertas Telegram falhou — seguindo deploy"

if [[ -n "$STAGING_SMOKE_WA" ]]; then
  SMOKE_WA_DIGITS="${STAGING_SMOKE_WA#+}"
  log "Configurando app.smoke_wa na VPS (smoke user seed)"
  run_cmd "docker compose ${COMPOSE_ENV} ${COMPOSE_FILES} exec -T postgres \
    psql -U mecontrola -d mecontrola_db -c \
    \"ALTER DATABASE mecontrola_db SET app.smoke_wa = '${SMOKE_WA_DIGITS}';\"" || \
    log "AVISO: não foi possível configurar app.smoke_wa — smoke user não será semeado"
fi

log "Executando migrações"
run_cmd "IMAGE_TAG=${IMAGE_TAG} docker compose ${COMPOSE_ENV} ${COMPOSE_FILES} run --rm --no-deps migrate" || {
  log "ERRO: migrações falharam — abortando deploy"
  exit 1
}

log "Atualizando containers server e worker"
run_cmd "IMAGE_TAG=${IMAGE_TAG} docker compose ${COMPOSE_ENV} ${COMPOSE_FILES} up -d --no-deps server worker"

log "Aguardando healthcheck do container server"
for i in $(seq 1 $HEALTHZ_RETRIES); do
  HEALTH=$(run_cmd "docker inspect --format='{{.State.Health.Status}}' mecontrola-server-1 2>/dev/null || echo 'unknown'")
  if [[ "$HEALTH" == "healthy" ]]; then
    log "Healthcheck OK após $((i * HEALTHZ_INTERVAL))s"
    break
  fi
  if [[ "$i" -eq "$HEALTHZ_RETRIES" ]]; then
    log "ERRO: healthcheck falhou após $((HEALTHZ_RETRIES * HEALTHZ_INTERVAL))s (status: ${HEALTH}) — iniciando rollback"
    if [[ -n "$PREVIOUS_TAG" && "$PREVIOUS_TAG" != "$IMAGE_TAG" ]]; then
      log "Revertendo para imagem anterior: ${PREVIOUS_TAG}"
      run_cmd "IMAGE_TAG=${PREVIOUS_TAG} docker compose ${COMPOSE_ENV} ${COMPOSE_FILES} up -d --no-deps server worker" || true
    else
      log "AVISO: sem imagem anterior para rollback — containers permanecem com tag atual"
    fi
    exit 1
  fi
  log "Aguardando... (${i}/${HEALTHZ_RETRIES}) status: ${HEALTH}"
  sleep "$HEALTHZ_INTERVAL"
done

log "Limpando imagens antigas"
run_cmd "docker image prune -f --filter 'until=72h'" || true

log "Deploy concluído — ${IMAGE_TAG}"
