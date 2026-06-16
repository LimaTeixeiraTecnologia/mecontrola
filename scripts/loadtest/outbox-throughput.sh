#!/usr/bin/env bash
# Outbox throughput drill.
#
# Inserts N synthetic events into mecontrola.outbox_events and measures how long
# the dispatcher takes to drain them (published_at IS NOT NULL OR status = 3).
#
# Status enum (see outbox publisher):
#   0 = pending, 1 = published, 2 = failed (retry), 3 = dead-letter (terminal)
#
# Envs:
#   DATABASE_URL   postgres conn string (required)
#   EVENT_COUNT    number of events to insert (default 1000)
#   TIMEOUT_SEC    max time to wait for drain (default 300)
#
# Exit codes:
#   0  drained within TIMEOUT_SEC
#   1  precondition failed
#   2  timeout — drain incomplete

set -euo pipefail

DATABASE_URL="${DATABASE_URL:?DATABASE_URL not set}"
EVENT_COUNT="${EVENT_COUNT:-1000}"
TIMEOUT_SEC="${TIMEOUT_SEC:-300}"
SCHEMA="${SCHEMA:-mecontrola}"
RUN_ID="loadtest-$(date +%s)"

log() { printf '[outbox-tp %s] %s\n' "$(date +%H:%M:%S)" "$*"; }
fail() { log "FAIL: $*"; exit 1; }

command -v psql >/dev/null || fail "psql ausente"

log "==> verificando schema e tabela outbox_events"
psql "$DATABASE_URL" -tAc "SELECT 1 FROM information_schema.tables WHERE table_schema='${SCHEMA}' AND table_name='outbox_events'" \
  | grep -q '^1$' || fail "tabela ${SCHEMA}.outbox_events ausente"

log "==> baseline: contagem de pending antes"
BEFORE=$(psql "$DATABASE_URL" -tAc \
  "SELECT count(*) FROM ${SCHEMA}.outbox_events WHERE published_at IS NULL AND status <> 3")
log "    pending atual: ${BEFORE}"

log "==> inserindo ${EVENT_COUNT} eventos sinteticos (run_id=${RUN_ID})"
psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -v count="${EVENT_COUNT}" -v run_id="${RUN_ID}" -v schema="${SCHEMA}" >/dev/null <<'SQL'
DO $$
DECLARE
  c integer := :'count'::integer;
  rid text := :'run_id';
  s text := :'schema';
  i integer;
  evt_id uuid;
BEGIN
  FOR i IN 1..c LOOP
    evt_id := gen_random_uuid();
    EXECUTE format(
      'INSERT INTO %I.outbox_events (event_id, aggregate_type, aggregate_id, aggregate_user_id, event_type, payload, occurred_at, status, attempts, created_at)
       VALUES ($1, $2, $3, NULL, $4, $5::jsonb, now(), 0, 0, now())',
      s
    ) USING evt_id, 'loadtest', evt_id, 'loadtest.synthetic.v1',
            jsonb_build_object('run_id', rid, 'seq', i);
  END LOOP;
END$$;
SQL

INSERTED_TS=$(date +%s)
log "==> aguardando drenagem (timeout ${TIMEOUT_SEC}s)"

DEADLINE=$((INSERTED_TS + TIMEOUT_SEC))
LAST_REMAIN=-1
while :; do
  REMAIN=$(psql "$DATABASE_URL" -tAc \
    "SELECT count(*) FROM ${SCHEMA}.outbox_events
     WHERE event_type='loadtest.synthetic.v1'
       AND payload->>'run_id' = '${RUN_ID}'
       AND published_at IS NULL
       AND status <> 3")
  NOW_TS=$(date +%s)
  if [ "$REMAIN" = "0" ]; then
    ELAPSED=$((NOW_TS - INSERTED_TS))
    if [ "$ELAPSED" -lt 1 ]; then ELAPSED=1; fi
    THROUGHPUT=$(( EVENT_COUNT / ELAPSED ))
    log "PASS: ${EVENT_COUNT} eventos drenados em ${ELAPSED}s (~${THROUGHPUT} events/s)"
    exit 0
  fi
  if [ "$REMAIN" != "$LAST_REMAIN" ]; then
    log "    remaining=${REMAIN} elapsed=$((NOW_TS - INSERTED_TS))s"
    LAST_REMAIN=$REMAIN
  fi
  if [ "$NOW_TS" -ge "$DEADLINE" ]; then
    log "FAIL: timeout — ${REMAIN} eventos ainda pendentes apos ${TIMEOUT_SEC}s"
    exit 2
  fi
  sleep 2
done
