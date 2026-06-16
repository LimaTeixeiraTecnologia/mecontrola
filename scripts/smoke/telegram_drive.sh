#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${ROOT_DIR}"

DB_URL="${DB_URL:-postgres://mecontrola:mecontrola@localhost:5432/mecontrola}"
MAILPIT_API="${MAILPIT_API:-http://localhost:8025/api/v1}"
BACKEND_URL="${BACKEND_URL:-http://localhost:8080}"
POLL_TIMEOUT="${POLL_TIMEOUT:-90}"

log() { printf '\n[telegram-drive %s] %s\n' "$(date +%H:%M:%S)" "$*"; }
fail() { log "FAIL: $*"; exit 1; }
pass() { log "PASS: $*"; }
prompt() { printf '\n>>> %s\n>>> Pressione ENTER quando completar (Ctrl+C para abortar)\n' "$*"; read -r _; }

require() { command -v "$1" >/dev/null 2>&1 || fail "binario ausente: $1"; }
require curl
require jq
require psql

load_env() {
  set -a; source .env; set +a
}

psql_q() { psql "${DB_URL}" -tAc "$1"; }

poll_until() {
  local label="$1"
  local query="$2"
  local expected="$3"
  local deadline=$(( $(date +%s) + POLL_TIMEOUT ))
  while [[ $(date +%s) -lt ${deadline} ]]; do
    local got
    got=$(psql_q "${query}" 2>/dev/null | tr -d '[:space:]')
    if [[ "${got}" == "${expected}" ]]; then
      pass "${label} (esperado=${expected})"
      return 0
    fi
    sleep 2
  done
  fail "${label} timeout — ultimo valor='${got:-NULL}', esperado='${expected}'"
}

step_kiwify() {
  log "==> 1/8 simulando webhook Kiwify"
  curl -fsS -X DELETE "${MAILPIT_API}/messages" >/dev/null

  SMOKE_TOKEN="tg-$(date +%s)-$RANDOM"
  SMOKE_EMAIL="dev-${SMOKE_TOKEN}@mecontrola.local"
  SMOKE_PHONE="+5511999$RANDOM${RANDOM:0:4}"
  SMOKE_SALE="sale-${SMOKE_TOKEN}"
  PAID_AT="$(date -u +'%Y-%m-%dT%H:%M:%SZ')"
  PAYLOAD=$(jq -nc \
    --arg sub "sub-${SMOKE_TOKEN}" \
    --arg prod "${KIWIFY_PRODUCT_ID_MONTHLY}" \
    --arg email "${SMOKE_EMAIL}" \
    --arg phone "${SMOKE_PHONE}" \
    --arg token "${SMOKE_TOKEN}" \
    --arg sale "${SMOKE_SALE}" \
    --arg paid "${PAID_AT}" \
    '{subscription_id:$sub, product_id:$prod, status:"paid", customer:{email:$email, mobile:$phone}, funnel_token:$token, external_sale_id:$sale, paid_at:$paid, occurred_at:$paid}')

  local status
  status=$(curl -sS -o /tmp/telegram-drive-kiwify.json -w "%{http_code}" \
    -X POST "${BACKEND_URL}/api/v1/billing/webhooks/kiwify" \
    -H "Content-Type: application/json" \
    -H "X-Kiwify-Webhook-Token: ${KIWIFY_WEBHOOK_SECRET}" \
    -d "${PAYLOAD}")
  if [[ ! "${status}" =~ ^20[0-9]$ ]]; then
    cat /tmp/telegram-drive-kiwify.json
    fail "Kiwify webhook HTTP ${status}"
  fi
  pass "Kiwify HTTP ${status} | email=${SMOKE_EMAIL} phone=${SMOKE_PHONE}"

  export SMOKE_TOKEN SMOKE_EMAIL SMOKE_PHONE
}

step_email() {
  log "==> 2/8 aguardando email no Mailpit"
  local deadline=$(( $(date +%s) + 30 ))
  local id=""
  while [[ -z "${id}" && $(date +%s) -lt ${deadline} ]]; do
    id=$(curl -fsS "${MAILPIT_API}/search?query=to:${SMOKE_EMAIL}&limit=1" 2>/dev/null | jq -r '.messages[0].ID // empty')
    [[ -z "${id}" ]] && sleep 1
  done
  [[ -z "${id}" ]] && fail "email nao chegou em 30s"

  local html
  html=$(curl -fsS "${MAILPIT_API}/message/${id}" | jq -r '.HTML')
  ACTIVATE_TOKEN=$(echo "${html}" | grep -oE 'token=[A-Za-z0-9._-]+' | head -1 | cut -d= -f2)
  [[ -z "${ACTIVATE_TOKEN}" ]] && fail "nao extrai token do email"
  pass "token extraido (len=${#ACTIVATE_TOKEN})"
  export ACTIVATE_TOKEN
}

step_state() {
  log "==> 3/8 consultando /state"
  local resp
  resp=$(curl -fsS "${BACKEND_URL}/api/v1/onboarding/tokens/${ACTIVATE_TOKEN}/state")
  echo "${resp}" | jq
  TG_LINK=$(echo "${resp}" | jq -r '.telegram_deep_link // empty')
  [[ -z "${TG_LINK}" ]] && fail "telegram_deep_link vazio (verifique TELEGRAM_BOT_USERNAME no .env)"
  pass "deep link=${TG_LINK}"
  export TG_LINK
}

