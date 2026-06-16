#!/usr/bin/env bash
set -euo pipefail

KIWIFY_URL="${KIWIFY_URL:-http://localhost:8080/api/v1/billing/webhooks/kiwify}"
STATE_URL_BASE="${STATE_URL_BASE:-http://localhost:8080/api/v1/onboarding/tokens}"
MAILPIT_API="${MAILPIT_API:-http://localhost:8025/api/v1}"
KIWIFY_WEBHOOK_SECRET="${KIWIFY_WEBHOOK_SECRET:?KIWIFY_WEBHOOK_SECRET nao definido}"
KIWIFY_WEBHOOK_TOKEN_HEADER="${KIWIFY_WEBHOOK_TOKEN_HEADER:-X-Kiwify-Webhook-Token}"
KIWIFY_PRODUCT_ID_MONTHLY="${KIWIFY_PRODUCT_ID_MONTHLY:?KIWIFY_PRODUCT_ID_MONTHLY nao definido}"
WAIT_MAX_SECONDS="${WAIT_MAX_SECONDS:-30}"
EMAIL_FROM_ADDRESS="${EMAIL_FROM_ADDRESS:-noreply@mecontrola.local}"

log() { printf '[mvp-smoke %s] %s\n' "$(date +%H:%M:%S)" "$*"; }
fail() { log "FAIL: $*"; exit 1; }
pass() { log "PASS: $*"; }

require() {
  command -v "$1" >/dev/null 2>&1 || fail "binario ausente: $1"
}

require curl
require jq

log "==> Step 1/5 — pre-checks"
if ! curl -fsS "${MAILPIT_API}/info" >/dev/null 2>&1; then
  fail "Mailpit nao responde em ${MAILPIT_API}. Suba via 'task local:up'."
fi
if ! curl -fsS "${KIWIFY_URL}" -X OPTIONS -o /dev/null 2>/dev/null; then
  log "  warn: backend nao respondeu OPTIONS — seguindo mesmo assim."
fi

log "==> Step 2/5 — limpando Mailpit"
curl -fsS -X DELETE "${MAILPIT_API}/messages" >/dev/null || fail "nao consegui limpar mailpit"

SMOKE_TOKEN="smoke-token-$(date +%s)-${RANDOM}"
SMOKE_EMAIL="smoke+${SMOKE_TOKEN}@mecontrola.local"
SMOKE_PHONE="+5511999${RANDOM}${RANDOM:0:4}"
SMOKE_SALE_ID="sale-${SMOKE_TOKEN}"
PAID_AT="$(date -u +'%Y-%m-%dT%H:%M:%SZ')"

PAYLOAD=$(cat <<EOF
{
  "subscription_id": "sub-${SMOKE_TOKEN}",
  "product_id": "${KIWIFY_PRODUCT_ID_MONTHLY}",
  "status": "paid",
  "customer": {
    "email": "${SMOKE_EMAIL}",
    "mobile": "${SMOKE_PHONE}"
  },
  "funnel_token": "${SMOKE_TOKEN}",
  "external_sale_id": "${SMOKE_SALE_ID}",
  "paid_at": "${PAID_AT}",
  "occurred_at": "${PAID_AT}"
}
EOF
)

log "==> Step 3/5 — disparando webhook Kiwify (token=${SMOKE_TOKEN})"
HTTP_STATUS=$(curl -sS -o /tmp/mvp_smoke_kiwify.json -w "%{http_code}" \
  -X POST "${KIWIFY_URL}" \
  -H "Content-Type: application/json" \
  -H "${KIWIFY_WEBHOOK_TOKEN_HEADER}: ${KIWIFY_WEBHOOK_SECRET}" \
  -d "${PAYLOAD}")

if [[ ! "${HTTP_STATUS}" =~ ^20[0-9]$ ]]; then
  cat /tmp/mvp_smoke_kiwify.json
  fail "webhook respondeu HTTP ${HTTP_STATUS}"
fi
pass "webhook aceito (HTTP ${HTTP_STATUS})"

log "==> Step 4/5 — aguardando email em Mailpit (timeout ${WAIT_MAX_SECONDS}s)"
DEADLINE=$(( $(date +%s) + WAIT_MAX_SECONDS ))
EMAIL_ID=""
while [[ -z "${EMAIL_ID}" && $(date +%s) -lt ${DEADLINE} ]]; do
  RESULT=$(curl -fsS "${MAILPIT_API}/search?query=to:${SMOKE_EMAIL}&limit=1" || echo '{}')
  EMAIL_ID=$(echo "${RESULT}" | jq -r '.messages[0].ID // empty')
  if [[ -z "${EMAIL_ID}" ]]; then sleep 1; fi
done

if [[ -z "${EMAIL_ID}" ]]; then
  fail "nenhum email entregue a ${SMOKE_EMAIL} em ${WAIT_MAX_SECONDS}s"
fi
pass "email recebido (id=${EMAIL_ID})"

EMAIL_BODY=$(curl -fsS "${MAILPIT_API}/message/${EMAIL_ID}")
SUBJECT=$(echo "${EMAIL_BODY}" | jq -r '.Subject')
HTML=$(echo "${EMAIL_BODY}" | jq -r '.HTML')

if [[ "${SUBJECT}" != *"Ative"* && "${SUBJECT}" != *"MeControla"* ]]; then
  fail "subject inesperado: ${SUBJECT}"
fi
if ! grep -q "/activate?token=" <<<"${HTML}"; then
  fail "HTML sem link /activate?token=..."
fi

ACTIVATE_TOKEN=$(echo "${HTML}" | grep -oE 'token=[A-Za-z0-9._-]+' | head -1 | cut -d= -f2)
if [[ -z "${ACTIVATE_TOKEN}" ]]; then
  fail "nao extraiu token do email"
fi
pass "subject=${SUBJECT} | token extraido (len=${#ACTIVATE_TOKEN})"

log "==> Step 5/5 — validando ${STATE_URL_BASE}/<token>/state"
STATE_RESPONSE=$(curl -fsS "${STATE_URL_BASE}/${ACTIVATE_TOKEN}/state")
READY=$(echo "${STATE_RESPONSE}" | jq -r '.ready_to_activate')
WAME=$(echo "${STATE_RESPONSE}" | jq -r '.wa_me_url // empty')

if [[ "${READY}" != "true" ]]; then
  echo "${STATE_RESPONSE}" | jq .
  fail "token invalido: ready_to_activate=${READY}"
fi
if [[ -z "${WAME}" || "${WAME}" != https://wa.me/* ]]; then
  fail "wa_me_url ausente ou invalido: ${WAME}"
fi
pass "state valido | wa_me_url=${WAME:0:60}..."

log "==> SMOKE E2E OK"
log "    token: ${SMOKE_TOKEN}"
log "    email destino: ${SMOKE_EMAIL}"
log "    wa.me: ${WAME}"
echo
echo "Para continuar manualmente:"
echo "  1. Abra ${WAME} (ou cole no WhatsApp Web)."
echo "  2. Confirme que recebe a mensagem de boas-vindas."
echo "  3. Responda com a renda mensal para iniciar onboarding."
