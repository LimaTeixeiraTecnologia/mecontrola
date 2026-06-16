#!/usr/bin/env bash
set -euo pipefail

KIWIFY_SECRET="${KIWIFY_WEBHOOK_SECRET:-}"
PAYLOAD_FILE="${1:-/dev/stdin}"
BASE_URL="${BASE_URL:-http://localhost:8080}"

if [[ -z "$KIWIFY_SECRET" ]]; then
  echo "ERRO: KIWIFY_WEBHOOK_SECRET nao definido" >&2
  exit 1
fi

if [[ "$PAYLOAD_FILE" != "/dev/stdin" && ! -f "$PAYLOAD_FILE" ]]; then
  echo "ERRO: arquivo nao encontrado: $PAYLOAD_FILE" >&2
  exit 1
fi

# Compute HMAC-SHA1 over the EXACT bytes that will be sent (file content as-is).
SIG=$(openssl dgst -sha1 -mac HMAC -macopt "key:${KIWIFY_SECRET}" "$PAYLOAD_FILE" | awk '{print $2}')

echo "-> Assinatura: $SIG" >&2
echo "-> Enviando POST ${BASE_URL}/api/v1/billing/webhooks/kiwify?signature=${SIG}" >&2

# IMPORTANT: --data-binary @file (NOT -d @file).
# curl -d strips newlines, breaking HMAC since we sign the file content as-is.
curl -sS -w "\n%{http_code}\n" \
  -X POST "${BASE_URL}/api/v1/billing/webhooks/kiwify?signature=${SIG}" \
  -H "Content-Type: application/json" \
  --data-binary "@${PAYLOAD_FILE}"
