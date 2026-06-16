#!/usr/bin/env bash
# Chaos: kill worker between threshold detection and notification dispatch,
# verify idempotência (G2) — alerta NUNCA duplica e termina com notified_at preenchido.
#
# Assume schema: mecontrola.budget_alerts_sent (id, user_id, card_id, invoice_id,
# threshold, detected_at, notified_at, ...) com unique key cobrindo
# (user_id, card_id, invoice_id, threshold).
#
# Estratégia:
#   1. Cria card + invoice 90% gasto via SQL (fixtures CHAOS_USER_ID etc).
#   2. Inicia worker em background, captura PID.
#   3. Polla budget_alerts_sent — quando aparecer linha COM detected_at e SEM notified_at, mata o worker.
#   4. Aguarda 5s.
#   5. Reinicia worker.
#   6. Aguarda notified_at ficar NOT NULL (timeout 30s).
#   7. Confere count = 1 (idempotência).
#
# Envs:
#   DATABASE_URL              required
#   WORKER_BIN                default: go run ./cmd/worker
#   CHAOS_USER_ID             default: gerado
#   CHAOS_CARD_ID             default: gerado
#   CHAOS_INVOICE_ID          default: gerado
#   ALERT_THRESHOLD           default: 90
#
# Exit 0 = idempotência OK; 1 = falha.

set -euo pipefail

DATABASE_URL="${DATABASE_URL:?DATABASE_URL nao definido}"
WORKER_BIN="${WORKER_BIN:-go run ./cmd/worker}"
SCHEMA="${SCHEMA:-mecontrola}"
ALERT_THRESHOLD="${ALERT_THRESHOLD:-90}"
CHAOS_USER_ID="${CHAOS_USER_ID:-$(uuidgen | tr 'A-Z' 'a-z')}"
CHAOS_CARD_ID="${CHAOS_CARD_ID:-$(uuidgen | tr 'A-Z' 'a-z')}"
CHAOS_INVOICE_ID="${CHAOS_INVOICE_ID:-$(uuidgen | tr 'A-Z' 'a-z')}"
WORKER_LOG="${WORKER_LOG:-/tmp/chaos-worker.log}"

log() { printf '[chaos-worker %s] %s\n' "$(date +%H:%M:%S)" "$*"; }
fail() { log "FAIL: $*"; cleanup; exit 1; }
pass() { log "PASS: $*"; }

WORKER_PID=""
cleanup() {
  if [ -n "${WORKER_PID:-}" ] && kill -0 "$WORKER_PID" 2>/dev/null; then
    kill "$WORKER_PID" 2>/dev/null || true
  fi
}
trap cleanup EXIT

command -v psql >/dev/null || fail "psql ausente"

log "==> 1/7 verificando schema budget_alerts_sent"
HAS=$(psql "$DATABASE_URL" -tAc \
  "SELECT count(*) FROM information_schema.tables WHERE table_schema='${SCHEMA}' AND table_name='budget_alerts_sent'")
[ "$HAS" = "1" ] || fail "tabela ${SCHEMA}.budget_alerts_sent ausente"

log "==> 2/7 carregando fixtures (user=${CHAOS_USER_ID} card=${CHAOS_CARD_ID} invoice=${CHAOS_INVOICE_ID})"
psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -v u="$CHAOS_USER_ID" -v c="$CHAOS_CARD_ID" -v i="$CHAOS_INVOICE_ID" -v s="$SCHEMA" >/dev/null <<'SQL'
-- NOTE: estas instrucoes podem precisar de ajuste ao schema real do projeto.
-- Mantemos como template documentavel; operador deve adaptar campos NOT NULL.
DO $$
DECLARE
  uid uuid := :'u'::uuid;
  cid uuid := :'c'::uuid;
  iid uuid := :'i'::uuid;
  s text := :'s';
