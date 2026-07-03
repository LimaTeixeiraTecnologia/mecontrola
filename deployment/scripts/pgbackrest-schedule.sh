#!/usr/bin/env bash
set -euo pipefail

STACK="${STACK:-mecontrola}"
POSTGRES_SERVICE="${POSTGRES_SERVICE:-postgres}"
STANZA="${STANZA:-mecontrola}"
SCRIPTS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WRAPPER="/usr/local/bin/pgbackrest-run.sh"
METRICS_SCRIPT="${SCRIPTS_DIR}/pgbackrest-backup-metrics.sh"
LOG_DIR="/var/log/pgbackrest"

log() { echo "[$(date -u +%Y-%m-%dT%H:%M:%SZ)] pgbackrest-schedule: $*"; }

install_wrapper() {
  cat > "$WRAPPER" <<EOF
#!/usr/bin/env bash
set -euo pipefail
CONTAINER=\$(docker ps --filter "name=${STACK}_${POSTGRES_SERVICE}" --format "{{.Names}}" | head -1)
if [[ -z "\$CONTAINER" ]]; then
  echo "pgbackrest-run: container ${STACK}_${POSTGRES_SERVICE} nao encontrado" >&2
  exit 1
fi
exec docker exec "\$CONTAINER" pgbackrest --stanza=${STANZA} "\$@"
EOF
  chmod +x "$WRAPPER"
  log "Wrapper instalado: $WRAPPER"
}

install_systemd() {
  cat > /etc/systemd/system/pgbackrest-full.service <<EOF
[Unit]
Description=pgBackRest full backup para ${STACK}
After=docker.service
Requires=docker.service

[Service]
Type=oneshot
ExecStart=${WRAPPER} --type=full backup
StandardOutput=journal
StandardError=journal
EOF

  cat > /etc/systemd/system/pgbackrest-full.timer <<EOF
[Unit]
Description=pgBackRest backup full semanal

[Timer]
OnCalendar=Sun 05:00:00 UTC
Persistent=true

[Install]
WantedBy=timers.target
EOF

  cat > /etc/systemd/system/pgbackrest-diff.service <<EOF
[Unit]
Description=pgBackRest differential backup para ${STACK}
After=docker.service
Requires=docker.service

[Service]
Type=oneshot
ExecStart=${WRAPPER} --type=diff backup
StandardOutput=journal
StandardError=journal
EOF

  cat > /etc/systemd/system/pgbackrest-diff.timer <<EOF
[Unit]
Description=pgBackRest backup diferencial diario (Seg-Sab)

[Timer]
OnCalendar=Mon..Sat 05:00:00 UTC
Persistent=true

[Install]
WantedBy=timers.target
EOF

  cat > /etc/systemd/system/pgbackrest-incr.service <<EOF
[Unit]
Description=pgBackRest incremental backup para ${STACK}
After=docker.service
Requires=docker.service

[Service]
Type=oneshot
ExecStart=${WRAPPER} --type=incr backup
StandardOutput=journal
StandardError=journal
EOF

  cat > /etc/systemd/system/pgbackrest-incr.timer <<EOF
[Unit]
Description=pgBackRest backup incremental a cada 6h

[Timer]
OnCalendar=*-*-* 0/6:00:00 UTC
Persistent=true

[Install]
WantedBy=timers.target
EOF

  cat > /etc/systemd/system/pgbackrest-metrics.service <<EOF
[Unit]
Description=pgBackRest exportador de metricas de backup

[Service]
Type=oneshot
ExecStart=${METRICS_SCRIPT}
StandardOutput=journal
StandardError=journal
EOF

  cat > /etc/systemd/system/pgbackrest-metrics.timer <<EOF
[Unit]
Description=pgBackRest coleta de metricas a cada 30 min

[Timer]
OnCalendar=*:0/30
Persistent=true

[Install]
WantedBy=timers.target
EOF

  systemctl daemon-reload

  for unit in pgbackrest-full pgbackrest-diff pgbackrest-incr pgbackrest-metrics; do
    systemctl enable --now "${unit}.timer"
    log "Timer habilitado: ${unit}.timer"
  done
}

install_cron() {
  mkdir -p "$LOG_DIR"
  cat > /etc/cron.d/pgbackrest <<EOF
SHELL=/bin/bash
PATH=/usr/local/sbin:/usr/local/bin:/sbin:/bin:/usr/sbin:/usr/bin

0 5 * * 0 root ${WRAPPER} --type=full backup >> ${LOG_DIR}/backup-full.log 2>&1
0 5 * * 1-6 root ${WRAPPER} --type=diff backup >> ${LOG_DIR}/backup-diff.log 2>&1
0 */6 * * * root ${WRAPPER} --type=incr backup >> ${LOG_DIR}/backup-incr.log 2>&1
*/30 * * * * root ${METRICS_SCRIPT} >> ${LOG_DIR}/metrics.log 2>&1
EOF
  chmod 640 /etc/cron.d/pgbackrest
  log "Cron instalado: /etc/cron.d/pgbackrest"
}

log "Iniciando instalacao do agendamento pgBackRest (idempotente)"

install_wrapper

if command -v systemctl >/dev/null 2>&1 && systemctl is-system-running >/dev/null 2>&1; then
  log "Usando systemd-timer"
  install_systemd
else
  log "systemd nao disponivel, usando cron fallback"
  install_cron
fi

log "Agendamento pgBackRest instalado com sucesso"
log "Wrapper disponivel em: ${WRAPPER}"
log "Metricas exportadas por: ${METRICS_SCRIPT}"
log "Para verificar: pgbackrest --stanza=${STANZA} info"
