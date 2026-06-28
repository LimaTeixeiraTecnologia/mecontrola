#!/usr/bin/env bash
set -euo pipefail

# deploy-local.sh — Deploy direto da máquina local para a VPS, sem registry (GHCR).
#
# Replica o fluxo manual: build local (linux/amd64) -> transferência da imagem via
# `docker save | ssh docker load` -> sync do repo no host -> migrations -> server+worker
# -> healthcheck com rollback automático -> verificação pós-deploy.
#
# Uso:
#   bash deployment/scripts/deploy-local.sh [IMAGE_TAG]
#
# IMAGE_TAG default = short SHA do HEAD. Overrides via env:
#   VPS_HOST VPS_USER VPS_DEPLOY_PATH VPS_SSH_KEY IMAGE_NAME PLATFORM
#   HEALTH_RETRIES HEALTH_INTERVAL ALLOW_DIRTY SKIP_BUILD
#
# Requer: docker local, git, acesso SSH por chave à VPS (BatchMode).

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

IMAGE_TAG="${1:-${IMAGE_TAG:-$(git rev-parse --short HEAD)}}"
IMAGE_NAME="${IMAGE_NAME:-ghcr.io/limateixeiratecnologia/mecontrola}"
PLATFORM="${PLATFORM:-linux/amd64}"
DOCKERFILE="${DOCKERFILE:-deployment/docker/Dockerfile}"
IMAGE_REF="${IMAGE_NAME}:${IMAGE_TAG}"

VPS_HOST="${VPS_HOST:-187.77.45.48}"
VPS_USER="${VPS_USER:-root}"
VPS_DEPLOY_PATH="${VPS_DEPLOY_PATH:-/opt/mecontrola}"
VPS_SSH_KEY="${VPS_SSH_KEY:-}"

HEALTH_RETRIES="${HEALTH_RETRIES:-24}"
HEALTH_INTERVAL="${HEALTH_INTERVAL:-5}"
ALLOW_DIRTY="${ALLOW_DIRTY:-false}"
SKIP_BUILD="${SKIP_BUILD:-false}"

SSH_OPTS=(-o BatchMode=yes -o StrictHostKeyChecking=accept-new -o ConnectTimeout=10)
[[ -n "$VPS_SSH_KEY" ]] && SSH_OPTS+=(-i "$VPS_SSH_KEY")

log() { echo "[$(date -u +%H:%M:%SZ)] $*"; }
die() { echo "[ERRO] $*" >&2; exit 1; }
remote() { ssh "${SSH_OPTS[@]}" "${VPS_USER}@${VPS_HOST}" "$@"; }

command -v docker >/dev/null || die "docker não encontrado localmente"
command -v git >/dev/null || die "git não encontrado"
docker info >/dev/null 2>&1 || die "docker daemon indisponível"

log "Checando SSH ${VPS_USER}@${VPS_HOST}"
remote 'true' || die "SSH para a VPS falhou (verifique VPS_SSH_KEY / host)"

if [[ -n "$(git status --porcelain)" && "$ALLOW_DIRTY" != "true" ]]; then
  die "árvore de trabalho suja: a tag ${IMAGE_TAG} não refletiria o commit. Commit/stash, ou rode com ALLOW_DIRTY=true."
fi

log "Deploy local -> VPS | tag=${IMAGE_TAG} platform=${PLATFORM}"

if [[ "$SKIP_BUILD" != "true" ]]; then
  log "1/5 build ${IMAGE_REF}"
  docker build --platform "$PLATFORM" --file "$DOCKERFILE" \
    --tag "$IMAGE_REF" --build-arg VERSION="$IMAGE_TAG" .
else
  log "1/5 build pulado (SKIP_BUILD=true)"
fi

log "2/5 transferindo imagem para a VPS (docker save | ssh docker load)"
docker save "$IMAGE_REF" | gzip -1 | ssh "${SSH_OPTS[@]}" "${VPS_USER}@${VPS_HOST}" 'gunzip | docker load'
remote "docker image inspect ${IMAGE_REF} --format 'imagem na VPS: {{.Architecture}}'" \
  || die "imagem ${IMAGE_REF} não carregou na VPS"

log "3/5 deploy Swarm no host"
ssh "${SSH_OPTS[@]}" "${VPS_USER}@${VPS_HOST}" \
  STACK=mecontrola IMAGE_NAME="$IMAGE_NAME" IMAGE_TAG="$IMAGE_TAG" \
  VPS_HOST=localhost VPS_USER="$(whoami)" VPS_DEPLOY_PATH="$VPS_DEPLOY_PATH" \
  HEALTH_RETRIES="$HEALTH_RETRIES" HEALTH_INTERVAL="$HEALTH_INTERVAL" \
  bash -s -- <<'REMOTE'
