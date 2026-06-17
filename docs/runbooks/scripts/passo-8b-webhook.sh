#!/usr/bin/env bash
#
# Passo 8B — Simula o webhook order_approved do Kiwify (sem gastar dinheiro).
#
# Fluxo: gera magic token via checkout real -> monta o envelope order_approved
# -> assina HMAC-SHA1/hex (igual ao middleware hmac_signature.go) -> POST no
# webhook -> valida {"received": true} -> (opcional via SSH) logs do worker + banco.
#
# Pre-condicao MANDATORIA: KIWIFY_WEBHOOK_SECRET real (do painel do Kiwify) na VPS.
# Enquanto for CHANGE_ME_*, este script falha de proposito.
#
# Uso:
#   KIWIFY_WEBHOOK_SECRET=<secret> ./passo-8b-webhook.sh
#   # ou deixe o script puxar o secret da VPS via SSH:
#   ./passo-8b-webhook.sh
#
# Variaveis (todas com default):
#   API_BASE   (https://api.mecontrola.app.br)
#   ORIGIN     (https://www.mecontrola.app.br)
#   PLAN       (MONTHLY | QUARTERLY | ANNUAL)
#   EMAIL      (jailton.junior94@outlook.com)   -> caixa que recebe o email
#   PHONE      (+55...)                          -> obrigatorio p/ ativacao no Passo 11
#   VPS        (root@187.77.45.48)               -> p/ logs/banco; vazio desativa
#   WATCH_LOGS (1)                               -> 0 pula o tail dos logs do worker

set -euo pipefail

API_BASE="${API_BASE:-https://api.mecontrola.app.br}"
ORIGIN="${ORIGIN:-https://www.mecontrola.app.br}"
PLAN="${PLAN:-MONTHLY}"
EMAIL="${EMAIL:-jailton.junior94@outlook.com}"
PHONE="${PHONE:-+5511999990000}"
VPS="${VPS:-root@187.77.45.48}"
WATCH_LOGS="${WATCH_LOGS:-1}"

case "$PLAN" in
  MONTHLY)   PRODUCT_ID="2d7d8e25-ecfd-45f0-98ba-54a496060959"; PRODUCT_NAME="Me Controla Mensal" ;;
  QUARTERLY) PRODUCT_ID="c2c2ec27-18d4-4bff-a551-ab5f98a78eb5"; PRODUCT_NAME="Me Controla Trimestral" ;;
  ANNUAL)    PRODUCT_ID="abaac314-0ab6-4474-aeab-aca498cb8c4a"; PRODUCT_NAME="Me Controla Anual" ;;
  *) echo "PLAN invalido: $PLAN (use MONTHLY|QUARTERLY|ANNUAL)"; exit 2 ;;
esac

log() { printf '\n\033[1;36m== %s\033[0m\n' "$*"; }
fail() { printf '\n\033[1;31mFALHA: %s\033[0m\n' "$*" >&2; exit 1; }

# ---------------------------------------------------------------------------
log "0. Resolvendo KIWIFY_WEBHOOK_SECRET"
SECRET="${KIWIFY_WEBHOOK_SECRET:-${1:-}}"
if [ -z "$SECRET" ] && [ -n "$VPS" ]; then
  echo "  secret nao informado; puxando da VPS ($VPS)..."
  SECRET="$(ssh -o BatchMode=yes "$VPS" 'grep "^KIWIFY_WEBHOOK_SECRET=" /opt/mecontrola/.env | cut -d= -f2-' 2>/dev/null || true)"
fi
[ -n "$SECRET" ] || fail "KIWIFY_WEBHOOK_SECRET ausente. Passe via env/arg ou configure SSH para a VPS."
case "$SECRET" in
  CHANGE_ME*) fail "KIWIFY_WEBHOOK_SECRET ainda e placeholder ($SECRET). Configure o valor real do painel do Kiwify em /opt/mecontrola/.env e reinicie server+worker." ;;
esac
echo "  secret OK (len=${#SECRET})"

# ---------------------------------------------------------------------------
log "1. Gerando magic token via checkout real ($PLAN)"
CHECKOUT="$(curl -fsS -X POST "$API_BASE/api/v1/onboarding/checkout" \
  -H "Content-Type: application/json" -H "Origin: $ORIGIN" \
  -d "{\"plan_id\":\"$PLAN\"}")"
echo "  resposta: $CHECKOUT"
TOKEN="$(printf '%s' "$CHECKOUT" | grep -o 'sck=[A-Za-z0-9_-]*' | cut -d= -f2 || true)"
[ -n "$TOKEN" ] || fail "nao consegui extrair o token (sck) da resposta do checkout."
echo "  TOKEN=$TOKEN"

