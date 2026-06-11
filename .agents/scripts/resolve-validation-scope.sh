#!/usr/bin/env bash
# resolve-validation-scope.sh
# Calcula escopo de validacao Go a partir dos arquivos tocados.
#
# Usa resolve-references.sh para obter o validation_profile e aplica uma politica
# simples e deterministica para gerar escopos de build/vet/test/lint e flags de race.
#
# Uso:
#   bash .agents/scripts/resolve-validation-scope.sh <files...>
#   echo "<diff>" | bash .agents/scripts/resolve-validation-scope.sh <files...>
#
# Saida:
#   METADATA\tkey\tvalue
#   SCOPE\tkey\tvalue
#
# Chaves:
#   task_type, validation_profile, race_required
#   scope_go, scope_build, scope_vet, scope_test, scope_lint

set -euo pipefail

AGENTS_ROOT="${AGENTS_ROOT:-$(pwd)}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
RESOLVE_SCRIPT="$SCRIPT_DIR/resolve-references.sh"

files=("$@")
[[ ${#files[@]} -gt 0 ]] || { echo "uso: resolve-validation-scope.sh <files...>" >&2; exit 64; }

diff_text=""
if [[ ! -t 0 ]]; then
  diff_text="$(cat)"
fi

resolver_output=""
if [[ -n "$diff_text" ]]; then
  resolver_output="$(RESOLVE_INCLUDE_METADATA=1 AGENTS_ROOT="$AGENTS_ROOT" bash "$RESOLVE_SCRIPT" go-implementation "${files[@]}" <<<"$diff_text")"
else
  resolver_output="$(RESOLVE_INCLUDE_METADATA=1 AGENTS_ROOT="$AGENTS_ROOT" bash "$RESOLVE_SCRIPT" go-implementation "${files[@]}")"
fi

task_type="$(printf '%s\n' "$resolver_output" | awk -F'\t' '$1 == "META" && $2 == "task_type" { print $3; exit }')"
validation_profile="$(printf '%s\n' "$resolver_output" | awk -F'\t' '$1 == "META" && $2 == "validation_profile" { print $3; exit }')"

[[ -n "$validation_profile" ]] || validation_profile="global"

unique_append() {
  local current="$1"
  local value="$2"
  if [[ -z "$current" ]]; then
    printf '%s' "$value"
    return 0
  fi
  case " $current " in
    *" $value "*) printf '%s' "$current" ;;
    *)
      printf '%s %s' "$current" "$value"
      ;;
  esac
}

dir_of() {
  local path="$1"
  if [[ -d "$path" ]]; then
    printf '%s' "$path"
  else
    dirname "$path"
  fi
}

module_of() {
  local path="$1"
  case "$path" in
    internal/*)
      printf '%s' "$path" | cut -d/ -f1-2
      ;;
    cmd/*)
      printf '%s' "cmd"
      ;;
    *)
      printf '.'
      ;;
  esac
}

package_scope_for() {
  local path="$1"
  local dir
  dir="$(dir_of "$path")"
  if [[ "$dir" == "." ]]; then
    printf './...'
    return 0
  fi
  printf './%s/...' "$dir"
}

build_scope_for() {
  local path="$1"
  case "$path" in
    cmd/*)
      local entry
      entry="$(printf '%s' "$path" | cut -d/ -f1-2)"
      printf './%s' "$entry"
      ;;
    internal/*)
      local mod
      mod="$(module_of "$path")"
      printf './%s/...' "$mod"
      ;;
    *)
      printf './...'
      ;;
  esac
}

scope_go=""
scope_build=""
scope_vet=""
scope_test=""
scope_lint=""
race_required="0"

for file in "${files[@]}"; do
  scope_go="$(unique_append "$scope_go" "$file")"
  pkg_scope="$(package_scope_for "$file")"
  build_scope="$(build_scope_for "$file")"
  scope_test="$(unique_append "$scope_test" "$pkg_scope")"
  scope_build="$(unique_append "$scope_build" "$build_scope")"
  scope_vet="$(unique_append "$scope_vet" "$build_scope")"
  scope_lint="$(unique_append "$scope_lint" "$build_scope")"

  case "$file" in
    *concurrency*|*worker*|*consumer*|*producer*|*job*|*module.go|cmd/*)
      race_required="1"
      ;;
  esac
done

if [[ "$validation_profile" == "boundary" ]]; then
  expanded_build=""
  expanded_vet=""
  expanded_lint=""
  expanded_test=""
  for file in "${files[@]}"; do
    mod="$(module_of "$file")"
    case "$mod" in
      .)
        expanded_build="$(unique_append "$expanded_build" "./...")"
        expanded_vet="$(unique_append "$expanded_vet" "./...")"
        expanded_lint="$(unique_append "$expanded_lint" "./...")"
        expanded_test="$(unique_append "$expanded_test" "./...")"
        ;;
      cmd)
        expanded_build="$(unique_append "$expanded_build" "./cmd/...")"
        expanded_vet="$(unique_append "$expanded_vet" "./cmd/...")"
        expanded_lint="$(unique_append "$expanded_lint" "./cmd/...")"
        expanded_test="$(unique_append "$expanded_test" "./cmd/...")"
        ;;
      *)
        expanded_build="$(unique_append "$expanded_build" "./$mod/...")"
        expanded_vet="$(unique_append "$expanded_vet" "./$mod/...")"
        expanded_lint="$(unique_append "$expanded_lint" "./$mod/...")"
        expanded_test="$(unique_append "$expanded_test" "./$mod/...")"
        ;;
    esac
  done
  scope_build="$expanded_build"
  scope_vet="$expanded_vet"
  scope_lint="$expanded_lint"
  scope_test="$expanded_test"
fi

if [[ "$validation_profile" == "global" ]]; then
  scope_build="./..."
  scope_vet="./..."
  scope_test="./..."
  scope_lint="./..."
  race_required="1"
fi

printf 'METADATA\ttask_type\t%s\n' "$task_type"
printf 'METADATA\tvalidation_profile\t%s\n' "$validation_profile"
printf 'METADATA\trace_required\t%s\n' "$race_required"
printf 'SCOPE\tscope_go\t%s\n' "$scope_go"
printf 'SCOPE\tscope_build\t%s\n' "$scope_build"
printf 'SCOPE\tscope_vet\t%s\n' "$scope_vet"
printf 'SCOPE\tscope_test\t%s\n' "$scope_test"
printf 'SCOPE\tscope_lint\t%s\n' "$scope_lint"
