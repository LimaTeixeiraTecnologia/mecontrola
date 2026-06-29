#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$REPO_ROOT"

FAIL=0

echo "==> [1/7] R-ADAPTER-001: SQL direto em internal/agents application e tools"
if grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" \
  "QueryContext\|ExecContext\|db\.Query\|tx\.Exec\|db\.Exec" \
  internal/agents/application/ internal/agents/infrastructure/ 2>/dev/null \
  | grep -v "infrastructure/repositories/postgres"; then
  echo "FAIL: SQL direto em internal/agents application/infrastructure"
  FAIL=1
else
  echo "OK"
fi

echo "==> [2/7] R-ADAPTER-001: import de repo/infra de outro BC em internal/agents"
if grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" \
  "internal/transactions/infrastructure/repositories\|internal/budgets/infrastructure/repositories\|internal/card/infrastructure/repositories\|internal/categories/infrastructure/repositories\|internal/onboarding/infrastructure/repositories\|internal/identity/infrastructure/repositories" \
  internal/agents/ 2>/dev/null; then
  echo "FAIL: import de repo/infra de outro BC em internal/agents"
  FAIL=1
else
  echo "OK"
fi

echo "==> [3/7] R-ADAPTER-001.1: zero comentarios em Go de producao"
if grep -rn --include="*.go" --exclude-dir=mocks --exclude="*.pb.go" --exclude="*_test.go" \
  "^[[:space:]]*//" internal/ configs/ cmd/ 2>/dev/null \
  | grep -Ev "(//go:|//nolint:|// Code generated)"; then
  echo "FAIL: comentarios proibidos em Go de producao"
  FAIL=1
else
  echo "OK"
fi

echo "==> [4/7] R-ADAPTER-001.2: SQL direto em adapters (handlers/consumers/producers/jobs)"
if grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" \
  "QueryContext\|ExecContext\|db\.Query\|tx\.Exec\|db\.Exec" \
  internal/*/infrastructure/http/server/handlers/ \
  internal/*/infrastructure/messaging/database/consumers/ \
  internal/*/infrastructure/messaging/database/producers/ \
  internal/*/infrastructure/jobs/handlers/ 2>/dev/null; then
  echo "FAIL: SQL direto em adapter"
  FAIL=1
else
  echo "OK"
fi

echo "==> [5/7] R-WF-KERNEL-001: import de dominio ou camada superior em kernel"
if grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" \
  "internal/platform/agent\|internal/platform/memory\|internal/transactions\|internal/billing\|internal/identity" \
  internal/platform/workflow/ 2>/dev/null; then
  echo "FAIL: import de dominio em workflow kernel"
  FAIL=1
else
  echo "OK"
fi

echo "==> [6/7] R-WF-KERNEL-001.4 + R-AGENT-WF-001.5: labels de alta cardinalidade em metricas"
if grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" \
  '"user_id"\|"correlation_key"\|"category_id"' \
  internal/platform/workflow/ 2>/dev/null; then
  echo "FAIL: label de alta cardinalidade em metrica do kernel"
  FAIL=1
else
  echo "OK"
fi

echo "==> [7/7] R-AGENT-WF-001.3 + R-WF-KERNEL-001.3: estados como string solta"
if grep -rn --include="*.go" --exclude-dir=mocks --exclude="*_test.go" \
  "RunStatus\s*=\s*\"[^\"]*\"\|StepStatus\s*=\s*\"[^\"]*\"\|SuspendReason\s*=\s*\"[^\"]*\"\|AwaitingApproval\s*=\s*\"[^\"]*\"\|OperationKind\s*=\s*\"[^\"]*\"" \
  internal/ 2>/dev/null; then
  echo "FAIL: estado como string solta"
  FAIL=1
else
  echo "OK"
fi

if [ "$FAIL" -ne 0 ]; then
  echo ""
  echo "GATE FALHOU: corrigir as violacoes acima antes do merge."
  exit 1
fi

echo ""
echo "GATE VERDE: todas as fronteiras de dados e governanca validadas."
exit 0