set -euo pipefail
DP="${VPS_DEPLOY_PATH}"
TAG="${IMAGE_TAG}"
STACK="${STACK:-mecontrola}"

log() { echo "[$(date -u +%Y-%m-%dT%H:%M:%SZ)] $*"; }

SWARM_STATE=$(docker info --format '{{.Swarm.LocalNodeState}}' 2>/dev/null || echo unknown)
if [ "$SWARM_STATE" != "active" ]; then
  echo "[vps] ERRO: Docker Swarm não está ativo (estado: $SWARM_STATE)"
  exit 1
fi

git config --global --add safe.directory "$DP" 2>/dev/null || true
( cd "$DP" && git pull --ff-only ) || { echo "[vps] ERRO: git pull falhou"; exit 1; }

chmod 600 "$DP/.env"

echo "[vps] criando/atualizando Docker secrets"
( cd "$DP" && bash deployment/scripts/create-secrets.sh "$DP/.env" )

if grep -qE '^AWS_ACCESS_KEY_ID=[^[:space:]]' "$DP/.env" && grep -qE '^AWS_SECRET_ACCESS_KEY=[^[:space:]]' "$DP/.env"; then
  echo "[vps] backup do .env para S3"
  ( cd "$DP" && bash deployment/scripts/backup-env-s3.sh "$DP/.env" )
fi

echo "[vps] migrate via docker run --rm"
docker run --rm \
  --network "${STACK}_backend" \
  --env-file "$DP/.env" \
  -e ENVIRONMENT=production \
  -e DB_HOST=postgres \
  -e DB_PORT=5432 \
  -e OTEL_EXPORTER_OTLP_ENDPOINT=otel-lgtm:4317 \
  -e OTEL_EXPORTER_OTLP_PROTOCOL=grpc \
  -e OTEL_EXPORTER_OTLP_INSECURE=true \
  --name "${STACK}-migrate-${TAG}" \
  "${IMAGE_NAME}:${TAG}" \
  migrate || { echo "[vps] ERRO: migrations falharam — abortando"; exit 1; }

echo "[vps] renderizando stack Swarm"
( cd "$DP" && python3 deployment/scripts/render-stack.py "$DP/.env" deployment/compose/compose.swarm.yml > "/tmp/${STACK}-stack-rendered.yml" )

echo "[vps] docker stack deploy"
docker stack deploy -c "/tmp/${STACK}-stack-rendered.yml" "$STACK"

echo "[vps] aguardando health checks"
ok=false
for i in $(seq 1 "${HEALTH_RETRIES}"); do
  s1=$(docker ps --filter name="${STACK}_server-1" --filter health=healthy --format '{{.Names}}' | head -n1 || true)
  s2=$(docker ps --filter name="${STACK}_server-2" --filter health=healthy --format '{{.Names}}' | head -n1 || true)
  w1=$(docker ps --filter name="${STACK}_worker-1" --filter health=healthy --format '{{.Names}}' | head -n1 || true)
  w2=$(docker ps --filter name="${STACK}_worker-2" --filter health=healthy --format '{{.Names}}' | head -n1 || true)
  if [ -n "$s1" ] && [ -n "$s2" ] && [ -n "$w1" ] && [ -n "$w2" ]; then
    ok=true
    echo "[vps] todos os services saudáveis após $((i * HEALTH_INTERVAL))s"
    break
  fi
  echo "[vps] aguardando ($i/${HEALTH_RETRIES}) s1=${s1:--} s2=${s2:--} w1=${w1:--} w2=${w2:--}"
  sleep "${HEALTH_INTERVAL}"
done

if [ "$ok" != true ]; then
  echo "[vps] healthcheck FALHOU — consulte logs e execute rollback manual conforme deployment/runbooks/rollback.md"
  exit 1
fi

echo "[vps] alinhando IMAGE_TAG no .env"
if grep -q '^IMAGE_TAG=' "$DP/.env"; then
  sed -i.bak-deploylocal "s/^IMAGE_TAG=.*/IMAGE_TAG=$TAG/" "$DP/.env"
fi

echo "[vps] === verificação pós-deploy ==="
docker stack ps "$STACK" --format '[vps] {{.Name}} {{.CurrentState}}' | grep -E 'server-|worker-'
docker service ls --format '[vps] {{.Name}} {{.Replicas}}' | grep -E "${STACK}_(server|worker)"
echo -n "[vps] HEAD host: "; git -C "$DP" rev-parse --short HEAD
docker image prune -f --filter 'until=72h' >/dev/null 2>&1 || true
echo "[vps] OK"
REMOTE

log "5/5 deploy concluído — ${IMAGE_TAG} em produção e saudável"
