#!/usr/bin/env bash
# Hook PreToolUse: validates observable governance prerequisites and emits
# surgical guidance before code edits.
#
# The hook cannot prove that an agent has read AGENTS.md in the current model
# context. Blocking on an environment variable by default creates permanent
# false positives in fresh sessions. Use GOVERNANCE_PRELOAD_MODE=fail when a
# caller explicitly wants that manual confirmation gate.

set -euo pipefail

GOVERNANCE_PRELOAD_MODE="${GOVERNANCE_PRELOAD_MODE:-warn}"
GOVERNANCE_PRELOAD_CONFIRMED="${GOVERNANCE_PRELOAD_CONFIRMED:-0}"

HOOK_DIR="$(cd "$(dirname "$0")" && pwd)"
if [[ -n "${AGENTS_ROOT:-}" ]]; then
  PROJECT_ROOT="$AGENTS_ROOT"
else
  PROJECT_ROOT="$(git -C "$HOOK_DIR/../.." rev-parse --show-toplevel 2>/dev/null || true)"
  [[ -n "$PROJECT_ROOT" ]] || PROJECT_ROOT="$(cd "$HOOK_DIR/../.." && pwd)"
fi

parse_lib=""
for candidate in \
  "$PROJECT_ROOT/.agents/lib/parse-hook-input.sh" \
  "$PROJECT_ROOT/scripts/lib/parse-hook-input.sh"
do
  if [[ -r "$candidate" ]]; then
    parse_lib="$candidate"
    break
  fi
done
[[ -n "$parse_lib" ]] || { echo "AVISO: parse-hook-input.sh nao encontrado" >&2; exit 0; }
# shellcheck source=/dev/null
source "$parse_lib"

_stdin=""
if [[ ! -t 0 ]]; then
  _stdin="$(cat)"
fi

file_path="${1:-}"
if [[ -z "$file_path" && -n "$_stdin" ]]; then
  file_path="$(printf '%s' "$_stdin" | parse_file_path)"
fi

[[ -n "$file_path" ]] || exit 0

prereq_gate="$PROJECT_ROOT/.agents/scripts/hook-prereq-gate.sh"
if [[ -f "$prereq_gate" ]]; then
  if ! printf '%s' "$_stdin" | AGENTS_ROOT="$PROJECT_ROOT" bash "$prereq_gate" "$file_path"; then
    exit 1
  fi
fi

case "$file_path" in
  *.go|*.py|*.ts|*.js|*.tsx|*.jsx|*.mjs|*.cjs|*.cs|*.csproj)
    if [[ "$GOVERNANCE_PRELOAD_CONFIRMED" == "1" ]]; then
      exit 0
    fi

    echo "LEMBRETE: confirme que AGENTS.md e as skills obrigatorias foram carregadas para editar $file_path." >&2
    if [[ "$GOVERNANCE_PRELOAD_MODE" == "fail" ]]; then
      echo "GOVERNANCE_PRELOAD_MODE=fail: bloqueando edicao ate confirmacao explicita." >&2
      echo "Para prosseguir: export GOVERNANCE_PRELOAD_CONFIRMED=1" >&2
      exit 1
    fi
    ;;
esac

exit 0
