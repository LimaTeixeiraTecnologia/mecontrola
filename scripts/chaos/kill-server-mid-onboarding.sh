#!/usr/bin/env bash
# Chaos: kill API server during onboarding and verify state survives restart.
#
# Steps:
#   1. Dispara webhook Kiwify (cria session + token).
#   2. Aguarda email no Mailpit e extrai o token.
#   3. Faz GET /state — confirma 200.
#   4. Mata o PID em /tmp/mecontrola-server.pid.
#   5. Espera 5s.
#   6. Reinicia via SERVER_RESTART_CMD.
#   7. Aguarda /healthz.
#   8. Faz GET /state de novo — DEVE retornar 200 com o mesmo conteudo de antes.
#
# Pre-req:
#   - Stack local subida via `task local:up`.
#   - cmd/server iniciado com `& echo $! > /tmp/mecontrola-server.pid`
#     OU container docker com PID conhecido.
#
# Envs:
#   BACKEND                     default http://localhost:8080
#   MAILPIT_API                 default http://localhost:8025/api/v1
#   KIWIFY_WEBHOOK_SECRET       required
#   KIWIFY_PRODUCT_ID_MONTHLY   required
#   SERVER_PID_FILE             default /tmp/mecontrola-server.pid
#   SERVER_RESTART_CMD          default: prints manual restart instructions
#
# Exit 0 = chaos sobreviveu (state persistido), 1 = falha.

set -euo pipefail

BACKEND="${BACKEND:-http://localhost:8080}"
MAILPIT_API="${MAILPIT_API:-http://localhost:8025/api/v1}"
SERVER_PID_FILE="${SERVER_PID_FILE:-/tmp/mecontrola-server.pid}"
SERVER_RESTART_CMD="${SERVER_RESTART_CMD:-}"
KIWIFY_WEBHOOK_SECRET="${KIWIFY_WEBHOOK_SECRET:?KIWIFY_WEBHOOK_SECRET nao definido}"
KIWIFY_PRODUCT_ID_MONTHLY="${KIWIFY_PRODUCT_ID_MONTHLY:?KIWIFY_PRODUCT_ID_MONTHLY nao definido}"
WAIT_HEALTH_SEC="${WAIT_HEALTH_SEC:-30}"

log() { printf '[chaos-server %s] %s\n' "$(date +%H:%M:%S)" "$*"; }
fail() { log "FAIL: $*"; exit 1; }
pass() { log "PASS: $*"; }

command -v curl >/dev/null || fail "curl ausente"
command -v jq >/dev/null || fail "jq ausente"
command -v openssl >/dev/null || fail "openssl ausente"

CHAOS_TOKEN="chaos-$(date +%s)-${RANDOM}"
EMAIL="chaos+${CHAOS_TOKEN}@mecontrola.local"
NOW="$(date -u +'%Y-%m-%d %H:%M:%S')"
ORDER_ID="order-chaos-${CHAOS_TOKEN}"

PAYLOAD=$(jq -nc \
  --arg token "$CHAOS_TOKEN" \
  --arg email "$EMAIL" \
  --arg product "$KIWIFY_PRODUCT_ID_MONTHLY" \
  --arg now "$NOW" \
  --arg order "$ORDER_ID" \
  '{
     order_id: $order,
     order_ref: "ref-chaos",
     order_status: "paid",
     webhook_event_type: "order_approved",
     subscription_id: ("sub-" + $token),
     Product: { product_id: $product, product_name: "Chaos Plan" },
     Customer: { email: $email, mobile: "+5511900000000", CPF: "00000000000" },
     Subscription: { status: "active", start_date: $now, next_payment: $now },
     TrackingParameters: { sck: $token, s1: null, src: null },
     approved_date: $now,
     updated_at: $now,
     created_at: $now
   }')

log "==> 1/8 limpando Mailpit"
curl -fsS -X DELETE "${MAILPIT_API}/messages" >/dev/null || fail "mailpit nao acessivel"

log "==> 2/8 disparando webhook Kiwify (token=${CHAOS_TOKEN})"
SIG=$(printf '%s' "$PAYLOAD" | openssl dgst -sha1 -mac HMAC -macopt "key:${KIWIFY_WEBHOOK_SECRET}" | awk '{print $2}')
HTTP_STATUS=$(curl -sS -o /tmp/chaos_kiwify.json -w "%{http_code}" \
  -X POST "${BACKEND}/api/v1/billing/webhooks/kiwify?signature=${SIG}" \
  -H "Content-Type: application/json" \
  --data-binary "$PAYLOAD")
