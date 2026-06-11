#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
SCRIPT="$ROOT/.agents/scripts/resolve-references.sh"
FIXTURES_DIR="$ROOT/.agents/scripts/testdata/resolve-references"

run_scenario() {
  local scenario="$1"
  local args_file="$FIXTURES_DIR/$scenario/files.txt"
  local diff_file="$FIXTURES_DIR/$scenario/diff.txt"
  local expected_file="$FIXTURES_DIR/$scenario/expected.txt"

  files=()
  while IFS= read -r line || [[ -n "$line" ]]; do
    files+=("$line")
  done < "$args_file"
  local output

  if [[ -f "$diff_file" ]]; then
    output="$(RESOLVE_INCLUDE_METADATA=1 AGENTS_ROOT="$ROOT" bash "$SCRIPT" go-implementation "${files[@]}" < "$diff_file")"
  else
    output="$(RESOLVE_INCLUDE_METADATA=1 AGENTS_ROOT="$ROOT" bash "$SCRIPT" go-implementation "${files[@]}")"
  fi

  if ! diff -u "$expected_file" <(printf '%s\n' "$output"); then
    echo "FAIL: scenario $scenario" >&2
    return 1
  fi
}

main() {
  local scenarios=(
    usecase-write
    repository
    handler
    module-wiring
    fallback-cross-cutting
  )

  for scenario in "${scenarios[@]}"; do
    run_scenario "$scenario"
  done

  echo "OK: $((${#scenarios[@]})) scenarios"
}

main "$@"
