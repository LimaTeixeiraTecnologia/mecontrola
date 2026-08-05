#!/usr/bin/env bash
set -euo pipefail

# deploy-swarm.sh — Deploy da stack Docker Swarm na VPS via SSH.
#
# Uso:
#   bash deployment/scripts/deploy-swarm.sh <IMAGE_TAG> [SECRETS_ENV_FILE] [IMAGE_DIGEST]
#
# Variaveis de ambiente obrigatorias:
#   VPS_HOST, VPS_USER, VPS_DEPLOY_PATH
# Variaveis opcionais:
#   VPS_SSH_KEY — caminho para chave SSH
#   IMAGE_NAME  — registry/repository (default: ghcr.io/limateixeiratecnologia/mecontrola)
#   IMAGE_DIGEST — digest imutavel da imagem (ex.: sha256:...); quando informado,
#                  o compose e as migracoes usam IMAGE_NAME@DIGEST.
#   MODE        — modo do create-secrets.sh (default: rotate; aplica alteracoes automaticamente)
#
# Fluxo:
#   1. git pull --ff-only na VPS
#   2. Copia deployment/config/prod.env e o arquivo de secrets descriptografado
#      para /tmp na VPS (removidos ao final)
#   3. Criar/atualizar Docker secrets (com rotacao automatica)
#   4. Renderizar provisioning de alertas do Grafana
#   5. Executar migrations via docker run --rm (advisory lock)
#   6. Renderizar stack Swarm e docker stack deploy
#   7. Aguardar health checks de server-1/2 e worker-1/2
#   8. Rollback para tag/digest anterior em caso de falha
#
# Seguranca:
#   - Nao ha .env persistente na VPS. Secrets trafegam apenas por /tmp
#     durante o deploy e sao removidos imediatamente.
#   - O arquivo de secrets pode ser o prod.secrets.env descriptografado pelo CI.

IMAGE_TAG="${1:-${IMAGE_TAG:?IMAGE_TAG obrigatorio}}"
SECRETS_ENV_FILE="${2:-${SECRETS_ENV_FILE:-}}"
IMAGE_DIGEST="${3:-${IMAGE_DIGEST:-}}"
LEGACY_ENV_FILE="${VPS_DEPLOY_PATH:-/opt/mecontrola}/.env"

IMAGE_NAME="${IMAGE_NAME:-ghcr.io/limateixeiratecnologia/mecontrola}"
STACK="${STACK:-mecontrola}"
VPS_HOST="${VPS_HOST:?VPS_HOST obrigatorio}"
VPS_USER="${VPS_USER:?VPS_USER obrigatorio}"
VPS_DEPLOY_PATH="${VPS_DEPLOY_PATH:?VPS_DEPLOY_PATH obrigatorio}"
VPS_SSH_KEY="${VPS_SSH_KEY:-}"
MODE="${MODE:-rotate}"

PROD_ENV_FILE="${PROD_ENV_FILE:-deployment/config/prod.env}"

HEALTH_RETRIES="${HEALTH_RETRIES:-24}"
HEALTH_INTERVAL="${HEALTH_INTERVAL:-5}"
MIGRATE_TIMEOUT="${MIGRATE_TIMEOUT:-300}"

if [[ -n "$IMAGE_DIGEST" ]]; then
  IMAGE_REF="${IMAGE_NAME}@${IMAGE_DIGEST}"
else
  IMAGE_REF="${IMAGE_NAME}:${IMAGE_TAG}"
fi

SSH_OPTS=(-o BatchMode=yes -o StrictHostKeyChecking=yes -o ConnectTimeout=10)
[[ -n "$VPS_SSH_KEY" ]] && SSH_OPTS+=(-i "$VPS_SSH_KEY")

# Retry de transporte SSH. A rota entre o runner e a VPS apresenta perda de
# pacote intermitente: o deploy ja falhou com "connect to host: Connection timed
# out" logo na primeira chamada, sem nada ter sido executado remotamente.
SSH_MAX_ATTEMPTS="${SSH_MAX_ATTEMPTS:-5}"
SSH_RETRY_BASE_DELAY="${SSH_RETRY_BASE_DELAY:-5}"
SSH_PREFLIGHT_ATTEMPTS="${SSH_PREFLIGHT_ATTEMPTS:-6}"