step_human_activate() {
  cat <<EOF

==================================================================
 ATIVACAO HUMANA — 1 acao
==================================================================
 1. Abra o link no celular ou navegador:

    ${TG_LINK}

 2. No Telegram, clique em INICIAR (ou digite manualmente):
    ATIVAR ${ACTIVATE_TOKEN}

 3. Aguarde o bot responder "Bem-vindo(a)..."
==================================================================
EOF
  log "==> 4/8 polling DB para ativacao (timeout ${POLL_TIMEOUT}s)..."
  poll_until "user_identities telegram criada" \
    "SELECT COUNT(*) FROM mecontrola.user_identities ui
     JOIN mecontrola.users u ON u.id=ui.user_id
     WHERE u.whatsapp_number='${SMOKE_PHONE}' AND ui.channel='telegram'" \
    "1"

  USER_ID=$(psql_q "SELECT id FROM mecontrola.users WHERE whatsapp_number='${SMOKE_PHONE}'")
  export USER_ID
  pass "user_id=${USER_ID}"
}

step_onboarding_fsm() {
  log "==> 5/8 onboarding FSM"

  prompt "Envie no Telegram: 3500"
  poll_until "income registrada" \
    "SELECT (payload->>'IncomeCents')::text FROM mecontrola.onboarding_sessions WHERE user_id='${USER_ID}'" \
    "350000"

  prompt "Envie no Telegram: nao"
  prompt "Envie no Telegram: esta otimo"

  poll_until "session.state=active" \
    "SELECT state FROM mecontrola.onboarding_sessions WHERE user_id='${USER_ID}'" \
    "active"
}

step_log_expense() {
  log "==> 6/8 LogExpense via LLM live"
  prompt "Envie no Telegram: gastei 50 reais no iFood"
  poll_until "expense persistido com source=agent" \
    "SELECT COUNT(*) FROM mecontrola.budgets_expenses WHERE user_id='${USER_ID}' AND source='agent' AND amount_cents=5000" \
    "1"
}

step_alert_setup() {
  log "==> 7/8 criando cartao + invoice 90% via SQL e disparando alert"

  CARD_ID=$(uuidgen | tr A-Z a-z)
  INVOICE_ID=$(uuidgen | tr A-Z a-z)
  REF_MONTH=$(date +%Y-%m)

  psql "${DB_URL}" -c "
    INSERT INTO mecontrola.cards (id, user_id, name, nickname, closing_day, due_day, limit_cents, created_at, updated_at)
    VALUES ('${CARD_ID}', '${USER_ID}', 'Nubank E2E', 'nu-e2e-${RANDOM}', 10, 20, 500000, now(), now());
  " >/dev/null

  psql "${DB_URL}" -c "
    INSERT INTO mecontrola.transactions_card_invoices (id, user_id, card_id, ref_month, closing_at, due_at, items_total_cents, version, created_at, updated_at)
    VALUES ('${INVOICE_ID}', '${USER_ID}', '${CARD_ID}', '${REF_MONTH}', now() + interval '15 days', now() + interval '30 days', 450000, 1, now(), now());
  " >/dev/null

  pass "card_id=${CARD_ID} | invoice 90% do limite | ref_month=${REF_MONTH}"
  export CARD_ID

  log "disparando ThresholdAlertsJob (worker efemero ~8s)..."
  BUDGETS_THRESHOLD_ALERTS_MODE=job \
  BUDGETS_THRESHOLD_ALERTS_CRON='@every 2s' \
    timeout 8 go run ./cmd worker >/tmp/telegram-drive-alerts.log 2>&1 || true

  poll_until "budget_alerts_sent kind=card_limit_near" \
    "SELECT COUNT(*) FROM mecontrola.budget_alerts_sent WHERE user_id='${USER_ID}' AND budget_id='${CARD_ID}' AND kind='card_limit_near'" \
    "1"

  poll_until "alerta notificado (notified_at NOT NULL)" \
    "SELECT COUNT(*) FROM mecontrola.budget_alerts_sent WHERE user_id='${USER_ID}' AND budget_id='${CARD_ID}' AND notified_at IS NOT NULL" \
    "1"
}

step_evidence() {
  log "==> 8/8 evidencias agregadas"
  echo "--- users ---"
  psql "${DB_URL}" -c "SELECT id, whatsapp_number, status FROM mecontrola.users WHERE id='${USER_ID}'"
  echo "--- identities ---"
  psql "${DB_URL}" -c "SELECT channel, external_id, verified_at FROM mecontrola.user_identities WHERE user_id='${USER_ID}'"
  echo "--- onboarding_sessions ---"
  psql "${DB_URL}" -c "SELECT state, payload->>'IncomeCents' AS income FROM mecontrola.onboarding_sessions WHERE user_id='${USER_ID}'"
  echo "--- expenses ---"
  psql "${DB_URL}" -c "SELECT amount_cents, source, root_slug, occurred_at FROM mecontrola.budgets_expenses WHERE user_id='${USER_ID}' ORDER BY occurred_at DESC"
  echo "--- alerts ---"
  psql "${DB_URL}" -c "SELECT kind, budget_id, sent_at, notified_at, notify_channel FROM mecontrola.budget_alerts_sent WHERE user_id='${USER_ID}'"
}

step_final_prompt() {
  cat <<EOF

==================================================================
 ULTIMO CHECK HUMANO — visual
==================================================================
 Voce deve ter recebido AGORA no Telegram uma mensagem proativa:

    "Sua fatura no cartao esta em R\$ 4.500,00.
     Voce ja utilizou 90% do limite."

 Se SIM   -> production-ready end-to-end (Telegram).
 Se NAO   -> verificar /tmp/telegram-worker.log para erro do consumer.
==================================================================
EOF
}

trap 'log "telegram-drive finalizado"' EXIT

load_env
step_kiwify
step_email
step_state
step_human_activate
step_onboarding_fsm
step_log_expense
step_alert_setup
step_evidence
step_final_prompt
