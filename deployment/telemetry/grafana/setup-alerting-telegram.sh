#!/usr/bin/env bash
set -euo pipefail

# Configura (idempotente) o contact point Telegram + notification policy no Grafana
# do otel-lgtm via API de provisioning. Os segredos ficam no .env, nunca no git.
#
# Uso (na VPS ou local, com o otel-lgtm no ar):
#   set -a; . ./.env; set +a
#   GRAFANA_ADMIN_PASSWORD="$OTEL_LGTM_ADMIN_PASSWORD" \
#     bash deployment/telemetry/grafana/setup-alerting-telegram.sh
#
# Variaveis: ALERT_TELEGRAM_BOT_TOKEN, ALERT_TELEGRAM_CHAT_ID (obrigatorias),
#            GRAFANA_URL (default http://127.0.0.1:3000),
#            GRAFANA_ADMIN_USER (default admin), GRAFANA_ADMIN_PASSWORD (obrigatoria).

GRAFANA_URL="${GRAFANA_URL:-http://127.0.0.1:3000}"
GRAFANA_ADMIN_USER="${GRAFANA_ADMIN_USER:-admin}"
GRAFANA_ADMIN_PASSWORD="${GRAFANA_ADMIN_PASSWORD:?defina GRAFANA_ADMIN_PASSWORD}"
ALERT_TELEGRAM_BOT_TOKEN="${ALERT_TELEGRAM_BOT_TOKEN:?defina ALERT_TELEGRAM_BOT_TOKEN}"
ALERT_TELEGRAM_CHAT_ID="${ALERT_TELEGRAM_CHAT_ID:?defina ALERT_TELEGRAM_CHAT_ID}"

AUTH=(-u "${GRAFANA_ADMIN_USER}:${GRAFANA_ADMIN_PASSWORD}")
HDR=(-H "Content-Type: application/json" -H "X-Disable-Provenance: true")
UID_CP="telegram-oncall"

read -r -d '' CONTACT_POINT <<JSON || true
{
  "uid": "${UID_CP}",
  "name": "telegram-oncall",
  "type": "telegram",
  "settings": {
    "bottoken": "${ALERT_TELEGRAM_BOT_TOKEN}",
    "chatid": "${ALERT_TELEGRAM_CHAT_ID}",
    "parse_mode": "HTML"
  },
  "disableResolveMessage": false
}
JSON

echo "==> contact point telegram-oncall"
if curl -s "${AUTH[@]}" "${GRAFANA_URL}/api/v1/provisioning/contact-points" | grep -q "\"uid\":\"${UID_CP}\""; then
  code=$(curl -s -o /dev/null -w "%{http_code}" "${AUTH[@]}" "${HDR[@]}" \
    -X PUT "${GRAFANA_URL}/api/v1/provisioning/contact-points/${UID_CP}" -d "${CONTACT_POINT}")
else
  code=$(curl -s -o /dev/null -w "%{http_code}" "${AUTH[@]}" "${HDR[@]}" \
    -X POST "${GRAFANA_URL}/api/v1/provisioning/contact-points" -d "${CONTACT_POINT}")
fi
[[ "$code" =~ ^2 ]] && echo "    ok (${code})" || { echo "    FALHOU (${code})"; exit 1; }

read -r -d '' POLICY <<'JSON' || true
{
  "receiver": "telegram-oncall",
  "group_by": ["alertname"],
  "group_wait": "30s",
  "group_interval": "5m",
  "repeat_interval": "4h",
  "routes": [
    { "receiver": "telegram-oncall", "object_matchers": [["severity", "=~", "critical|warning"]] }
  ]
}
JSON

echo "==> notification policy -> telegram-oncall"
code=$(curl -s -o /dev/null -w "%{http_code}" "${AUTH[@]}" "${HDR[@]}" \
  -X PUT "${GRAFANA_URL}/api/v1/provisioning/policies" -d "${POLICY}")
[[ "$code" =~ ^2 ]] && echo "    ok (${code})" || { echo "    FALHOU (${code})"; exit 1; }

echo "==> teste de entrega (envia uma mensagem de teste ao Telegram)"
curl -s "${AUTH[@]}" "${HDR[@]}" -X POST "${GRAFANA_URL}/api/v1/provisioning/contact-points/test" \
  -d "${CONTACT_POINT}" -o /dev/null -w "    test status: %{http_code}\n" || true

echo "OK — alertas roteando para Telegram."