log() { echo "[$(date -u +%Y-%m-%dT%H:%M:%SZ)] $*"; }
# Logs de retry vao para stderr: varias chamadas sao consumidas por command
# substitution e nao podem ter a saida poluida.
log_err() { echo "[$(date -u +%Y-%m-%dT%H:%M:%SZ)] $*" >&2; }

# Repete apenas quando o ssh falha no transporte (exit 255): timeout de conexao,
# rede indisponivel, host inalcancavel. Erro do comando remoto propaga sem
# retry, para nunca reexecutar efeito colateral por falha de negocio.
ssh_with_retry() {
  local attempt=1 rc=0 delay="$SSH_RETRY_BASE_DELAY"
  while :; do
    # O rc precisa ser capturado no ramo else: apos um `if` sem else, o `$?`
    # passa a ser o status do proprio `if` (0), mascarando a falha.
    if "$@"; then
      return 0
    else
      rc=$?
    fi
    if [[ "$rc" -ne 255 ]] || [[ "$attempt" -ge "$SSH_MAX_ATTEMPTS" ]]; then
      return "$rc"
    fi
    log_err "SSH indisponivel (exit 255) — tentativa ${attempt}/${SSH_MAX_ATTEMPTS}; repetindo em ${delay}s"
    sleep "$delay"
    attempt=$((attempt + 1))
    delay=$((delay * 2))
  done
}

ssh_raw() { ssh "${SSH_OPTS[@]}" "${VPS_USER}@${VPS_HOST}" "$@"; }
ssh_exec() { ssh_with_retry ssh_raw "$@"; }

# O corpo do script remoto vem por stdin (heredoc). Precisa ser bufferizado
# antes do retry, senao a segunda tentativa enviaria stdin vazio.
ssh_script_once() {
  local payload="$1"
  shift
  printf '%s' "$payload" | ssh "${SSH_OPTS[@]}" "${VPS_USER}@${VPS_HOST}" bash -s -- "$@"
}
ssh_script() {
  local payload
  payload="$(cat)"
  ssh_with_retry ssh_script_once "$payload" "$@"
}

scp_raw() { scp "${SSH_OPTS[@]}" "$1" "${VPS_USER}@${VPS_HOST}:$2"; }

upload_file() {
  local src="$1" dst="$2"
  if [[ "${LOCAL_DEPLOY:-false}" == "true" ]]; then
    cp "$src" "$dst"
    chmod 600 "$dst"
  else
    ssh_with_retry scp_raw "$src" "$dst"
    ssh_exec "chmod 600 '${dst}'"
  fi
}

# Preflight: nao adianta iniciar o deploy se a VPS esta inalcancavel. Falha aqui
# e barata e nao deixa estado remoto pela metade.
wait_for_ssh() {
  [[ "${LOCAL_DEPLOY:-false}" == "true" ]] && return 0
  local attempt=1 delay="$SSH_RETRY_BASE_DELAY"
  while :; do
    if ssh_raw true >/dev/null 2>&1; then
      [[ "$attempt" -gt 1 ]] && log "SSH disponivel apos ${attempt} tentativas"
      return 0
    fi
    if [[ "$attempt" -ge "$SSH_PREFLIGHT_ATTEMPTS" ]]; then
      log "ERRO: VPS inalcancavel por SSH apos ${SSH_PREFLIGHT_ATTEMPTS} tentativas"
      return 1
    fi
    log "VPS ainda inalcancavel por SSH — tentativa ${attempt}/${SSH_PREFLIGHT_ATTEMPTS}; repetindo em ${delay}s"
    sleep "$delay"
    attempt=$((attempt + 1))
    delay=$((delay * 2))
  done
}

log "Iniciando deploy Swarm — tag: ${IMAGE_TAG}${IMAGE_DIGEST:+, digest: ${IMAGE_DIGEST}}"

if [[ -z "$SECRETS_ENV_FILE" ]]; then
  if [[ -f ".env" ]]; then
    log "AVISO: usando .env local (legado). Prefira passar o arquivo de secrets descriptografado."
    SECRETS_ENV_FILE=".env"
  elif [[ -f "$LEGACY_ENV_FILE" ]]; then
    log "AVISO: usando ${LEGACY_ENV_FILE} na VPS (legado)."
    SECRETS_ENV_FILE="$LEGACY_ENV_FILE"
  else
    log "ERRO: arquivo de secrets nao informado e nenhum .env encontrado"
    exit 1
  fi
fi

