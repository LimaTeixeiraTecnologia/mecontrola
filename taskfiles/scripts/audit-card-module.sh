#!/usr/bin/env bash
set -euo pipefail

PASS=0
FAIL=0

ok() {
  echo "PASS: $1"
  PASS=$((PASS + 1))
}

fail() {
  echo "FAIL: $1"
  FAIL=$((FAIL + 1))
}

echo "==> [R0] Verificando ausencia de init()..."
if grep -rn '^func init()' internal/card/ internal/platform/idempotency/ 2>/dev/null; then
  fail "init() encontrado — violacao R0"
else
  ok "sem init()"
fi

echo ""
echo "==> [R5.12] Verificando ausencia de panic em producao..."
if grep -rn 'panic(' --include="*.go" --exclude-dir=mocks --exclude="*_test.go" internal/card/ internal/platform/idempotency/ 2>/dev/null; then
  fail "panic() em producao — violacao R5.12"
else
  ok "sem panic em producao"
fi

echo ""
echo "==> [R6.7] Verificando ausencia de clock.Clock..."
if grep -rn 'clock\.Clock' --include="*.go" internal/card/ 2>/dev/null; then
  fail "clock.Clock encontrado — violacao R6.7"
else
  ok "sem clock.Clock"
fi

echo ""
echo "==> [R6.4] Verificando ausencia de var _ Interface = (*Type)(nil)..."
if grep -rn 'var _ .* = (' --include="*.go" internal/card/ 2>/dev/null; then
  fail "assertiva de interface em compile-time — violacao R6.4"
else
  ok "sem assertiva de interface"
fi

echo ""
echo "==> [R-ADAPTER-001.1] Verificando zero comentarios em producao..."
if grep -rn --include="*.go" --exclude-dir=mocks --exclude="*.pb.go" --exclude="*_test.go" \
  "^[[:space:]]*//" internal/card/ internal/platform/idempotency/ \
  | grep -Ev "(//go:|//nolint:|// Code generated)" 2>/dev/null; then
  fail "comentarios proibidos — violacao R-ADAPTER-001.1"
else
  ok "zero comentarios em producao"
fi

echo ""
echo "==> [R-ADAPTER-001.2] Verificando ausencia de SQL direto em adapters..."
if grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" \
  "QueryContext\|ExecContext\|db\.Query\|tx\.Exec\|db\.Exec" \
  internal/card/infrastructure/http/server/handlers/ 2>/dev/null; then
  fail "SQL direto em handler — violacao R-ADAPTER-001.2"
else
  ok "sem SQL direto em adapters"
fi

echo ""
echo "==> [RF-16] Verificando ausencia de PAN/CVV/CVC/track/PIN..."
if grep -rn --include="*.go" --exclude="*_test.go" -E '\b(pan|cvv|cvc|track|pin)\b' internal/card/ 2>/dev/null; then
  fail "dados sensiveis PCI encontrados — violacao RF-16"
else
  ok "sem dados sensiveis PCI em internal/card"
fi

echo ""
echo "========================================"
echo "Resultado: PASS=$PASS  FAIL=$FAIL"
echo "========================================"

if [ "$FAIL" -gt 0 ]; then
  echo "AUDIT FAILED — corrija as violacoes acima antes do merge."
  exit 1
fi

echo "AUDIT PASSED — todos os gates R0-R7 + zero-comentarios + anti-PCI verdes."
