#!/usr/bin/env sh
# Valida Conventional Commits em dois modos:
#   check-conventional-commit.sh <arquivo-commit-msg>
#   check-conventional-commit.sh <base-ref> <head-ref>
set -e

PATTERN='^(feat|fix|docs|style|refactor|test|chore|perf|ci|build|revert)(\(.+\))?(!)?: .{1,}'

validate_subject() {
  subject="$1"
  if echo "$subject" | grep -Eq "$PATTERN"; then
    return 0
  fi

  echo "ERRO: Mensagem de commit nao segue Conventional Commits."
  echo "Formato esperado: <tipo>(<escopo>): <descricao>"
  echo "Tipos validos: feat, fix, docs, style, refactor, test, chore, perf, ci, build, revert"
  echo "Mensagem recebida: $subject"
  exit 1
}

if [ "$#" -eq 1 ]; then
  validate_subject "$(head -1 "$1")"
  exit 0
fi

if [ "$#" -eq 2 ]; then
  BASE_REF="$1"
  HEAD_REF="$2"
  if echo "$BASE_REF" | grep -Eq '^0+$'; then
    BASE_REF="$(git rev-list --max-parents=0 "$HEAD_REF")"
  fi

  TMP_FILE="$(mktemp)"
  trap 'rm -f "$TMP_FILE"' EXIT
  git log --format=%s "$BASE_REF..$HEAD_REF" > "$TMP_FILE"

  while IFS= read -r subject; do
    [ -z "$subject" ] && continue
    validate_subject "$subject"
  done < "$TMP_FILE"
  exit 0
fi

echo "Uso: $0 <arquivo-commit-msg> OU $0 <base-ref> <head-ref>"
exit 1
