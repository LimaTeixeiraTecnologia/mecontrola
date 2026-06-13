#!/usr/bin/env bash
# Gate de defesa em profundidade: toda query UPDATE/DELETE em repositorios
# postgres de modulos per-user DEVE conter "user_id" na clausula WHERE para
# prevenir vazamento cross-tenant em caso de bug no use case caller.
#
# Modulos avaliados: budgets, card, transactions. Categories esta excluido
# por design (dicionario global, sem isolamento per-user).
#
# Falha o build com lista de queries em violacao.
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
cd "$repo_root"

targets=(
  "internal/budgets/infrastructure/repositories/postgres"
  "internal/card/infrastructure/repositories/postgres"
  "internal/transactions/infrastructure/repositories/postgres"
)
# Modulos deliberadamente excluidos (decisao arquitetural explicita):
#   - internal/categories: dicionario compartilhado por design, ver
#     .specs/prd-categories-crud/adr-007-dicionario-compartilhado-sem-user-id.md

for dir in "${targets[@]}"; do
  [[ -d "$dir" ]] || continue
  while IFS= read -r file; do
    awk -v file="$file" '
      BEGIN { in_q = 0; q = ""; start_line = 0; func_name = "" }
      /^func[[:space:]]/ {
        func_name = $0
      }
      /const[[:space:]]+query[[:space:]]*=[[:space:]]*`/ { in_q = 1; q = ""; start_line = NR; next }
      in_q && /`/ {
        in_q = 0
        upper_q = toupper(q)
        is_mutation = (upper_q ~ /^[[:space:]]*UPDATE/ || upper_q ~ /^[[:space:]]*DELETE[[:space:]]+FROM/)
        has_user_id = (upper_q ~ /USER_ID/)
        is_retention_job = (func_name ~ /Purge|Cleanup|Reap|HousekeepingPurge|RetentionPurge/)
        is_retention_query = (upper_q ~ /NOW\(\)[[:space:]]*-/ && upper_q ~ /INTERVAL/)
        if (is_mutation && !has_user_id && !is_retention_job && !is_retention_query) {
          print file ":" start_line ": query UPDATE/DELETE sem user_id"
        }
        q = ""
        next
      }
      in_q { q = q " " $0 }
    ' "$file"
  done < <(find "$dir" -maxdepth 1 -name "*.go" -not -name "*_test.go")
done > /tmp/repo-user-id-violations.txt

if [[ -s /tmp/repo-user-id-violations.txt ]]; then
  echo "FAIL: queries UPDATE/DELETE sem 'user_id' na clausula WHERE em repositorios per-user:" >&2
  cat /tmp/repo-user-id-violations.txt >&2
  rm -f /tmp/repo-user-id-violations.txt
  exit 1
fi

rm -f /tmp/repo-user-id-violations.txt
echo "ok: defesa em profundidade per-user OK (UPDATE/DELETE com user_id em budgets/card/transactions)"
