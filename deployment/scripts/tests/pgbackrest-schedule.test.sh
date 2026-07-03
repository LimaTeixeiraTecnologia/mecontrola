#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
DEPLOY_SCRIPT="${ROOT_DIR}/deployment/scripts/deploy-swarm.sh"
RENDER_SCRIPT="${ROOT_DIR}/deployment/scripts/render-stack.py"
SCHEDULE_SCRIPT="${ROOT_DIR}/deployment/scripts/pgbackrest-schedule.sh"
TMP_DIR=""
FAILS=0

cleanup() {
  [[ -n "$TMP_DIR" ]] && rm -rf "$TMP_DIR"
}
trap cleanup EXIT

assert_exit() {
  local name="$1"
  local expected="$2"
  local actual="$3"
  if [[ "$actual" -ne "$expected" ]]; then
    echo "[FAIL] ${name}: esperado exit ${expected}, obtido ${actual}"
    FAILS=$((FAILS + 1))
    return
  fi
  echo "[PASS] ${name}"
}

assert_output_contains() {
  local name="$1"
  local pattern="$2"
  local output="$3"
  if ! echo "$output" | grep -q "$pattern"; then
    echo "[FAIL] ${name}: saida nao contem '${pattern}'"
    echo "  saida: ${output}"
    FAILS=$((FAILS + 1))
    return
  fi
  echo "[PASS] ${name}"
}

setup() {
  TMP_DIR=$(mktemp -d)
}

# ===== Guard tests: deploy-swarm.sh =====

test_guard_deploy_default_image_fails() {
  setup
  local env_file="${TMP_DIR}/prod.env"
  local secrets_file="${TMP_DIR}/secrets.env"
  printf 'ENVIRONMENT=production\nPOSTGRES_IMAGE=postgres:16-alpine\n' > "$env_file"
  printf 'DB_PASSWORD=dummy\n' > "$secrets_file"

  local out
  set +e
  out=$(
    VPS_HOST="mock-vps" \
    VPS_USER="mock-user" \
    VPS_DEPLOY_PATH="/opt/mecontrola" \
    PROD_ENV_FILE="$env_file" \
    bash "$DEPLOY_SCRIPT" "tag123" "$secrets_file" 2>&1
  )
  local code=$?
  set -e

  assert_exit "guard deploy: imagem default falha" 1 "$code"
  assert_output_contains "guard deploy: mensagem de erro correta" "POSTGRES_IMAGE nao e a imagem custom" "$out"
}

test_guard_deploy_empty_image_fails() {
  setup
  local env_file="${TMP_DIR}/prod.env"
  local secrets_file="${TMP_DIR}/secrets.env"
  printf 'ENVIRONMENT=production\n' > "$env_file"
  printf 'DB_PASSWORD=dummy\n' > "$secrets_file"

  local out
  set +e
  out=$(
    VPS_HOST="mock-vps" \
    VPS_USER="mock-user" \
    VPS_DEPLOY_PATH="/opt/mecontrola" \
    PROD_ENV_FILE="$env_file" \
    bash "$DEPLOY_SCRIPT" "tag123" "$secrets_file" 2>&1
  )
  local code=$?
  set -e

  assert_exit "guard deploy: imagem vazia falha" 1 "$code"
  assert_output_contains "guard deploy: mensagem vazio" "POSTGRES_IMAGE nao e a imagem custom" "$out"
}

test_guard_deploy_custom_image_passes_to_swarm_check() {
  setup
  local env_file="${TMP_DIR}/prod.env"
  local secrets_file="${TMP_DIR}/secrets.env"
  printf 'ENVIRONMENT=production\nPOSTGRES_IMAGE=ghcr.io/limateixeiratecnologia/mecontrola-postgres:test-tag\n' > "$env_file"
  printf 'DB_PASSWORD=dummy\n' > "$secrets_file"

  local mock_bin="${TMP_DIR}/bin"
  mkdir -p "$mock_bin"
  cat > "${mock_bin}/ssh" <<'MOCK'
#!/usr/bin/env bash
args=("$@")
for i in "${!args[@]}"; do
  if [[ "${args[$i]}" == *"@"* ]]; then
    idx=$((i + 1))
    exec_line="${args[@]:$idx}"
    if echo "$exec_line" | grep -q "docker info"; then
      echo "active"
      exit 0
    fi
    echo "mock: $exec_line" >&2
    exit 1
  fi
done
exit 1
MOCK
  chmod +x "${mock_bin}/ssh"

  local out
  set +e
  out=$(
    PATH="${mock_bin}:${PATH}" \
    VPS_HOST="mock-vps" \
    VPS_USER="mock-user" \
    VPS_DEPLOY_PATH="/opt/mecontrola" \
    PROD_ENV_FILE="$env_file" \
    bash "$DEPLOY_SCRIPT" "tag123" "$secrets_file" 2>&1
  )
  local code=$?
  set -e

  assert_output_contains "guard deploy: imagem custom passa validacao" "POSTGRES_IMAGE validada" "$out"
}

