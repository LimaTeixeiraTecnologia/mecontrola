#!/usr/bin/env bash
set -euo pipefail

# export-env.sh — Exporta variaveis de um arquivo .env de forma segura,
# ignorando linhas invalidas, comentarios e valores que quebrariam o shell.

ENV_FILE="${1:-.env}"

[[ -f "$ENV_FILE" ]] || { echo "ERRO: $ENV_FILE nao encontrado" >&2; exit 1; }

while IFS= read -r line || [[ -n "$line" ]]; do
  line="${line%%#*}"
  [[ -z "$line" ]] && continue
  [[ "$line" =~ ^[[:space:]]*([A-Za-z_][A-Za-z0-9_]*)[[:space:]]*=[[:space:]]*(.*)$ ]] || continue
  key="${BASH_REMATCH[1]}"
  value="${BASH_REMATCH[2]}"
  value="${value%\"}"
  value="${value#\"}"
  value="${value%\'}"
  value="${value#\'}"
  printf '%s=%q\n' "$key" "$value"
done < "$ENV_FILE"
