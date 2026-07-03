#!/usr/bin/env bash
set -euo pipefail

# setup-grafana-alerts.sh — Renderiza credenciais do Telegram no provisioning de alertas.
#
# Uso:
#   bash deployment/scripts/setup-grafana-alerts.sh <env-file>
#
# O arquivo contact-points.yaml contem placeholders ${ALERT_TELEGRAM_BOT_TOKEN}
# e ${ALERT_TELEGRAM_CHAT_ID} que sao substituidos pelos valores reais do .env.
# O resultado e escrito em deployment/telemetry/grafana/provisioning/alerting/contact-points.rendered.yaml,
# que deve ser montado no container otel-lgtm em vez do arquivo original.
# Este script nao cria contact points via API e nao envia mensagem de teste.

ENV_FILE="${1:-.env}"
SRC="deployment/telemetry/grafana/provisioning/alerting/contact-points.yaml"
DST="deployment/telemetry/grafana/provisioning/alerting/contact-points.rendered.yaml"

if [[ ! -f "$ENV_FILE" ]]; then
  echo "ERRO: $ENV_FILE nao encontrado"
  exit 1
fi

if [[ ! -f "$SRC" ]]; then
  echo "ERRO: $SRC nao encontrado"
  exit 1
fi

# Le apenas as variaveis necessarias sem executar o .env como shell.
raw_bot_token="$(grep -E '^ALERT_TELEGRAM_BOT_TOKEN=' "$ENV_FILE" | cut -d= -f2- | sed 's/^[[:space:]]*//;s/[[:space:]]*$//' || true)"
raw_chat_id="$(grep -E '^ALERT_TELEGRAM_CHAT_ID=' "$ENV_FILE" | cut -d= -f2- | sed 's/^[[:space:]]*//;s/[[:space:]]*$//' || true)"

# Remove aspas externas se presentes (formato env comum).
raw_bot_token="${raw_bot_token#\"}"; raw_bot_token="${raw_bot_token%\"}"
raw_chat_id="${raw_chat_id#\"}"; raw_chat_id="${raw_chat_id%\"}"

if [[ -z "$raw_bot_token" || -z "$raw_chat_id" ]]; then
  echo "AVISO: ALERT_TELEGRAM_BOT_TOKEN ou ALERT_TELEGRAM_CHAT_ID nao configurados — alertas do Telegram nao serao ativados via provisioning"
  exit 0
fi

sed \
  -e "s|\\\${ALERT_TELEGRAM_BOT_TOKEN}|${raw_bot_token}|g" \
  -e "s|\\\${ALERT_TELEGRAM_CHAT_ID}|${raw_chat_id}|g" \
  "$SRC" > "$DST"

echo "OK: $DST gerado com credenciais do Telegram"