# ===== Guard tests: render-stack.py =====

test_guard_render_default_image_fails() {
  setup
  local env_file="${TMP_DIR}/prod.env"
  printf 'ENVIRONMENT=production\nPOSTGRES_IMAGE=postgres:16-alpine\n' > "$env_file"

  local out
  set +e
  out=$(python3 "$RENDER_SCRIPT" /dev/null --env-file "$env_file" 2>&1)
  local code=$?
  set -e

  assert_exit "guard render: imagem default falha" 1 "$code"
  assert_output_contains "guard render: mensagem de erro" "POSTGRES_IMAGE" "$out"
}

test_guard_render_empty_image_fails() {
  setup
  local env_file="${TMP_DIR}/prod.env"
  printf 'ENVIRONMENT=production\n' > "$env_file"

  local out
  set +e
  out=$(python3 "$RENDER_SCRIPT" /dev/null --env-file "$env_file" 2>&1)
  local code=$?
  set -e

  assert_exit "guard render: imagem vazia falha" 1 "$code"
  assert_output_contains "guard render: mensagem vazio" "POSTGRES_IMAGE" "$out"
}

test_guard_render_non_production_skips_guard() {
  setup
  local env_file="${TMP_DIR}/prod.env"
  printf 'ENVIRONMENT=staging\nPOSTGRES_IMAGE=postgres:16-alpine\n' > "$env_file"

  local out
  set +e
  out=$(python3 "$RENDER_SCRIPT" /dev/null --env-file "$env_file" 2>&1)
  local code=$?
  set -e

  if echo "$out" | grep -q "POSTGRES_IMAGE nao e a imagem custom"; then
    echo "[FAIL] guard render: staging nao deveria acionar guard de imagem"
    FAILS=$((FAILS + 1))
  else
    echo "[PASS] guard render: staging ignora guard de imagem"
  fi
}

# ===== Schedule script sanity checks =====

test_schedule_script_is_executable() {
  if [[ -x "$SCHEDULE_SCRIPT" ]]; then
    echo "[PASS] pgbackrest-schedule.sh e executavel"
  else
    echo "[FAIL] pgbackrest-schedule.sh nao e executavel"
    FAILS=$((FAILS + 1))
  fi
}

test_metrics_script_is_executable() {
  local metrics_script="${ROOT_DIR}/deployment/scripts/pgbackrest-backup-metrics.sh"
  if [[ -x "$metrics_script" ]]; then
    echo "[PASS] pgbackrest-backup-metrics.sh e executavel"
  else
    echo "[FAIL] pgbackrest-backup-metrics.sh nao e executavel"
    FAILS=$((FAILS + 1))
  fi
}

test_schedule_script_syntax() {
  local out
  set +e
  out=$(bash -n "$SCHEDULE_SCRIPT" 2>&1)
  local code=$?
  set -e
  assert_exit "pgbackrest-schedule.sh sintaxe bash" 0 "$code"
}

test_metrics_script_syntax() {
  local metrics_script="${ROOT_DIR}/deployment/scripts/pgbackrest-backup-metrics.sh"
  local out
  set +e
  out=$(bash -n "$metrics_script" 2>&1)
  local code=$?
  set -e
  assert_exit "pgbackrest-backup-metrics.sh sintaxe bash" 0 "$code"
}

# ===== Run all tests =====

test_guard_deploy_default_image_fails
test_guard_deploy_empty_image_fails
test_guard_deploy_custom_image_passes_to_swarm_check
test_guard_render_default_image_fails
test_guard_render_empty_image_fails
test_guard_render_non_production_skips_guard
test_schedule_script_is_executable
test_metrics_script_is_executable
test_schedule_script_syntax
test_metrics_script_syntax

echo ""
if [[ "$FAILS" -gt 0 ]]; then
  echo "${FAILS} cenario(s) falharam"
  exit 1
fi

echo "Todos os cenarios passaram"