# ---------------------------------------------------------------------------
log "2. Montando envelope order_approved (assinatura sobre bytes exatos)"
STAMP="$(date +%s)"
NOW_ISO="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
BODY_FILE="$(mktemp)"
trap 'rm -f "$BODY_FILE"' EXIT
cat > "$BODY_FILE" <<JSON
{"order_id":"test-e2e-$STAMP","order_ref":"test-ref-$STAMP","order_status":"paid","webhook_event_type":"order_approved","subscription_id":"sub-test-$STAMP","Product":{"product_id":"$PRODUCT_ID","product_name":"$PRODUCT_NAME"},"Customer":{"email":"$EMAIL","mobile":"$PHONE","CPF":""},"Subscription":{"start_date":"$NOW_ISO","next_payment":"$NOW_ISO","status":"active"},"TrackingParameters":{"sck":"$TOKEN","s1":"","src":""},"approved_date":"$NOW_ISO","updated_at":"$NOW_ISO","created_at":"$NOW_ISO"}
JSON
SIG="$(openssl dgst -sha1 -hmac "$SECRET" < "$BODY_FILE" | awk '{print $NF}')"
echo "  signature=$SIG"

# ---------------------------------------------------------------------------
log "3. POST no webhook (espera {\"received\": true})"
RESP="$(curl -sS -X POST "$API_BASE/api/v1/billing/webhooks/kiwify?signature=$SIG" \
  -H "Content-Type: application/json" --data-binary @"$BODY_FILE" -w $'\n%{http_code}')"
HTTP="$(printf '%s' "$RESP" | tail -n1)"
JSON_RESP="$(printf '%s' "$RESP" | sed '$d')"
echo "  HTTP=$HTTP body=$JSON_RESP"
case "$HTTP" in
  200|201|202) printf '%s' "$JSON_RESP" | grep -q '"received":[[:space:]]*true' \
                 && echo "  webhook ACEITO" || echo "  ATENCAO: aceito mas sem received:true" ;;
  401) fail "401 invalid signature — KIWIFY_WEBHOOK_SECRET nao corresponde ao que o server carregou (reinicie server apos editar .env)." ;;
  *)   fail "HTTP $HTTP inesperado — ver body acima." ;;
esac

# ---------------------------------------------------------------------------
log "4. Estado do token apos webhook (state endpoint)"
sleep 3
curl -fsS "$API_BASE/api/v1/onboarding/tokens/$TOKEN/state" -H "Origin: $ORIGIN" || true
echo

# ---------------------------------------------------------------------------
if [ -n "$VPS" ] && [ "$WATCH_LOGS" = "1" ]; then
  log "5. Logs do worker (~45s) — Passo 9"
  echo "  procurando: process_sale_approved | mark_token_paid | activation_email_dispatched"
  ssh -o BatchMode=yes "$VPS" \
    "timeout 45 docker logs -f mecontrola-worker-1 2>&1 | grep -iE 'process_sale_approved|mark_token_paid|activation_email|email_dispatched|send_failed|error' | head -30" \
    || echo "  (timeout/sem match — verifique manualmente)"

  log "6. Banco — Passo 12 (token deve avancar PENDING -> PAID -> CONSUMED apos ATIVAR)"
  ssh -o BatchMode=yes "$VPS" '
    DB_USER=$(grep "^DB_USER=" /opt/mecontrola/.env | cut -d= -f2)
    DB_NAME=$(grep "^DB_NAME=" /opt/mecontrola/.env | cut -d= -f2)
    docker exec mecontrola-postgres-1 psql -U "$DB_USER" -d "$DB_NAME" -At -c "
      SELECT mt.status, mt.activation_path, s.plan_id, s.status
      FROM mecontrola.magic_tokens mt
      LEFT JOIN mecontrola.subscriptions s ON s.id = mt.subscription_id
      ORDER BY mt.created_at DESC LIMIT 3;"' \
    || echo "  (consulta falhou — verifique manualmente)"
fi

log "Concluido"
cat <<EOF

Proximos passos manuais:
  - Passo 10: confirmar email de ativacao em $EMAIL (cheque spam).
  - Passo 11: abrir $ORIGIN/ativar?token=$TOKEN  (ou enviar "ATIVAR $TOKEN" no WhatsApp).
  - Passo 12: rodar de novo o bloco de banco acima; token deve estar CONSUMED.
EOF