[[ -f "$SECRETS_ENV_FILE" ]] || { log "ERRO: arquivo de secrets nao encontrado: $SECRETS_ENV_FILE"; exit 1; }
[[ -f "$PROD_ENV_FILE" ]] || { log "ERRO: arquivo de config nao encontrado: $PROD_ENV_FILE"; exit 1; }

log "Verificando conectividade SSH com a VPS"
wait_for_ssh || exit 1

log "Verificando estado do Swarm na VPS"
SWARM_STATE=$(ssh_exec "docker info --format '{{.Swarm.LocalNodeState}}' 2>/dev/null || echo unknown")
if [[ "$SWARM_STATE" != "active" ]]; then
  log "ERRO: Docker Swarm nao esta ativo (estado: $SWARM_STATE)"
  exit 1
fi

if [[ -n "${GITHUB_TOKEN:-}" ]]; then
  log "Autenticando VPS no GHCR com GITHUB_TOKEN"
  ssh_exec "echo '${GITHUB_TOKEN}' | docker login ghcr.io -u '${GITHUB_ACTOR:-github-actions}' --password-stdin" || {
    log "ERRO: docker login na VPS falhou"
    exit 1
  }
fi

log "Capturando imagem anterior para rollback"
PREVIOUS_IMAGE=$(ssh_exec "docker service inspect ${STACK}_server-1 --format '{{.Spec.TaskTemplate.ContainerSpec.Image}}' 2>/dev/null || echo")
PREVIOUS_TAG=""
PREVIOUS_DIGEST=""
if [[ "$PREVIOUS_IMAGE" == *"@sha256:"* ]]; then
  PREVIOUS_DIGEST="${PREVIOUS_IMAGE#*@}"
  BEFORE_AT="${PREVIOUS_IMAGE%%@*}"
  if [[ "$BEFORE_AT" == *":"* ]]; then
    PREVIOUS_TAG="${BEFORE_AT##*:}"
  fi
elif [[ "$PREVIOUS_IMAGE" == *":"* ]]; then
  PREVIOUS_TAG="${PREVIOUS_IMAGE##*:}"
fi
log "Imagem anterior: ${PREVIOUS_IMAGE:-<nenhuma>} (tag=${PREVIOUS_TAG:-<nenhuma>}, digest=${PREVIOUS_DIGEST:-<nenhuma>})"

log "Atualizando codigo na VPS"
ssh_exec "git config --global --add safe.directory ${VPS_DEPLOY_PATH} 2>/dev/null || true"
if [[ "${SKIP_GIT_PULL:-0}" == "1" ]]; then
  log "SKIP_GIT_PULL=1 — pulando git pull (assume codigo ja sincronizado)"
elif ! ssh_exec "cd ${VPS_DEPLOY_PATH} && git pull --ff-only"; then
  log "ERRO: git pull falhou — abortando deploy (use SKIP_GIT_PULL=1 se o codigo ja foi sincronizado)"
  exit 1
fi

REMOTE_PROD_ENV="/tmp/mecontrola-prod.env.$$"
REMOTE_SECRETS_ENV="/tmp/mecontrola-secrets.env.$$"
REMOTE_RENDERED_STACK="/tmp/mecontrola-stack-rendered.yml.$$"
trap 'ssh_exec "rm -f $REMOTE_PROD_ENV $REMOTE_SECRETS_ENV $REMOTE_RENDERED_STACK" >/dev/null 2>&1 || true' EXIT

log "Enviando arquivos de configuracao para /tmp na VPS"
upload_file "$PROD_ENV_FILE" "$REMOTE_PROD_ENV"
upload_file "$SECRETS_ENV_FILE" "$REMOTE_SECRETS_ENV"

log "Criando/atualizando Docker secrets (modo: ${MODE})"
ssh_exec "cd ${VPS_DEPLOY_PATH} && MODE='${MODE}' bash deployment/scripts/create-secrets.sh '$REMOTE_SECRETS_ENV'"

log "Renderizando provisioning de alertas do Grafana"
ssh_exec "cd ${VPS_DEPLOY_PATH} && bash deployment/scripts/setup-grafana-alerts.sh '$REMOTE_SECRETS_ENV'"

