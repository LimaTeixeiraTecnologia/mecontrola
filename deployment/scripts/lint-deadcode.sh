#!/usr/bin/env bash
#
# Gate RF-40: deadcode em internal/agent bloqueia o build.
#
# Roda `deadcode ./cmd/...` (entrypoints cmd/server + cmd/worker) e falha
# quando reporta qualquer funcao inalcancavel em internal/agent que NAO esteja
# na allowlist de simbolos intencionalmente preservados.
#
# A allowlist (deadcode-agent-allowlist.txt) documenta cada simbolo mantido por
# simetria de API (closed-type Parse*/String*), satisfacao de interface via
# registry, ou feature HITL ainda nao 100% wired. Qualquer NOVO codigo morto em
# internal/agent fora da allowlist falha o gate.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
ALLOWLIST="$SCRIPT_DIR/deadcode-agent-allowlist.txt"

cd "$REPO_ROOT"

DEADCODE_BIN="$(command -v deadcode || true)"
if [ -z "$DEADCODE_BIN" ]; then
  GOBIN_DIR="$(go env GOBIN)"
  [ -z "$GOBIN_DIR" ] && GOBIN_DIR="$(go env GOPATH)/bin"
  if [ -x "$GOBIN_DIR/deadcode" ]; then
    DEADCODE_BIN="$GOBIN_DIR/deadcode"
  else
    echo "==> deadcode ausente; instalando golang.org/x/tools/cmd/deadcode@latest..."
    go install golang.org/x/tools/cmd/deadcode@latest
    DEADCODE_BIN="$GOBIN_DIR/deadcode"
  fi
fi

if [ ! -r "$ALLOWLIST" ]; then
  echo "FAIL: allowlist ausente em $ALLOWLIST"
  exit 1
fi

RAW="$("$DEADCODE_BIN" ./cmd/... 2>/dev/null | grep "internal/agent/" || true)"

VIOLATIONS=""
while IFS= read -r line; do
  [ -z "$line" ] && continue
  file="${line%%:*}"
  func="${line##*unreachable func: }"
  key="$file:$func"
  if ! grep -Fxq "$key" "$ALLOWLIST"; then
    VIOLATIONS="$VIOLATIONS$line"$'\n'
  fi
done <<< "$RAW"

if [ -n "${VIOLATIONS//[$'\n']/}" ]; then
  echo "FAIL: codigo morto em internal/agent fora da allowlist (RF-40):"
  printf '%s' "$VIOLATIONS"
  echo ""
  echo "Remova o codigo morto ou justifique a manutencao em deadcode-agent-allowlist.txt."
  exit 1
fi

echo "PASS lint:deadcode: nenhum codigo morto acionavel em internal/agent (RF-40)"
