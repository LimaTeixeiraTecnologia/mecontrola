#!/usr/bin/env bash
set -euo pipefail

STACK="${STACK:-mecontrola}"
POSTGRES_SERVICE="${POSTGRES_SERVICE:-postgres}"
STANZA="${STANZA:-mecontrola}"
TEXTFILE_DIR="${TEXTFILE_DIR:-/var/lib/node_exporter/textfile_collector}"
OUTFILE="${TEXTFILE_DIR}/pgbackrest.prom"
TMP="${OUTFILE}.tmp.$$"

log() { echo "[$(date -u +%Y-%m-%dT%H:%M:%SZ)] pgbackrest-metrics: $*"; }

cleanup() { rm -f "$TMP"; }
trap cleanup EXIT

CONTAINER="$(docker ps --filter "name=${STACK}_${POSTGRES_SERVICE}\." --format "{{.Names}}" | head -1 || true)"
if [[ -z "$CONTAINER" ]]; then
  log "Container ${STACK}_${POSTGRES_SERVICE} nao encontrado, nada a exportar"
  exit 0
fi

mkdir -p "$TEXTFILE_DIR"

TMP_JSON="/tmp/pgbackrest-info-$$.json"
trap 'rm -f "$TMP_JSON" "$TMP"' EXIT
docker exec "$CONTAINER" pgbackrest --stanza="$STANZA" info --output=json > "$TMP_JSON" 2>/dev/null || echo "[]" > "$TMP_JSON"

now="$(date +%s)"
last_full=0
last_diff=0
last_incr=0

if command -v python3 >/dev/null 2>&1; then
  read -r last_full last_diff last_incr < <(python3 - "$TMP_JSON" <<'PYEOF'
import json, sys

with open(sys.argv[1]) as f:
    data = json.load(f)

last_full = 0
last_diff = 0
last_incr = 0

for stanza in (data if isinstance(data, list) else []):
    for b in stanza.get("backup", []):
        btype = b.get("type", "")
        stop_ts = int(b.get("timestamp", {}).get("stop", 0))
        if btype == "full":
            last_full = max(last_full, stop_ts)
        elif btype == "diff":
            last_diff = max(last_diff, stop_ts)
        elif btype == "incr":
            last_incr = max(last_incr, stop_ts)

print(last_full, last_diff, last_incr)
PYEOF
  )
fi

archive_push_failed=0
if ! docker exec "$CONTAINER" pgbackrest --stanza="$STANZA" check >/dev/null 2>&1; then
  archive_push_failed=1
fi

age_full=$(( last_full > 0 ? now - last_full : 999999 ))
age_diff=$(( last_diff > 0 ? now - last_diff : 999999 ))
age_incr=$(( last_incr > 0 ? now - last_incr : 999999 ))

cat > "$TMP" <<PROM
# HELP pgbackrest_backup_last_success_timestamp_seconds Epoch do ultimo backup bem-sucedido por tipo
# TYPE pgbackrest_backup_last_success_timestamp_seconds gauge
pgbackrest_backup_last_success_timestamp_seconds{stanza="${STANZA}",type="full"} ${last_full}
pgbackrest_backup_last_success_timestamp_seconds{stanza="${STANZA}",type="diff"} ${last_diff}
pgbackrest_backup_last_success_timestamp_seconds{stanza="${STANZA}",type="incr"} ${last_incr}
# HELP pgbackrest_backup_age_seconds Segundos desde o ultimo backup por tipo
# TYPE pgbackrest_backup_age_seconds gauge
pgbackrest_backup_age_seconds{stanza="${STANZA}",type="full"} ${age_full}
pgbackrest_backup_age_seconds{stanza="${STANZA}",type="diff"} ${age_diff}
pgbackrest_backup_age_seconds{stanza="${STANZA}",type="incr"} ${age_incr}
# HELP pgbackrest_archive_push_failed 1 se archive-push tem falhas detectadas pelo check, 0 caso contrario
# TYPE pgbackrest_archive_push_failed gauge
pgbackrest_archive_push_failed{stanza="${STANZA}"} ${archive_push_failed}
# HELP pgbackrest_metrics_collected_timestamp_seconds Epoch da ultima coleta de metricas
# TYPE pgbackrest_metrics_collected_timestamp_seconds gauge
pgbackrest_metrics_collected_timestamp_seconds{stanza="${STANZA}"} ${now}
PROM

mv "$TMP" "$OUTFILE"
log "Metricas exportadas para ${OUTFILE} (full_age=${age_full}s diff_age=${age_diff}s archive_failed=${archive_push_failed})"
