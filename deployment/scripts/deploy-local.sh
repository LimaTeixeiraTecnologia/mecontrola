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

log "3/5 migrate + 4/5 server/worker + healthcheck (no host)"
ssh "${SSH_OPTS[@]}" "${VPS_USER}@${VPS_HOST}" \
  bash -s -- "$IMAGE_TAG" "$VPS_DEPLOY_PATH" "$HEALTH_RETRIES" "$HEALTH_INTERVAL" <<'REMOTE'
set -uo pipefail
TAG="$1"; DP="$2"; RETRIES="$3"; INTERVAL="$4"
CF="-f $DP/deployment/compose/compose.yml -f $DP/deployment/compose/compose.prod.yml"
CE="--env-file $DP/.env"
dc() { IMAGE_TAG="$TAG" docker compose $CE $CF "$@"; }

PREV=$(docker inspect mecontrola-server-1 --format '{{.Config.Image}}' 2>/dev/null | sed 's/.*://' || echo '')
echo "[vps] rollback target: ${PREV:-<nenhuma>}"

git config --global --add safe.directory "$DP" 2>/dev/null || true
( cd "$DP" && git pull --ff-only ) || echo "[vps] AVISO: git pull falhou — seguindo (binário e migrations vêm da imagem)"

echo "[vps] migrate"
( cd "$DP" && dc run --rm --no-deps migrate ) || { echo "[vps] ERRO: migrations falharam — abortando (containers intactos)"; exit 1; }

echo "[vps] up server worker"
( cd "$DP" && dc up -d --no-deps server worker ) || { echo "[vps] ERRO: up server/worker falhou"; exit 1; }

ok=false
for i in $(seq 1 "$RETRIES"); do
  H=$(docker inspect --format='{{.State.Health.Status}}' mecontrola-server-1 2>/dev/null || echo unknown)
  W=$(docker inspect --format='{{.State.Health.Status}}' mecontrola-worker-1 2>/dev/null || echo unknown)
  if [ "$H" = healthy ] && [ "$W" = healthy ]; then ok=true; echo "[vps] healthy após $((i * INTERVAL))s"; break; fi
  echo "[vps] aguardando ($i/$RETRIES) server=$H worker=$W"; sleep "$INTERVAL"
done

if [ "$ok" != true ]; then
  echo "[vps] healthcheck FALHOU — iniciando rollback"
  if [ -n "$PREV" ] && [ "$PREV" != "$TAG" ]; then
    ( cd "$DP" && IMAGE_TAG="$PREV" docker compose $CE $CF up -d --no-deps server worker ) || true
    echo "[vps] rollback para $PREV concluído"
  else
    echo "[vps] sem imagem anterior para rollback — containers permanecem em $TAG"
  fi
  exit 1
fi

echo "[vps] === verificação pós-deploy ==="
PU=$(docker exec mecontrola-postgres-1 printenv POSTGRES_USER)
PD=$(docker exec mecontrola-postgres-1 printenv POSTGRES_DB)
echo -n "[vps] schema_migrations (version dirty): "
docker exec mecontrola-postgres-1 psql -U "$PU" -d "$PD" -tAc 'SELECT version, dirty FROM schema_migrations;'
docker ps --format '[vps] {{.Names}} {{.Image}} {{.Status}}' | grep -E 'server|worker'
echo -n "[vps] HEAD host: "; git -C "$DP" rev-parse --short HEAD
docker image prune -f --filter 'until=72h' >/dev/null 2>&1 || true
echo "[vps] OK"
REMOTE

log "5/5 deploy concluído — ${IMAGE_TAG} em produção e saudável"
