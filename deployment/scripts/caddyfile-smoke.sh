#!/usr/bin/env bash
set -euo pipefail

CADDY_IMAGE="${CADDY_IMAGE:-caddy:2-alpine}"
RUN_ID="$$"
NETWORK_NAME="caddy-smoke-net-${RUN_ID}"
CADDY_CONTAINER="caddy-smoke-${RUN_ID}"
APP_CONTAINER="app-smoke-${RUN_ID}"
HOST_PORT="${HOST_PORT:-$((18000 + RANDOM % 1000))}"
PASS=0
FAIL=0
SMOKE_CADDYFILE=""
UPSTREAM_SCRIPT=""

cleanup() {
  docker rm -f "$CADDY_CONTAINER" 2>/dev/null || true
  docker rm -f "$APP_CONTAINER"   2>/dev/null || true
  docker network rm "$NETWORK_NAME" 2>/dev/null || true
  [[ -n "$SMOKE_CADDYFILE" ]] && rm -f "$SMOKE_CADDYFILE" || true
  [[ -n "$UPSTREAM_SCRIPT" ]] && rm -f "$UPSTREAM_SCRIPT" || true
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
if ! VALIDATE_OUTPUT="$(
  docker run --rm \
    -e CADDY_EMAIL=smoke@example.com \
    -e APP_DOMAIN=localhost \
    -e PORT=8080 \
    -v "$CADDYFILE:/etc/caddy/Caddyfile:ro" \
    "$CADDY_IMAGE" \
    caddy validate --config /etc/caddy/Caddyfile --adapter caddyfile 2>&1
)"; then
  echo "$VALIDATE_OUTPUT" | grep -v '"level":"info"' || true
  echo "ERROR: Caddyfile syntax invalid" >&2
  exit 1
fi
echo "$VALIDATE_OUTPUT" | grep -v '"level":"info"' || true
echo "  Caddyfile syntax: valid"

echo "==> Creating isolated Docker network"
docker network create "$NETWORK_NAME" >/dev/null

UPSTREAM_SCRIPT="$(mktemp)"
cat > "$UPSTREAM_SCRIPT" <<'PYEOF'
import json
from http.server import BaseHTTPRequestHandler, HTTPServer

class Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        body = json.dumps(
            {
                "path": self.path,
                "headers": {k.lower(): v for k, v in self.headers.items()},
            }
        ).encode()
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.send_header("X-Smoke-Upstream", "1")
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, format, *args):
        return

HTTPServer(("0.0.0.0", 8080), Handler).serve_forever()
PYEOF

echo "==> Starting stub upstream app (${APP_CONTAINER})"
docker run -d --name "$APP_CONTAINER" \
  --network "$NETWORK_NAME" \
  -v "$UPSTREAM_SCRIPT:/app/echo.py:ro" \
  python:3.12-alpine \
  python /app/echo.py >/dev/null

SMOKE_CADDYFILE="$(mktemp)"
{
  printf '{\nauto_https off\nadmin off\nemail smoke@example.com\n}\n\n'
  tail -n +5 "$CADDYFILE" \
    | sed 's/^{$APP_DOMAIN} {/:80 {/' \
    | sed "s/server:{\$PORT:-8080}/${APP_CONTAINER}:8080/"
} > "$SMOKE_CADDYFILE"

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
check "Permissions-Policy valido"         "echo \"\$HEADERS\" | grep -qi 'Permissions-Policy: camera=(), microphone=(), geolocation=()'"
check "X-Frame-Options: DENY"             "echo \"\$HEADERS\" | grep -qi 'X-Frame-Options: DENY'"
check "Server header stripped"            "! echo \"\$HEADERS\" | grep -qi '^Server:'"

echo ""
echo "==> Smoke: admin/debug/metrics blocked (expect 404, no upstream)"
for path in /admin /debug/pprof /metrics; do
  RESP=$(curl -s -D - -o /dev/null "${BASE}${path}" 2>/dev/null || true)
  STATUS=$(echo "$RESP" | awk 'NR==1{print $2}')
  check "GET ${path} -> 404" "[[ '$STATUS' == '404' ]]"
  check "GET ${path} did not reach upstream" "! echo \"\$RESP\" | grep -qi '^X-Smoke-Upstream:'"
done

echo ""
echo "==> Smoke: positive path reaches upstream (expect 200 + X-Smoke-Upstream)"
PING_RESP=$(curl -s -D - -o /dev/null "${BASE}/api/ping" 2>/dev/null || true)
PING_STATUS=$(echo "$PING_RESP" | awk 'NR==1{print $2}')
check "GET /api/ping -> 200" "[[ '$PING_STATUS' == '200' ]]"
check "GET /api/ping reached upstream" "echo \"\$PING_RESP\" | grep -qi '^X-Smoke-Upstream: 1'"

echo ""
echo "==> Smoke: gateway header strip"
STRIP_BODY=$(curl -s \
  -H "X-User-ID: test-uuid" \
  -H "X-Gateway-Auth: fake" \
  -H "X-Gateway-Timestamp: 123" \
  "${BASE}/headers" 2>/dev/null || true)

check "X-User-ID stripped before upstream"           "! echo \"\$STRIP_BODY\" | grep -qi 'x-user-id'"
check "X-Gateway-Auth stripped before upstream"      "! echo \"\$STRIP_BODY\" | grep -qi 'x-gateway-auth'"
check "X-Gateway-Timestamp stripped before upstream" "! echo \"\$STRIP_BODY\" | grep -qi 'x-gateway-timestamp'"

echo ""
echo "==> Results: $PASS passed, $FAIL failed"

if [[ $FAIL -gt 0 ]]; then
  echo "SMOKE FAILED" >&2
  exit 1
fi

echo "SMOKE PASSED"
