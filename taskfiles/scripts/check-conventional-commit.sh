#!/usr/bin/env sh
# Valida que a primeira linha da mensagem de commit segue o formato Conventional Commits.
# Uso: check-conventional-commit.sh <arquivo-commit-msg>
set -e

COMMIT_MSG_FILE="$1"
if [ -z "$COMMIT_MSG_FILE" ]; then
  echo "Uso: $0 <arquivo-commit-msg>"
  exit 1
fi

FIRST_LINE="$(head -1 "$COMMIT_MSG_FILE")"

if echo "$FIRST_LINE" | grep -Eq '^(feat|fix|docs|style|refactor|test|chore|perf|ci|build|revert)(\(.+\))?(!)?: .{1,}'; then
  exit 0
else
  echo "ERRO: Mensagem de commit nao segue Conventional Commits."
  echo "Formato esperado: <tipo>(<escopo>): <descricao>"
  echo "Tipos validos: feat, fix, docs, style, refactor, test, chore, perf, ci, build, revert"
  echo "Mensagem recebida: $FIRST_LINE"
  exit 1
fi