log "Executando migrations (docker run --rm com advisory lock)"
ssh_script "$REMOTE_SECRETS_ENV" "$REMOTE_PROD_ENV" "$STACK" "$IMAGE_REF" "$MIGRATE_TIMEOUT" <<'MIGRATE'
set -euo pipefail
REMOTE_SECRETS_ENV="$1"
REMOTE_PROD_ENV="$2"
STACK="$3"
IMAGE_REF="$4"
MIGRATE_TIMEOUT="$5"
migrate_env="/tmp/mecontrola-migrate.env.$$"
trap 'rm -f "$migrate_env"' EXIT
cat "$REMOTE_SECRETS_ENV" > "$migrate_env"
cat "$REMOTE_PROD_ENV" >> "$migrate_env"
docker run --rm \
  --network "${STACK}_backend" \
  --env-file "$migrate_env" \
  -e ENVIRONMENT=production \
  -e DB_HOST=postgres \
  -e DB_PORT=5432 \
  -e OTEL_EXPORTER_OTLP_ENDPOINT=otel-lgtm:4317 \
  -e OTEL_EXPORTER_OTLP_PROTOCOL=grpc \
  -e OTEL_EXPORTER_OTLP_INSECURE=true \
  --stop-timeout "${MIGRATE_TIMEOUT}" \
  --name "${STACK}-migrate-$(date +%s)" \
  "$IMAGE_REF" \
  migrate
MIGRATE

log "Renderizando stack Swarm a partir do compose"
ssh_exec "cd ${VPS_DEPLOY_PATH} && IMAGE_TAG='${IMAGE_TAG}' IMAGE_DIGEST='${IMAGE_DIGEST}' python3 deployment/scripts/render-stack.py deployment/compose/compose.swarm.yml --env-file '$REMOTE_PROD_ENV' --secrets-env-file '$REMOTE_SECRETS_ENV' > '$REMOTE_RENDERED_STACK'"

log "Fazendo deploy da stack Swarm"
ssh_exec "docker stack deploy --resolve-image=always -c '$REMOTE_RENDERED_STACK' ${STACK}"

wait_service_running() {
  local svc="$1"
  local retries="${2:-$HEALTH_RETRIES}"
  local interval="${3:-$HEALTH_INTERVAL}"
  for i in $(seq 1 "$retries"); do
    local state
    state=$(ssh_exec "docker service ps ${STACK}_${svc} --format '{{.CurrentState}}' 2>/dev/null | head -n1 || echo unknown")
    if [[ "$state" == Running* ]]; then
      log "${svc} em execucao apos $((i * interval))s"
      return 0
    fi
    log "Aguardando ${svc}... (${i}/${retries}) estado: ${state}"
    sleep "$interval"
  done
  log "ERRO: ${svc} nao ficou em execucao apos $((retries * interval))s"
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
      log "${svc} saudavel (container: ${healthy})"
      return 0
    fi
    log "Aguardando health check ${svc}... (${i}/${retries})"
    sleep "$interval"
  done
  log "ERRO: ${svc} nao ficou saudavel apos $((retries * interval))s"
  return 1
}

rollback() {
  local failed_svc="$1"
  log "ERRO: deploy falhou em ${failed_svc} — iniciando rollback"
  if [[ -n "$PREVIOUS_IMAGE" && "$PREVIOUS_IMAGE" != "$IMAGE_REF" ]]; then
    log "Revertendo para imagem anterior: ${PREVIOUS_IMAGE}"
    ssh_exec "cd ${VPS_DEPLOY_PATH} && IMAGE_TAG='${PREVIOUS_TAG}' IMAGE_DIGEST='${PREVIOUS_DIGEST}' python3 deployment/scripts/render-stack.py deployment/compose/compose.swarm.yml --env-file '$REMOTE_PROD_ENV' --secrets-env-file '$REMOTE_SECRETS_ENV' > '$REMOTE_RENDERED_STACK' && docker stack deploy --resolve-image=always -c '$REMOTE_RENDERED_STACK' ${STACK}; rm -f '$REMOTE_RENDERED_STACK'"
  else
    log "AVISO: sem imagem anterior para rollback"
  fi
  exit 1
}

log "Aguardando services entrarem em execucao"
for svc in server-1 server-2 worker-1 worker-2; do
  wait_service_running "$svc" || rollback "$svc"
done

log "Aguardando health checks dos app services"
for svc in server-1 server-2 worker-1 worker-2; do
  wait_service_healthy "$svc" || rollback "$svc"
done

log "Limpando imagens antigas"
ssh_exec "docker image prune -f --filter 'until=72h'" || true

log "Deploy Swarm concluido — ${IMAGE_REF}"
