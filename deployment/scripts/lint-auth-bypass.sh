#!/usr/bin/env bash
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
cd "$repo_root"

INJECT_PATTERN='InjectPrincipalFromHeader(WithO11y)?'
WINDOW=3

violations=()

while IFS= read -r file; do
  while IFS= read -r match_line; do
    start=$(( match_line - WINDOW ))
    [[ $start -lt 1 ]] && start=1
    end=$(( match_line - 1 ))

    if [[ $end -lt $start ]]; then
      violations+=("${file}:${match_line}: InjectPrincipalFromHeader sem RequireGatewayAuth antes (nenhuma linha anterior)")
      continue
    fi

    previous_use=$(sed -n "${start},${end}p" "$file" | grep -E 'sub\.Use\(' | tail -n1 || true)
    if [[ -z "$previous_use" ]]; then
      violations+=("${file}:${match_line}: InjectPrincipalFromHeader sem middleware anterior verificavel")
      continue
    fi

    if ! echo "$previous_use" | grep -qE 'sub\.Use\([^)]*(RequireGatewayAuth|gatewayAuthMiddleware|rt\.gatewayAuth)'; then
      violations+=("${file}:${match_line}: InjectPrincipalFromHeader sem RequireGatewayAuth nas ${WINDOW} linhas anteriores")
    fi
  done < <(grep -nE "$INJECT_PATTERN" "$file" | cut -d: -f1)
done < <(find internal \( -type f -name '*router*.go' -o -type f -name '*routes*.go' \) 2>/dev/null \
  | grep -v '_test\.go' \
  | grep -v '/mocks/')

if [[ ${#violations[@]} -gt 0 ]]; then
  echo "FAIL lint:auth-bypass — InjectPrincipalFromHeader sem RequireGatewayAuth imediatamente antes (M-09 violado):" >&2
  for v in "${violations[@]}"; do
    echo "  $v" >&2
  done
  echo "" >&2
  echo "Politica: todo router que usa InjectPrincipalFromHeader DEVE ter RequireGatewayAuth nas ${WINDOW} linhas anteriores." >&2
  echo "Ver .specs/prd-gateway-auth-forensics/techspec.md secao 'Fluxo de Dados' e ADR-004." >&2
  exit 1
fi

echo "PASS lint:auth-bypass — RequireGatewayAuth presente antes de InjectPrincipalFromHeader em todos os routers inspecionados"
