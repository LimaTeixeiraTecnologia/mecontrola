#!/usr/bin/env bash
set -euo pipefail

# Configura (idempotente) o contact point Telegram + notification policy no Grafana
# do otel-lgtm via API de provisioning. Os segredos ficam no .env, nunca no git.
#
# Le ALERT_TELEGRAM_BOT_TOKEN / ALERT_TELEGRAM_CHAT_ID / OTEL_LGTM_ADMIN_PASSWORD
# do ambiente ou, se ausentes, do arquivo ENV_FILE (default ./.env). Nao precisa
# "sourcear" o .env (que tem valores com espaco e quebra o shell).
#
# Uso (na VPS ou local, com o otel-lgtm no ar):
#   bash deployment/telemetry/grafana/setup-alerting-telegram.sh
#
# Overrides opcionais: GRAFANA_URL (default http://127.0.0.1:3000),
#                      GRAFANA_ADMIN_USER (default admin), ENV_FILE (default .env).

ENV_FILE="${ENV_FILE:-.env}"
from_env_file() { [ -f "$ENV_FILE" ] && grep "^$1=" "$ENV_FILE" | tail -1 | cut -d= -f2- || true; }

: "${ALERT_TELEGRAM_BOT_TOKEN:=$(from_env_file ALERT_TELEGRAM_BOT_TOKEN)}"
: "${ALERT_TELEGRAM_CHAT_ID:=$(from_env_file ALERT_TELEGRAM_CHAT_ID)}"
: "${GRAFANA_ADMIN_PASSWORD:=$(from_env_file OTEL_LGTM_ADMIN_PASSWORD)}"

if [ -z "${ALERT_TELEGRAM_BOT_TOKEN}" ] || [ -z "${ALERT_TELEGRAM_CHAT_ID}" ]; then
  echo "ALERT_TELEGRAM_* nao definidos — alertas apenas no painel (sem Telegram). Pulando."
  exit 0
fi

GRAFANA_URL="${GRAFANA_URL:-http://127.0.0.1:3000}"
GRAFANA_ADMIN_USER="${GRAFANA_ADMIN_USER:-admin}"
GRAFANA_ADMIN_PASSWORD="${GRAFANA_ADMIN_PASSWORD:?defina GRAFANA_ADMIN_PASSWORD ou OTEL_LGTM_ADMIN_PASSWORD no .env}"

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
    "message": "{{ template \"mc.telegram.message\" . }}",
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

echo "==> teste de entrega (Telegram direto)"
resp=$(curl -s "https://api.telegram.org/bot${ALERT_TELEGRAM_BOT_TOKEN}/sendMessage" \
  --data-urlencode "chat_id=${ALERT_TELEGRAM_CHAT_ID}" \
  --data-urlencode "text=🔔 MeControla: alertas configurados (mensagem de teste)" || true)
if echo "$resp" | grep -q '"ok":true'; then
  echo "    entregue ✓ — alertas roteando para Telegram."
else
  echo "    AVISO: Telegram nao entregou. Verifique se voce deu /start no bot. Resposta: $(echo "$resp" | head -c 200)"
fi
