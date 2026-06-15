#!/usr/bin/env bash
# deployment/scripts/pgbackrest-setup.sh
#
# Fase 1: configura pgBackRest (stanza-create + ativa archive_mode via ALTER SYSTEM).
# Fase 2: primeiro backup full (executar APÓS reiniciar o PostgreSQL).
#
# archive_mode é PGC_POSTMASTER: requer reinício do processo postgres.
# pg_reload_conf() NÃO é suficiente — o script avisa e encerra após ALTER SYSTEM.
#
# Uso:
#   Fase 1:
#     PGBACKREST_S3_BUCKET=meu-bucket \
#     PGBACKREST_S3_KEY=... \
#     PGBACKREST_S3_KEY_SECRET=... \
#     sudo ./pgbackrest-setup.sh
#
#   Fase 2 (após reiniciar o postgres):
#     PGBACKREST_S3_KEY=... PGBACKREST_S3_KEY_SECRET=... \
#     sudo ./pgbackrest-setup.sh --backup

set -euo pipefail

: "${PGBACKREST_S3_BUCKET:?PGBACKREST_S3_BUCKET obrigatório}"
: "${PGBACKREST_S3_KEY:?PGBACKREST_S3_KEY obrigatório}"
: "${PGBACKREST_S3_KEY_SECRET:?PGBACKREST_S3_KEY_SECRET obrigatório}"

STANZA=mecontrola
PGBACKREST_CONF=/etc/pgbackrest/pgbackrest.conf
RUN_BACKUP=false

for arg in "$@"; do
  [[ "$arg" == "--backup" ]] && RUN_BACKUP=true
done

if ! command -v pgbackrest >/dev/null 2>&1; then
  echo "==> Instalando pgBackRest..."
  apt-get update -qq
  DEBIAN_FRONTEND=noninteractive apt-get install -y -qq pgbackrest
fi

echo "==> Criando diretórios de log e spool..."
mkdir -p /var/log/pgbackrest /var/spool/pgbackrest
chown postgres:postgres /var/log/pgbackrest /var/spool/pgbackrest
chmod 750 /var/log/pgbackrest /var/spool/pgbackrest

if [[ ! -f "$PGBACKREST_CONF" ]]; then
  echo "ERRO: $PGBACKREST_CONF não encontrado." >&2
  echo "      Monte via Docker volume ou copie de deployment/pgbackrest/pgbackrest.conf." >&2
  exit 1
fi

PGB_FLAGS=(
  --config="$PGBACKREST_CONF"
  "--repo1-s3-key=$PGBACKREST_S3_KEY"
  "--repo1-s3-key-secret=$PGBACKREST_S3_KEY_SECRET"
  "--stanza=$STANZA"
)

if [[ "$RUN_BACKUP" == "true" ]]; then
  echo "==> Verificando archive antes do backup full..."
  pgbackrest "${PGB_FLAGS[@]}" check

  echo "==> Executando primeiro backup FULL..."
  pgbackrest "${PGB_FLAGS[@]}" --type=full backup

  echo ""
  echo "==> Backup full concluído."
  echo "    Instale o crontab: crontab -u postgres deployment/pgbackrest/crontab.txt"
  echo "    Exporte as variáveis S3 no ambiente do cron."
  exit 0
fi

echo "==> Criando stanza '$STANZA'..."
pgbackrest "${PGB_FLAGS[@]}" stanza-create

echo "==> Ativando archive_mode via ALTER SYSTEM..."
PGPASSWORD="${DB_PASSWORD:-}" psql \
  -h "${DB_HOST:-localhost}" \
  -p "${DB_PORT:-5432}" \
  -U "${DB_USER:-mecontrola}" \
  -d "${DB_NAME:-mecontrola_db}" \
  -c "ALTER SYSTEM SET archive_mode = on;" \
  -c "ALTER SYSTEM SET archive_command = 'pgbackrest --stanza=${STANZA} archive-push %p';"

echo ""
echo "=================================================================="
echo "  AÇÃO NECESSÁRIA: reiniciar o PostgreSQL antes do backup."
echo "  archive_mode=on requer reinício (PGC_POSTMASTER)."
echo "  pg_reload_conf() NÃO é suficiente."
echo ""
echo "  Docker: docker compose -f deployment/compose/compose.yml \\"
echo "          -f deployment/compose/compose.prod.yml restart postgres"
echo ""
echo "  Após reinício, execute a Fase 2:"
echo "    PGBACKREST_S3_KEY=... PGBACKREST_S3_KEY_SECRET=... \\"
echo "    sudo ./pgbackrest-setup.sh --backup"
echo "=================================================================="