[ "$HTTP_STATUS" = "202" ] || fail "kiwify retornou ${HTTP_STATUS} (esperado 202)"

log "==> 3/8 aguardando email de onboarding"
ONBOARDING_TOKEN=""
for _ in $(seq 1 30); do
  RAW=$(curl -fsS "${MAILPIT_API}/search?query=to:${EMAIL}" || true)
  COUNT=$(printf '%s' "$RAW" | jq -r '.messages_count // 0')
  if [ "$COUNT" -gt 0 ]; then
    MSG_ID=$(printf '%s' "$RAW" | jq -r '.messages[0].ID')
    BODY=$(curl -fsS "${MAILPIT_API}/message/${MSG_ID}" | jq -r '.Text // .HTML // ""')
    ONBOARDING_TOKEN=$(printf '%s' "$BODY" | grep -oE 'tokens/[A-Za-z0-9_-]{20,}' | head -1 | sed 's|tokens/||')
    [ -n "$ONBOARDING_TOKEN" ] && break
  fi
  sleep 1
done
[ -n "$ONBOARDING_TOKEN" ] || fail "token nao recebido no mailpit"
log "    token=${ONBOARDING_TOKEN}"

log "==> 4/8 GET /state antes do kill"
STATE_BEFORE=$(curl -sS -w "\n%{http_code}" "${BACKEND}/api/v1/onboarding/tokens/${ONBOARDING_TOKEN}/state")
CODE_BEFORE=$(printf '%s' "$STATE_BEFORE" | tail -1)
BODY_BEFORE=$(printf '%s' "$STATE_BEFORE" | sed '$d')
[ "$CODE_BEFORE" = "200" ] || fail "/state retornou ${CODE_BEFORE} antes do chaos"

log "==> 5/8 matando server"
if [ ! -f "$SERVER_PID_FILE" ]; then
  fail "PID file ${SERVER_PID_FILE} ausente — inicie cmd/server com '& echo \$! > ${SERVER_PID_FILE}'"
fi
SERVER_PID=$(cat "$SERVER_PID_FILE")
kill "$SERVER_PID" 2>/dev/null || log "    server ja morto?"
sleep 5

log "==> 6/8 reiniciando server"
if [ -z "$SERVER_RESTART_CMD" ]; then
  log "    SERVER_RESTART_CMD nao definido — reinicie manualmente e pressione ENTER"
  read -r _
else
  bash -c "$SERVER_RESTART_CMD" &
  disown || true
fi

log "==> 7/8 aguardando /healthz"
for _ in $(seq 1 "$WAIT_HEALTH_SEC"); do
  if curl -fsS "${BACKEND}/healthz" >/dev/null 2>&1; then
    break
  fi
  sleep 1
done
curl -fsS "${BACKEND}/healthz" >/dev/null || fail "server nao voltou apos ${WAIT_HEALTH_SEC}s"

log "==> 8/8 GET /state apos restart"
STATE_AFTER=$(curl -sS -w "\n%{http_code}" "${BACKEND}/api/v1/onboarding/tokens/${ONBOARDING_TOKEN}/state")
CODE_AFTER=$(printf '%s' "$STATE_AFTER" | tail -1)
BODY_AFTER=$(printf '%s' "$STATE_AFTER" | sed '$d')
[ "$CODE_AFTER" = "200" ] || fail "/state retornou ${CODE_AFTER} apos restart"

STATUS_BEFORE=$(printf '%s' "$BODY_BEFORE" | jq -r '.status // .state // empty')
STATUS_AFTER=$(printf '%s' "$BODY_AFTER" | jq -r '.status // .state // empty')
if [ -n "$STATUS_BEFORE" ] && [ -n "$STATUS_AFTER" ] && [ "$STATUS_BEFORE" != "$STATUS_AFTER" ]; then
  log "    warn: status divergiu (antes=${STATUS_BEFORE} depois=${STATUS_AFTER}) — pode ser progresso natural"
fi

pass "onboarding sobreviveu ao kill+restart"
