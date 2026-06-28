#!/usr/bin/env bash
set -euo pipefail

# Testes funcionais para deployment/scripts/deploy-swarm.sh
# Usam um mock de SSH para validar o fluxo sem acesso real à VPS.

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SCRIPT="${ROOT_DIR}/scripts/deploy-swarm.sh"
TMP_DIR=""
FAILS=0

cleanup() {
  [[ -n "$TMP_DIR" ]] && rm -rf "$TMP_DIR"
}
trap cleanup EXIT

setup_mock_ssh() {
  TMP_DIR=$(mktemp -d)
  mkdir -p "${TMP_DIR}/bin"

  cat > "${TMP_DIR}/bin/ssh" <<'MOCK'
#!/usr/bin/env bash
# Mock de SSH: ignora opções (-o ...) e host (user@host), usa o comando como chave.
CMD_FILE="${MOCK_RESPONSES:-/dev/null}"

# Extrai o comando: tudo após o argumento que contém '@'
args=("$@")
idx=0
for i in "${!args[@]}"; do
  if [[ "${args[$i]}" == *"@"* ]]; then
    idx=$((i + 1))
    break
  fi
done

if [[ "$idx" -ge "${#args[@]}" ]]; then
  echo "MOCK_SSH: nenhum comando após host: $*" >&2
  exit 1
fi

exec_line="${args[@]:$idx}"
CMD_LOG="${MOCK_CMD_LOG:-/dev/null}"
printf '%s\n' "$exec_line" >> "$CMD_LOG"

if [[ ! -f "$CMD_FILE" ]]; then
  echo "MOCK_SSH: nenhuma fixture em MOCK_RESPONSES" >&2
  exit 1
fi

# Normaliza aspas simples/duplas, escapes e múltiplos espaços para matching
cmd_norm=$(printf '%s' "$exec_line" | tr -d '\\"\047' | tr -s ' ')

found=false
response=""
while IFS= read -r line || [[ -n "$line" ]]; do
  [[ "$line" =~ ^#.*$ ]] && continue
  [[ -z "$line" ]] && continue
  pattern="${line%%::*}"
  response="${line#*::}"
  pattern_norm=$(printf '%s' "$pattern" | tr -d '\\"\047' | tr -s ' ')
  if [[ "$cmd_norm" == *"$pattern_norm"* ]]; then
    found=true
    break
  fi
done < "$CMD_FILE"

if [[ "$found" != true ]]; then
  echo "MOCK_SSH: comando não esperado: $exec_line" >&2
  exit 1
fi

if [[ "$response" == EXIT_* ]]; then
  code="${response#EXIT_}"
  exit "${code}"
fi

printf '%s\n' "$response"
exit 0
MOCK
  chmod +x "${TMP_DIR}/bin/ssh"
}

run_scenario() {
  local name="$1"
  local fixture="$2"
  local expected_exit="${3:-0}"
  local expect_rollback="${4:-false}"

  setup_mock_ssh
  export PATH="${TMP_DIR}/bin:${PATH}"
  export MOCK_RESPONSES="$fixture"
  export MOCK_CMD_LOG="${TMP_DIR}/commands.log"
  export VPS_HOST="mock-vps"
  export VPS_USER="mock-user"
  export VPS_DEPLOY_PATH="/opt/mecontrola"
  export IMAGE_TAG="newtag"
  export IMAGE_NAME="ghcr.io/limateixeiratecnologia/mecontrola"
  export HEALTH_RETRIES="2"
  export HEALTH_INTERVAL="1"
  export MIGRATE_TIMEOUT="30"

  set +e
  bash "$SCRIPT" "newtag" >"${TMP_DIR}/stdout.log" 2>"${TMP_DIR}/stderr.log"
  local code=$?
  set -e

  if [[ "$code" -ne "$expected_exit" ]]; then
    echo "[FAIL] ${name}: esperado exit ${expected_exit}, obtido ${code}"
    echo "stdout:"; cat "${TMP_DIR}/stdout.log"
    echo "stderr:"; cat "${TMP_DIR}/stderr.log"
    FAILS=$((FAILS + 1))
    return
  fi

  if [[ "$expect_rollback" == "true" ]]; then
    if ! grep -qE 'IMAGE_TAG=oldtag.*docker stack deploy' "${TMP_DIR}/commands.log" 2>/dev/null; then
      echo "[FAIL] ${name}: rollback para oldtag não foi executado"
      echo "comandos executados:"; cat "${TMP_DIR}/commands.log"
      FAILS=$((FAILS + 1))
      return
    fi
  fi

  echo "[PASS] ${name}"
}

# Fixture helper: arquivo com pares substring::resposta (uma linha por comando).
# O primeiro match é usado; a ordem importa quando há prefixos comuns.

# Cenário 1: deploy bem-sucedido
fixture_success=$(mktemp)
cat > "$fixture_success" <<'EOF'
docker info --format {{.Swarm.LocalNodeState}}::active
docker service inspect mecontrola_server-1 --format {{.Spec.TaskTemplate.ContainerSpec.Image}}::oldtag
git config --global --add safe.directory::
git pull --ff-only::Already up to date.
chmod 600 /opt/mecontrola/.env::
grep -qE '^AWS_ACCESS_KEY_ID::
deployment/scripts/create-secrets.sh::
deployment/scripts/backup-env-s3.sh::
docker run --rm --network mecontrola_backend --env-file /opt/mecontrola/.env::
docker stack deploy -c deployment/compose/compose.swarm.yml mecontrola::
docker service ps mecontrola_server-1 --format {{.CurrentState}}::Running 5 minutes ago
docker service ps mecontrola_server-2 --format {{.CurrentState}}::Running 5 minutes ago
docker service ps mecontrola_worker-1 --format {{.CurrentState}}::Running 5 minutes ago
docker service ps mecontrola_worker-2 --format {{.CurrentState}}::Running 5 minutes ago
docker ps --filter name=mecontrola_server-1 --filter health=healthy --format {{.Names}}::mecontrola_server-1.abc123
docker ps --filter name=mecontrola_server-2 --filter health=healthy --format {{.Names}}::mecontrola_server-2.def456
docker ps --filter name=mecontrola_worker-1 --filter health=healthy --format {{.Names}}::mecontrola_worker-1.ghi789
docker ps --filter name=mecontrola_worker-2 --filter health=healthy --format {{.Names}}::mecontrola_worker-2.jkl012
docker image prune -f --filter 'until=72h'::
EOF

run_scenario "deploy bem-sucedido" "$fixture_success" 0
rm -f "$fixture_success"

# Cenário 2: falha de health check → rollback
fixture_rollback=$(mktemp)
cat > "$fixture_rollback" <<'EOF'
docker info --format {{.Swarm.LocalNodeState}}::active
docker service inspect mecontrola_server-1 --format {{.Spec.TaskTemplate.ContainerSpec.Image}}::oldtag
git config --global --add safe.directory::
git pull --ff-only::Already up to date.
chmod 600 /opt/mecontrola/.env::
grep -qE '^AWS_ACCESS_KEY_ID::
deployment/scripts/create-secrets.sh::
deployment/scripts/backup-env-s3.sh::
docker run --rm --network mecontrola_backend --env-file /opt/mecontrola/.env::
docker stack deploy -c deployment/compose/compose.swarm.yml mecontrola::
docker service ps mecontrola_server-1 --format {{.CurrentState}}::Running 5 minutes ago
docker service ps mecontrola_server-2 --format {{.CurrentState}}::Running 5 minutes ago
docker service ps mecontrola_worker-1 --format {{.CurrentState}}::Running 5 minutes ago
docker service ps mecontrola_worker-2 --format {{.CurrentState}}::Running 5 minutes ago
docker ps --filter name=mecontrola_server-1 --filter health=healthy --format {{.Names}}::
docker ps --filter name=mecontrola_server-1 --filter health=healthy --format {{.Names}}::
docker stack deploy -c deployment/compose/compose.swarm.yml mecontrola::
EOF

run_scenario "rollback por falha de health check" "$fixture_rollback" 1 true
rm -f "$fixture_rollback"

if [[ "$FAILS" -gt 0 ]]; then
  echo ""
  echo "${FAILS} cenário(s) falharam"
  exit 1
fi

echo ""
echo "Todos os cenários passaram"
