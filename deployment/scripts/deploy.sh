#!/usr/bin/env bash
set -euo pipefail

# deploy.sh — deploy idempotente na VPS via SSH
#
# Uso: ./deployment/scripts/deploy.sh <image-tag>
# Ex:  IMAGE_TAG=abc12345 ./deployment/scripts/deploy.sh abc12345
#
# Variáveis de ambiente obrigatórias:
#   VPS_HOST        — IP ou hostname da VPS
#   VPS_USER        — usuário SSH (padrão: deploy)
#   VPS_SSH_KEY     — caminho para a chave SSH privada
#   VPS_DEPLOY_PATH — caminho do repo na VPS (padrão: /opt/mecontrola)
#
# Pré-requisitos na VPS:
#   - Repo clonado em VPS_DEPLOY_PATH
#   - .env configurado em VPS_DEPLOY_PATH/.env
#   - Docker Engine + Compose v2 instalados

IMAGE_TAG="${1:-${IMAGE_TAG:?IMAGE_TAG must be provided as argument or env var}}"
VPS_HOST="${VPS_HOST:?VPS_HOST is required}"
VPS_USER="${VPS_USER:-deploy}"
VPS_SSH_KEY="${VPS_SSH_KEY:-}"
VPS_DEPLOY_PATH="${VPS_DEPLOY_PATH:-/opt/mecontrola}"
STAGING_SMOKE_WA="${STAGING_SMOKE_WA:-}"

HEALTHZ_RETRIES=12
HEALTHZ_INTERVAL=5

COMPOSE_FILES="-f ${VPS_DEPLOY_PATH}/deployment/compose/compose.yml -f ${VPS_DEPLOY_PATH}/deployment/compose/compose.prod.yml"

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

log "Fazendo pull da nova imagem"
ssh_exec "IMAGE_TAG=${IMAGE_TAG} docker compose ${COMPOSE_FILES} pull server worker"

if [[ -n "$STAGING_SMOKE_WA" ]]; then
  SMOKE_WA_DIGITS="${STAGING_SMOKE_WA#+}"
  log "Configurando app.smoke_wa na VPS (smoke user seed)"
  ssh_exec "docker compose ${COMPOSE_FILES} exec -T postgres \
    psql -U mecontrola -d mecontrola_db -c \
    \"ALTER DATABASE mecontrola_db SET app.smoke_wa = '${SMOKE_WA_DIGITS}';\"" || \
    log "AVISO: não foi possível configurar app.smoke_wa — smoke user não será semeado"
fi

log "Executando migrações"
ssh_exec "IMAGE_TAG=${IMAGE_TAG} docker compose ${COMPOSE_FILES} run --rm migrate" || {
  log "ERRO: migrações falharam — abortando deploy"
  exit 1
}

log "Atualizando containers server e worker"
# Intencional: postgres e pgbouncer excluídos — banco não reinicia em deploys rotineiros.
# Para reiniciar o banco manualmente: docker compose ... restart postgres pgbouncer
ssh_exec "IMAGE_TAG=${IMAGE_TAG} docker compose ${COMPOSE_FILES} up -d --no-deps server worker"

log "Aguardando healthcheck em /health"
APP_URL="http://localhost:8080"
for i in $(seq 1 $HEALTHZ_RETRIES); do
  STATUS=$(ssh_exec "curl -sf -o /dev/null -w '%{http_code}' ${APP_URL}/health || true")
  if [[ "$STATUS" == "200" ]]; then
    log "Healthcheck OK após $((i * HEALTHZ_INTERVAL))s"
    break
  fi
  if [[ "$i" -eq "$HEALTHZ_RETRIES" ]]; then
    log "ERRO: healthcheck falhou após $((HEALTHZ_RETRIES * HEALTHZ_INTERVAL))s — iniciando rollback"
    ssh_exec "docker compose ${COMPOSE_FILES} up -d --no-deps server worker" || true
    exit 1
  fi
  log "Aguardando... (${i}/${HEALTHZ_RETRIES})"
  sleep "$HEALTHZ_INTERVAL"
done

log "Limpando imagens antigas"
ssh_exec "docker image prune -f --filter 'until=72h'" || true

log "Deploy concluído — ${IMAGE_TAG}"
