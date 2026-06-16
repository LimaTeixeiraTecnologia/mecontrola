#!/usr/bin/env bash
set -euo pipefail

IMAGE_TAG="${1:-${IMAGE_TAG:?IMAGE_TAG must be provided as argument or env var}}"
VPS_HOST="${VPS_HOST:?VPS_HOST is required}"
VPS_USER="${VPS_USER:-deploy}"
VPS_SSH_KEY="${VPS_SSH_KEY:-}"
VPS_DEPLOY_PATH="${VPS_DEPLOY_PATH:-/opt/mecontrola}"
STAGING_SMOKE_WA="${STAGING_SMOKE_WA:-}"
GHCR_USER="${GHCR_USER:-}"
GHCR_TOKEN="${GHCR_TOKEN:-}"

HEALTHZ_RETRIES=24
HEALTHZ_INTERVAL=5
OTELCOL_RETRIES=12
OTELCOL_INTERVAL=5

COMPOSE_FILES="-f ${VPS_DEPLOY_PATH}/deployment/compose/compose.yml -f ${VPS_DEPLOY_PATH}/deployment/compose/compose.prod.yml"
COMPOSE_ENV="--env-file ${VPS_DEPLOY_PATH}/.env"

log() { echo "[$(date -u +"%Y-%m-%dT%H:%M:%SZ")] $*"; }

ssh_exec() {
  local key_args=()
  [[ -n "$VPS_SSH_KEY" ]] && key_args=(-i "$VPS_SSH_KEY")
  ssh "${key_args[@]}" \
    -o StrictHostKeyChecking=accept-new \
    -o BatchMode=yes \
    "${VPS_USER}@${VPS_HOST}" "$@"
}

log "Iniciando deploy — tag: ${IMAGE_TAG}"

log "Atualizando código no servidor"
ssh_exec "cd ${VPS_DEPLOY_PATH} && git pull --ff-only"

if [[ -n "${GHCR_TOKEN}" ]]; then
  log "Autenticando no GHCR"
  ssh_exec "echo '${GHCR_TOKEN}' | docker login ghcr.io -u '${GHCR_USER:-x-access-token}' --password-stdin"
fi

log "Capturando imagem anterior para rollback"
PREVIOUS_TAG=$(ssh_exec "docker inspect mecontrola-server-1 --format '{{index .Config.Image}}' 2>/dev/null | sed 's/.*://' || echo ''")
log "Imagem anterior: ${PREVIOUS_TAG:-<nenhuma>}"

log "Fazendo pull da nova imagem"
ssh_exec "IMAGE_TAG=${IMAGE_TAG} docker compose ${COMPOSE_ENV} ${COMPOSE_FILES} pull server worker"

log "Garantindo otelcol ativo"
ssh_exec "docker compose ${COMPOSE_ENV} ${COMPOSE_FILES} up -d otelcol"
for i in $(seq 1 $OTELCOL_RETRIES); do
  OTELCOL_HEALTH=$(ssh_exec "docker inspect --format='{{.State.Health.Status}}' mecontrola-otelcol-1 2>/dev/null || echo 'unknown'")
  if [[ "$OTELCOL_HEALTH" == "healthy" ]]; then
    log "otelcol saudável após $((i * OTELCOL_INTERVAL))s"
    break
  fi
  if [[ "$i" -eq "$OTELCOL_RETRIES" ]]; then
    log "AVISO: otelcol não ficou saudável após $((OTELCOL_RETRIES * OTELCOL_INTERVAL))s (status: ${OTELCOL_HEALTH}) — continuando deploy"
    break
  fi
  log "Aguardando otelcol... (${i}/${OTELCOL_RETRIES}) status: ${OTELCOL_HEALTH}"
  sleep "$OTELCOL_INTERVAL"
done

if [[ -n "$STAGING_SMOKE_WA" ]]; then
  SMOKE_WA_DIGITS="${STAGING_SMOKE_WA#+}"
  log "Configurando app.smoke_wa na VPS (smoke user seed)"
  ssh_exec "docker compose ${COMPOSE_ENV} ${COMPOSE_FILES} exec -T postgres \
    psql -U mecontrola -d mecontrola_db -c \
    \"ALTER DATABASE mecontrola_db SET app.smoke_wa = '${SMOKE_WA_DIGITS}';\"" || \
    log "AVISO: não foi possível configurar app.smoke_wa — smoke user não será semeado"
fi

log "Executando migrações"
ssh_exec "IMAGE_TAG=${IMAGE_TAG} docker compose ${COMPOSE_ENV} ${COMPOSE_FILES} run --rm --no-deps migrate" || {
  log "ERRO: migrações falharam — abortando deploy"
  exit 1
}

log "Atualizando containers server e worker"
ssh_exec "IMAGE_TAG=${IMAGE_TAG} docker compose ${COMPOSE_ENV} ${COMPOSE_FILES} up -d --no-deps server worker"

log "Aguardando healthcheck do container server"
for i in $(seq 1 $HEALTHZ_RETRIES); do
  HEALTH=$(ssh_exec "docker inspect --format='{{.State.Health.Status}}' mecontrola-server-1 2>/dev/null || echo 'unknown'")
  if [[ "$HEALTH" == "healthy" ]]; then
    log "Healthcheck OK após $((i * HEALTHZ_INTERVAL))s"
    break
  fi
  if [[ "$i" -eq "$HEALTHZ_RETRIES" ]]; then
    log "ERRO: healthcheck falhou após $((HEALTHZ_RETRIES * HEALTHZ_INTERVAL))s (status: ${HEALTH}) — iniciando rollback"
    if [[ -n "$PREVIOUS_TAG" && "$PREVIOUS_TAG" != "$IMAGE_TAG" ]]; then
      log "Revertendo para imagem anterior: ${PREVIOUS_TAG}"
      ssh_exec "IMAGE_TAG=${PREVIOUS_TAG} docker compose ${COMPOSE_ENV} ${COMPOSE_FILES} up -d --no-deps server worker" || true
    else
      log "AVISO: sem imagem anterior para rollback — containers permanecem com tag atual"
    fi
    exit 1
  fi
  log "Aguardando... (${i}/${HEALTHZ_RETRIES}) status: ${HEALTH}"
  sleep "$HEALTHZ_INTERVAL"
done

log "Limpando imagens antigas"
ssh_exec "docker image prune -f --filter 'until=72h'" || true

log "Deploy concluído — ${IMAGE_TAG}"
