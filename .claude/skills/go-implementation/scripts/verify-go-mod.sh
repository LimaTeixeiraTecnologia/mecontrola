#!/usr/bin/env bash
set -euo pipefail

start_dir="${1:-$PWD}"
dir="$start_dir"

while [[ "$dir" != "/" ]]; do
  if [[ -f "$dir/go.mod" ]]; then
    go_version="$(awk '/^go[[:space:]]+/ { print $2; exit }' "$dir/go.mod")"
    echo "OK: go.mod encontrado em $dir/go.mod"
    if [[ -n "${go_version:-}" ]]; then
      echo "OK: versao Go declarada: $go_version"
    else
      echo "WARN: versao Go nao encontrada em $dir/go.mod"
    fi
    exit 0
  fi
  dir="$(dirname "$dir")"
done

echo "ERRO: go.mod ausente em $start_dir e em seus diretorios ancestrais. Verifique se a tarefa realmente envolve um contexto Go." >&2
exit 1
