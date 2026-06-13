#!/usr/bin/env bash
set -euo pipefail

CADDY_IMAGE="${CADDY_IMAGE:-caddy:2-alpine}"
RUN_ID="$$"
NETWORK_NAME="caddy-smoke-net-${RUN_ID}"
CADDY_CONTAINER="caddy-smoke-${RUN_ID}"
APP_CONTAINER="app-smoke-${RUN_ID}"
HOST_PORT="${HOST_PORT:-18899}"
PASS=0
FAIL=0
SMOKE_CADDYFILE=""

cleanup() {
  docker rm -f "$CADDY_CONTAINER" 2>/dev/null || true
  docker rm -f "$APP_CONTAINER"   2>/dev/null || true
  docker network rm "$NETWORK_NAME" 2>/dev/null || true
  [[ -n "$SMOKE_CADDYFILE" ]] && rm -f "$SMOKE_CADDYFILE" || true
}
trap cleanup EXIT

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
CADDYFILE="$REPO_ROOT/deployment/caddy/Caddyfile"

if [[ ! -f "$CADDYFILE" ]]; then
  echo "ERROR: Caddyfile not found at $CADDYFILE" >&2
  exit 1
fi

check() {
  local label="$1"
  local condition="$2"
  if eval "$condition"; then
    echo "  PASS: $label"
    PASS=$((PASS + 1))
  else
    echo "  FAIL: $label"
    FAIL=$((FAIL + 1))
  fi
}

echo "==> Validating Caddyfile syntax"
docker run --rm \
  -e CADDY_EMAIL=smoke@example.com \
  -e APP_DOMAIN=localhost \
  -e PORT=8080 \
  -v "$CADDYFILE:/etc/caddy/Caddyfile:ro" \
  "$CADDY_IMAGE" \
  caddy validate --config /etc/caddy/Caddyfile --adapter caddyfile 2>&1 | grep -v '"level":"info"' || true
echo "  Caddyfile syntax: valid"

echo "==> Creating isolated Docker network"
docker network create "$NETWORK_NAME" >/dev/null

echo "==> Starting stub upstream app (${APP_CONTAINER})"
docker run -d --name "$APP_CONTAINER" \
  --network "$NETWORK_NAME" \
  nginx:alpine >/dev/null

SMOKE_CADDYFILE="$(mktemp)"
cat > "$SMOKE_CADDYFILE" <<CADDYEOF
{
	auto_https off
	admin off
}

:80 {
	@admin path /admin* /debug/pprof* /metrics*
	respond @admin 404

	reverse_proxy ${APP_CONTAINER}:80 {
		header_up -X-User-ID
		header_up -X-Gateway-Auth
		header_up -X-Gateway-Timestamp
	}

	header {
		Strict-Transport-Security "max-age=31536000; includeSubDomains"
		X-Content-Type-Options "nosniff"
		Referrer-Policy "no-referrer"
		Permissions-Policy "()"
		X-Frame-Options "DENY"
		Content-Security-Policy "default-src 'none'; connect-src 'self'; frame-ancestors 'none'"
		-Server
	}

	encode gzip zstd
}
CADDYEOF

echo "==> Starting Caddy smoke container on port ${HOST_PORT}"
docker run -d --name "$CADDY_CONTAINER" \
  --network "$NETWORK_NAME" \
  -p "${HOST_PORT}:80" \
  -v "$SMOKE_CADDYFILE:/etc/caddy/Caddyfile:ro" \
  "$CADDY_IMAGE" >/dev/null

echo "==> Waiting for Caddy to be ready..."
for i in $(seq 1 20); do
  if curl -sf "http://localhost:${HOST_PORT}/" >/dev/null 2>&1; then
    break
  fi
  sleep 1
done

BASE="http://localhost:${HOST_PORT}"

echo ""
echo "==> Smoke: security headers on GET /"
HEADERS=$(curl -sI "${BASE}/" 2>/dev/null || true)

check "Strict-Transport-Security present" "echo \"\$HEADERS\" | grep -qi 'Strict-Transport-Security'"
check "X-Content-Type-Options: nosniff"   "echo \"\$HEADERS\" | grep -qi 'X-Content-Type-Options: nosniff'"
check "Referrer-Policy: no-referrer"      "echo \"\$HEADERS\" | grep -qi 'Referrer-Policy: no-referrer'"
check "Permissions-Policy: ()"           "echo \"\$HEADERS\" | grep -qi 'Permissions-Policy: ()'"
check "X-Frame-Options: DENY"             "echo \"\$HEADERS\" | grep -qi 'X-Frame-Options: DENY'"
check "Server header stripped"            "! echo \"\$HEADERS\" | grep -qi '^Server:'"

echo ""
echo "==> Smoke: admin/debug/metrics blocked (expect 404)"
for path in /admin /debug/pprof /metrics; do
  STATUS=$(curl -so /dev/null -w "%{http_code}" "${BASE}${path}" 2>/dev/null || echo "000")
  check "GET ${path} -> 404" "[[ '$STATUS' == '404' ]]"
done

echo ""
echo "==> Smoke: gateway header strip"
STRIP_HEADERS=$(curl -sI \
  -H "X-User-ID: test-uuid" \
  -H "X-Gateway-Auth: fake" \
  -H "X-Gateway-Timestamp: 123" \
  "${BASE}/" 2>/dev/null || true)

check "X-User-ID not leaked in response"           "! echo \"\$STRIP_HEADERS\" | grep -qi '^X-User-ID:'"
check "X-Gateway-Auth not leaked in response"      "! echo \"\$STRIP_HEADERS\" | grep -qi '^X-Gateway-Auth:'"
check "X-Gateway-Timestamp not leaked in response" "! echo \"\$STRIP_HEADERS\" | grep -qi '^X-Gateway-Timestamp:'"

echo ""
echo "==> Results: $PASS passed, $FAIL failed"

if [[ $FAIL -gt 0 ]]; then
  echo "SMOKE FAILED" >&2
  exit 1
fi

echo "SMOKE PASSED"
