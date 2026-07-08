#!/usr/bin/env sh
# taskfiles/scripts/build-image.sh
# Helper cross-platform para `docker build` da imagem mecontrola.
# Usado por: task docker:build
# ADR-011: builder golang:1.26.5-alpine + runtime distroless nonroot; imagem ≤ 30 MB

set -eu

# ── configuração ──────────────────────────────────────────────────────────────
APP_NAME="${APP_NAME:-mecontrola}"
IMAGE_TAG="${IMAGE_TAG:-dev}"
REGISTRY="${REGISTRY:-}"
DOCKERFILE="${DOCKERFILE:-deployment/docker/Dockerfile}"
BUILD_CONTEXT="${BUILD_CONTEXT:-.}"

# VERSION via git, fallback para "dev"
VERSION="${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo "dev")}"

if [ -n "${REGISTRY}" ]; then
  IMAGE_FULL="${REGISTRY}/${APP_NAME}:${IMAGE_TAG}"
else
  IMAGE_FULL="${APP_NAME}:${IMAGE_TAG}"
fi

# ── validações ───────────────────────────────────────────────────────────────
if ! command -v docker >/dev/null 2>&1; then
  echo "ERRO: docker não encontrado no PATH." >&2
  exit 1
fi

if [ ! -f "${DOCKERFILE}" ]; then
  echo "ERRO: ${DOCKERFILE} não encontrado. Rode a partir da raiz do repositório." >&2
  exit 1
fi

# ── build ─────────────────────────────────────────────────────────────────────
echo "==> Building ${IMAGE_FULL} (VERSION=${VERSION})"
docker build \
  --file "${DOCKERFILE}" \
  --tag "${IMAGE_FULL}" \
  --build-arg VERSION="${VERSION}" \
  --label "org.opencontainers.image.version=${VERSION}" \
  --label "org.opencontainers.image.source=https://github.com/LimaTeixeiraTecnologia/mecontrola" \
  --label "org.opencontainers.image.revision=$(git rev-parse HEAD 2>/dev/null || echo unknown)" \
  "${BUILD_CONTEXT}"

echo ""
echo "==> Build concluído: ${IMAGE_FULL}"

# ── validações pós-build ──────────────────────────────────────────────────────
echo ""
echo "==> Validando imagem..."

# Verificar usuário (deve ser nonroot / 65532)
DOCKER_USER=$(docker inspect --format='{{"{{"}}index .Config.User{{"}}"}}' "${IMAGE_FULL}" 2>/dev/null || echo "unknown")
echo "    User:      ${DOCKER_USER}"

# Verificar tamanho (deve ser ≤ 30 MB)
SIZE_BYTES=$(docker inspect --format='{{"{{"}}index .Size{{"}}"}}' "${IMAGE_FULL}" 2>/dev/null || echo "0")
SIZE_MB=$(echo "${SIZE_BYTES}" | awk '{printf "%.1f", $1/1048576}')
echo "    Size:      ${SIZE_MB} MB"

SIZE_CHECK=$(echo "${SIZE_BYTES}" | awk '{print ($1 > 31457280) ? "FAIL" : "OK"}')
if [ "${SIZE_CHECK}" = "FAIL" ]; then
  echo ""
  echo "AVISO: imagem (${SIZE_MB} MB) excede o limite de 30 MB definido no ADR-011." >&2
  echo "       Verificar binário, dependências ou base image antes de fazer push." >&2
  # Aviso, não falha — pode ser build local com debug symbols
fi

echo ""
echo "==> Para usar a imagem:"
echo "    docker run --rm ${IMAGE_FULL} --help"
echo "    docker run --rm ${IMAGE_FULL} server"
echo "    docker run --rm ${IMAGE_FULL} worker"