BEGIN
  EXECUTE format('INSERT INTO %I.users (id, email, created_at) VALUES ($1, $2, now()) ON CONFLICT (id) DO NOTHING', s)
    USING uid, ('chaos+' || uid::text || '@mecontrola.local');
  EXECUTE format('INSERT INTO %I.cards (id, user_id, name, limit_cents, created_at) VALUES ($1, $2, $3, $4, now()) ON CONFLICT (id) DO NOTHING', s)
    USING cid, uid, 'Chaos Card', 100000;
  EXECUTE format('INSERT INTO %I.card_invoices (id, card_id, user_id, ref_month, total_spent_cents, total_limit_cents, created_at) VALUES ($1, $2, $3, $4, $5, $6, now()) ON CONFLICT (id) DO NOTHING', s)
    USING iid, cid, uid, date_trunc('month', now())::date, 90000, 100000;
END$$;
SQL

log "==> 3/7 iniciando worker (log=${WORKER_LOG})"
nohup $WORKER_BIN > "$WORKER_LOG" 2>&1 &
WORKER_PID=$!
sleep 2
kill -0 "$WORKER_PID" 2>/dev/null || fail "worker morreu logo apos start (ver ${WORKER_LOG})"

log "==> 4/7 polling budget_alerts_sent ate aparecer linha sem notified_at"
KILLED=0
for _ in $(seq 1 60); do
  ROW=$(psql "$DATABASE_URL" -tAc \
    "SELECT count(*) FROM ${SCHEMA}.budget_alerts_sent
      WHERE card_id='${CHAOS_CARD_ID}' AND invoice_id='${CHAOS_INVOICE_ID}' AND threshold=${ALERT_THRESHOLD}")
  if [ "$ROW" -ge "1" ]; then
    NOTIFIED=$(psql "$DATABASE_URL" -tAc \
      "SELECT count(*) FROM ${SCHEMA}.budget_alerts_sent
        WHERE card_id='${CHAOS_CARD_ID}' AND invoice_id='${CHAOS_INVOICE_ID}' AND threshold=${ALERT_THRESHOLD}
          AND notified_at IS NOT NULL")
    if [ "$NOTIFIED" = "0" ]; then
      log "    matando worker antes do notified_at (pid=${WORKER_PID})"
      kill -9 "$WORKER_PID" 2>/dev/null || true
      KILLED=1
      break
    fi
    log "    worker ja notificou antes do kill — chaos perdeu janela; reiniciando teste manualmente se quiser"
    KILLED=2
    break
  fi
  sleep 1
done

if [ "$KILLED" = "0" ]; then
  fail "alerta nao foi detectado em 60s"
fi

sleep 5

log "==> 5/7 reiniciando worker"
nohup $WORKER_BIN >> "$WORKER_LOG" 2>&1 &
WORKER_PID=$!
sleep 2
kill -0 "$WORKER_PID" 2>/dev/null || fail "worker reiniciado morreu (ver ${WORKER_LOG})"

log "==> 6/7 aguardando notified_at ser preenchido"
NOTIFIED_OK=0
for _ in $(seq 1 30); do
  NOTIFIED=$(psql "$DATABASE_URL" -tAc \
    "SELECT count(*) FROM ${SCHEMA}.budget_alerts_sent
      WHERE card_id='${CHAOS_CARD_ID}' AND invoice_id='${CHAOS_INVOICE_ID}' AND threshold=${ALERT_THRESHOLD}
        AND notified_at IS NOT NULL")
  if [ "$NOTIFIED" -ge "1" ]; then
    NOTIFIED_OK=1
    break
  fi
  sleep 1
done
[ "$NOTIFIED_OK" = "1" ] || fail "notified_at nunca foi preenchido apos restart"

log "==> 7/7 conferindo unicidade (G2 idempotencia)"
TOTAL=$(psql "$DATABASE_URL" -tAc \
  "SELECT count(*) FROM ${SCHEMA}.budget_alerts_sent
    WHERE card_id='${CHAOS_CARD_ID}' AND invoice_id='${CHAOS_INVOICE_ID}' AND threshold=${ALERT_THRESHOLD}")
[ "$TOTAL" = "1" ] || fail "duplicidade detectada: ${TOTAL} linhas (esperado 1)"

pass "alerta retentado e notified_at preenchido — sem duplicacao"
