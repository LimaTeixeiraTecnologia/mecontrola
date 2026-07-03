#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$REPO_ROOT"

PASS=0
FAIL=0

_pass() { echo "PASS: $1"; PASS=$((PASS+1)); }
_fail() { echo "FAIL: $1"; FAIL=$((FAIL+1)); }

_run_gate() {
  local compose="$1"
  local otelcol="$2"
  local tmp_compose tmp_otelcol
  tmp_compose="$(mktemp /tmp/compose_XXXXXX.yml)"
  tmp_otelcol="$(mktemp /tmp/otelcol_XXXXXX.yaml)"
  cp "$compose" "$tmp_compose"
  cp "$otelcol" "$tmp_otelcol"
  COMPOSE="$tmp_compose" OTELCOL_OVERRIDE="$tmp_otelcol" bash scripts/ci/deploy-anti-storm.sh 2>&1
  local rc=$?
  rm -f "$tmp_compose" "$tmp_otelcol"
  return $rc
}

REAL_COMPOSE="deployment/compose/compose.swarm.yml"
REAL_OTELCOL="deployment/telemetry/grafana/otelcol-config.yaml"

echo "=== Teste 1: gate verde com configuracao valida ==="
if bash scripts/ci/deploy-anti-storm.sh > /dev/null 2>&1; then
  _pass "gate verde com compose e otelcol validos"
else
  _fail "gate falhou com configuracao valida"
fi

echo "=== Teste 2: gate falha quando tail_sampling ausente no otelcol ==="
TMP_OTELCOL="$(mktemp /tmp/otelcol_XXXXXX.yaml)"
grep -v "tail_sampling\|errors-policy\|probabilistic-policy\|decision_wait\|num_traces\|expected_new\|status_code\|sampling_percentage" "$REAL_OTELCOL" > "$TMP_OTELCOL"
ORIGINAL_OTELCOL="deployment/telemetry/grafana/otelcol-config.yaml"
cp "$TMP_OTELCOL" /tmp/_otelcol_test_backup.yaml
cp /tmp/_otelcol_test_backup.yaml "$TMP_OTELCOL"
rm -f "$TMP_OTELCOL"

TMP_OTELCOL_NOTAIL="$(mktemp /tmp/otelcol_notail_XXXXXX.yaml)"
grep -v "tail_sampling\|errors-policy\|probabilistic-policy\|decision_wait\|num_traces\|expected_new\|status_codes\|sampling_percentage" "$REAL_OTELCOL" > "$TMP_OTELCOL_NOTAIL" || true
sed -i '' 's/processors: \[memory_limiter, tail_sampling, batch\]/processors: [memory_limiter, batch]/' "$TMP_OTELCOL_NOTAIL" 2>/dev/null || \
  sed -i 's/processors: \[memory_limiter, tail_sampling, batch\]/processors: [memory_limiter, batch]/' "$TMP_OTELCOL_NOTAIL" 2>/dev/null || true

cp "$REAL_OTELCOL" /tmp/_otelcol_backup.yaml
cp "$TMP_OTELCOL_NOTAIL" "$REAL_OTELCOL"
if bash scripts/ci/deploy-anti-storm.sh > /dev/null 2>&1; then
  _fail "gate deveria falhar sem tail_sampling no otelcol"
else
  _pass "gate rejeita otelcol sem tail_sampling"
fi
cp /tmp/_otelcol_backup.yaml "$REAL_OTELCOL"
rm -f "$TMP_OTELCOL_NOTAIL"

echo "=== Teste 3: gate falha quando OTEL_TRACE_SAMPLE_RATE diverge (nao e 1) ==="
TMP_COMPOSE="$(mktemp /tmp/compose_XXXXXX.yml)"
sed 's/OTEL_TRACE_SAMPLE_RATE: "1"/OTEL_TRACE_SAMPLE_RATE: "0.1"/' "$REAL_COMPOSE" > "$TMP_COMPOSE"
cp "$REAL_COMPOSE" /tmp/_compose_backup.yaml
cp "$TMP_COMPOSE" "$REAL_COMPOSE"
if bash scripts/ci/deploy-anti-storm.sh > /dev/null 2>&1; then
  _fail "gate deveria falhar com OTEL_TRACE_SAMPLE_RATE=0.1 no compose"
else
  _pass "gate rejeita OTEL_TRACE_SAMPLE_RATE diferente de 1 no compose"
fi
cp /tmp/_compose_backup.yaml "$REAL_COMPOSE"
rm -f "$TMP_COMPOSE"

echo "=== Teste 4: gate falha quando errors-policy ausente no otelcol ==="
TMP_OTELCOL_NOERR="$(mktemp /tmp/otelcol_noerr_XXXXXX.yaml)"
grep -v "errors-policy\|status_code\|status_codes: \[ERROR\]" "$REAL_OTELCOL" > "$TMP_OTELCOL_NOERR" || true
cp "$REAL_OTELCOL" /tmp/_otelcol_backup2.yaml
cp "$TMP_OTELCOL_NOERR" "$REAL_OTELCOL"
if bash scripts/ci/deploy-anti-storm.sh > /dev/null 2>&1; then
  _fail "gate deveria falhar sem errors-policy no otelcol"
else
  _pass "gate rejeita otelcol sem errors-policy"
fi
cp /tmp/_otelcol_backup2.yaml "$REAL_OTELCOL"
rm -f "$TMP_OTELCOL_NOERR"

echo ""
echo "=== Resultado ==="
echo "PASS: $PASS  FAIL: $FAIL"
if [ "$FAIL" -ne 0 ]; then
  echo "RESULTADO: FALHOU"
  exit 1
fi
echo "RESULTADO: OK"
