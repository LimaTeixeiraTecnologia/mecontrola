#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${ROOT_DIR}"

NGROK_API="${NGROK_API:-http://localhost:4040/api/tunnels}"
HEALTH_TIMEOUT="${HEALTH_TIMEOUT:-30}"
COMPOSE="docker compose -f deployment/compose/compose.yml -f deployment/compose/compose.local.yml"

log() { printf '\n[telegram-prepare %s] %s\n' "$(date +%H:%M:%S)" "$*"; }
fail() { log "FAIL: $*"; exit 1; }
pass() { log "PASS: $*"; }

require() { command -v "$1" >/dev/null 2>&1 || fail "binario ausente: $1"; }
require curl
require jq
require docker
require psql
require ngrok

load_env() {
  if [[ ! -f .env ]]; then fail ".env nao encontrado em ${ROOT_DIR}"; fi
  set -a; source .env; set +a
}

check_env() {
  local missing=()
  for var in TELEGRAM_BOT_TOKEN TELEGRAM_BOT_ID TELEGRAM_BOT_USERNAME TELEGRAM_SECRET_TOKEN \
             KIWIFY_WEBHOOK_SECRET KIWIFY_PRODUCT_ID_MONTHLY \
             ONBOARDING_TOKEN_ENCRYPTION_KEY OPENROUTER_API_KEY \
             EMAIL_PROVIDER SMTP_HOST SMTP_PORT EMAIL_ACTIVATE_URL; do
    local val="${!var:-}"
    if [[ -z "$val" || "$val" == *CHANGE_ME* ]]; then
      missing+=("$var")
    fi
  done
  if [[ ${#missing[@]} -gt 0 ]]; then
    fail "variaveis .env faltando: ${missing[*]}"
  fi
}

step_build_check() {
  log "==> 1/8 sanidade do codigo"
  go build ./... >/dev/null
  go vet ./... >/dev/null
  pass "build + vet limpos"
}

step_infra_up() {
  log "==> 2/8 subindo postgres + mailpit + otel-lgtm"
  ${COMPOSE} up -d postgres mailpit otel-lgtm >/dev/null
  log "aguardando postgres healthy..."
  for _ in $(seq 1 30); do
    if ${COMPOSE} ps postgres 2>/dev/null | grep -q "healthy"; then break; fi
    sleep 1
  done
  ${COMPOSE} ps postgres mailpit | head -5 || true
  pass "infra up"
}

step_migrate() {
  log "==> 3/8 rodando migrations"
  go run ./cmd migrate >/tmp/telegram-prepare-migrate.log 2>&1 || \
    { tail -20 /tmp/telegram-prepare-migrate.log; fail "migrate falhou"; }
  pass "migrations aplicadas"
}

step_start_server_worker() {
  log "==> 4/8 iniciando server + worker em background"
  pkill -f "go run ./cmd/server" 2>/dev/null || true
  pkill -f "go run ./cmd/worker" 2>/dev/null || true
  sleep 1
  nohup go run ./cmd server >/tmp/telegram-server.log 2>&1 &
  SERVER_PID=$!
  nohup go run ./cmd worker >/tmp/telegram-worker.log 2>&1 &
  WORKER_PID=$!
  echo "${SERVER_PID}" > /tmp/telegram-server.pid
  echo "${WORKER_PID}" > /tmp/telegram-worker.pid
  log "server pid=${SERVER_PID} worker pid=${WORKER_PID} (logs em /tmp/telegram-{server,worker}.log)"

  log "aguardando server responder em /healthz (timeout ${HEALTH_TIMEOUT}s)..."
  local deadline=$(( $(date +%s) + HEALTH_TIMEOUT ))
  while [[ $(date +%s) -lt ${deadline} ]]; do
    if curl -fsS http://localhost:8080/healthz >/dev/null 2>&1; then
      pass "server healthy"
      return 0
    fi
    sleep 1
  done
  tail -30 /tmp/telegram-server.log || true
  fail "server nao ficou healthy em ${HEALTH_TIMEOUT}s"
}

step_ngrok() {
  log "==> 5/8 subindo ngrok http 8080"
  pkill -f "ngrok http" 2>/dev/null || true
  sleep 1
  nohup ngrok http 8080 --log=stdout >/tmp/telegram-ngrok.log 2>&1 &
  echo "$!" > /tmp/telegram-ngrok.pid
  sleep 3
  local url=""
  for _ in $(seq 1 15); do
    url=$(curl -fsS "${NGROK_API}" 2>/dev/null | jq -r '.tunnels[] | select(.proto=="https") | .public_url' | head -1)
    if [[ -n "$url" && "$url" == https* ]]; then break; fi
    sleep 1
  done
  if [[ -z "$url" || "$url" != https* ]]; then
    tail -30 /tmp/telegram-ngrok.log || true
    fail "ngrok nao expos URL https"
  fi
  echo "$url" > /tmp/telegram-ngrok.url
  pass "ngrok url=${url}"
}

step_register_webhook() {
  log "==> 6/8 registrando webhook Telegram"
  local ngrok_url
  ngrok_url=$(cat /tmp/telegram-ngrok.url)
  local resp
  resp=$(curl -fsS -X POST "https://api.telegram.org/bot${TELEGRAM_BOT_TOKEN}/setWebhook" \
    -d "url=${ngrok_url}/api/v1/channels/telegram/webhook" \
    -d "secret_token=${TELEGRAM_SECRET_TOKEN}" \
    -d "allowed_updates=[\"message\"]" \
    -d "drop_pending_updates=true")
  if [[ "$(echo "$resp" | jq -r '.ok')" != "true" ]]; then
    echo "$resp" | jq .
    fail "setWebhook falhou"
  fi
  curl -fsS "https://api.telegram.org/bot${TELEGRAM_BOT_TOKEN}/getWebhookInfo" | jq '.result | {url, pending_update_count, last_error_message}'
  pass "webhook registrado"
}

step_mailpit_check() {
  log "==> 7/8 checando Mailpit"
  if ! curl -fsS http://localhost:8025/api/v1/info >/dev/null; then
    fail "Mailpit nao responde em http://localhost:8025"
  fi
  pass "Mailpit OK (UI http://localhost:8025)"
}

step_summary() {
  log "==> 8/8 stack pronto"
  local ngrok_url
  ngrok_url=$(cat /tmp/telegram-ngrok.url)
  cat <<EOF

==================================================================
 STACK PRONTO PARA E2E TELEGRAM
==================================================================
 ngrok URL:        ${ngrok_url}
 Telegram bot:     @${TELEGRAM_BOT_USERNAME}
 Server logs:      tail -f /tmp/telegram-server.log
 Worker logs:      tail -f /tmp/telegram-worker.log
 ngrok logs:       tail -f /tmp/telegram-ngrok.log
 Mailpit UI:       http://localhost:8025
 Postgres:         postgres://mecontrola:mecontrola@localhost:5432/mecontrola

 PROXIMO PASSO:
   task mvp:telegram:drive
==================================================================
EOF
}

trap 'log "telegram-prepare concluido"' EXIT

load_env
check_env
step_build_check
step_infra_up
step_migrate
step_start_server_worker
step_ngrok
step_register_webhook
step_mailpit_check
step_summary
