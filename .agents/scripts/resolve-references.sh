#!/usr/bin/env bash
# resolve-references.sh
# Resolve referencias surgical a partir de uma lista de arquivos tocados pela tarefa
# e, opcionalmente, sinais de diff recebidos em stdin.
#
# Compatibilidade:
# - schema 1: references[] com always/when_to_load
# - schema 2: references[] + task_types[] com required_refs/optional_refs/forbidden_refs
#
# Uso:
#   bash .agents/scripts/resolve-references.sh <skill-name> <files...>
#   echo "<diff text>" | bash .agents/scripts/resolve-references.sh <skill-name> <files...>
#
# Variaveis:
#   AGENTS_ROOT=$(pwd)
#   RESOLVE_VERBOSE=1
#   RESOLVE_INCLUDE_METADATA=1
#
# Saida padrao (stdout):
#   <skill>/<ref-id>\t<absolute-path-to-md>\t<trigger>
#
# Saida com metadata (RESOLVE_INCLUDE_METADATA=1):
#   META\ttask_type\t<id>
#   META\tvalidation_profile\t<profile>
#   META\trequired_refs\t<csv>
#   META\toptional_refs\t<csv>
#   META\tforbidden_refs\t<csv>
#   <skill>/<ref-id>\t<absolute-path-to-md>\t<trigger>

set -euo pipefail

skill_name="${1:-}"
[[ -n "$skill_name" ]] || { echo "uso: resolve-references.sh <skill> <files...>" >&2; exit 64; }
shift

AGENTS_ROOT="${AGENTS_ROOT:-$(pwd)}"
skill_dir="$AGENTS_ROOT/.agents/skills/$skill_name"
index="$skill_dir/references/INDEX.yaml"

if [[ ! -f "$index" ]]; then
  echo "INDEX.yaml ausente em $skill_dir/references/ — skill nao instalada ou nao indexada" >&2
  exit 2
fi

diff_text=""
if [[ ! -t 0 ]]; then
  diff_text="$(cat)"
fi

files=("$@")

detect_project_root_for() {
  local f="$1"
  local dir
  dir=$(dirname "$f")
  while [[ "$dir" != "/" && "$dir" != "." && -n "$dir" ]]; do
    if [[ -d "$dir/.agents" || -d "$dir/.git" ]]; then
      printf '%s' "$dir"
      return 0
    fi
    dir=$(dirname "$dir")
  done
  return 1
}

rel_files=()
for f in "${files[@]}"; do
  if [[ "$AGENTS_ROOT" != "/" && "$f" == "$AGENTS_ROOT/"* ]]; then
    rel_files+=("${f#$AGENTS_ROOT/}")
    continue
  fi
  if proj_root=$(detect_project_root_for "$f"); then
    rel_files+=("${f#$proj_root/}")
    continue
  fi
  rel_files+=("$f")
done
files=("${rel_files[@]}")

schema="$(awk '/^schema:/ { print $2; exit }' "$index")"
[[ -n "$schema" ]] || schema="1"

match_glob() {
  local pattern="$1" file="$2"
  local re
  re=$(printf '%s' "$pattern" | sed \
    -e 's/\./\\./g' \
    -e 's|\*\*|__GLOBSTAR__|g' \
    -e 's|\*|[^/]*|g' \
    -e 's|__GLOBSTAR__|.*|g')
  [[ "$file" =~ ^.*${re}$ ]]
}

match_signal() {
  local needle="$1"
  [[ -n "$needle" && -n "$diff_text" ]] || return 1
  grep -F -q -- "$needle" <<<"$diff_text" 2>/dev/null
}

emit_ref() {
  local id="$1" trigger="$2"
  local file
  file="$(ref_file_for "$id")"
  if [[ -z "$file" ]]; then
    echo "WARN: referencia '$id' nao encontrada em $index" >&2
    return 0
  fi
  local abs="$skill_dir/references/$file"
  [[ -f "$abs" ]] || abs="(missing)"
  printf '%s/%s\t%s\t%s\n' "$skill_name" "$id" "$abs" "$trigger"
}

ref_file_for() {
  local wanted="$1"
  awk -v target="$wanted" '
    $0 ~ /^references:/ { in_refs=1; next }
    in_refs && $0 ~ /^task_types:/ { exit }
    in_refs && $0 ~ /^  - id:/ {
      id=$3
      next
    }
    in_refs && id == target && $0 ~ /^    file:/ {
      print $2
      exit
    }
  ' "$index"
}

parse_schema1_refs() {
  awk -v US=$'\x1f' '
    function parse_inline_array(raw,    out, val) {
      sub(/^[^[]*\[/, "", raw)
      sub(/\][[:space:]]*$/, "", raw)
      out = ""
      while (length(raw) > 0) {
        if (match(raw, /"[^"]*"/)) {
          val = substr(raw, RSTART+1, RLENGTH-2)
          raw = substr(raw, RSTART+RLENGTH)
          sub(/^[[:space:]]*,[[:space:]]*/, "", raw)
          out = (out == "" ? val : out US val)
        } else if (match(raw, /[^,]+/)) {
          val = substr(raw, RSTART, RLENGTH)
          raw = substr(raw, RSTART+RLENGTH)
          sub(/^[[:space:]]*,[[:space:]]*/, "", raw)
          gsub(/^[[:space:]]+|[[:space:]]+$/, "", val)
          if (val != "") out = (out == "" ? val : out US val)
        } else break
      }
      return out
    }
    BEGIN { in_refs = 0; in_when = 0; cur_id = ""; cur_file = ""; cur_always = "0"; cur_fp = ""; cur_ds = "" }
    /^references:/ { in_refs = 1; next }
    !in_refs { next }
    /^  - id:/ {
      if (cur_id != "") printf "REF|%s|%s|%s|%s|%s\n", cur_id, cur_file, cur_always, cur_fp, cur_ds
      cur_id = $3; cur_file = ""; cur_always = "0"; cur_fp = ""; cur_ds = ""; in_when = 0; next
    }
    /^    file:/ { cur_file = $2; next }
    /^    always:[[:space:]]*true/ { cur_always = "1"; next }
    /^    when_to_load:/ { in_when = 1; next }
    in_when && /^      file_patterns:/ {
      if (index($0, "[") > 0) cur_fp = parse_inline_array($0)
      next
    }
    in_when && /^      diff_signals:/ {
      if (index($0, "[") > 0) cur_ds = parse_inline_array($0)
      next
    }
    END {
      if (cur_id != "") printf "REF|%s|%s|%s|%s|%s\n", cur_id, cur_file, cur_always, cur_fp, cur_ds
    }
  ' "$index"
}

parse_schema2() {
  awk -v US=$'\x1f' '
    function parse_inline_array(raw,    out, val) {
      sub(/^[^[]*\[/, "", raw)
      sub(/\][[:space:]]*$/, "", raw)
      out = ""
      while (length(raw) > 0) {
        if (match(raw, /"[^"]*"/)) {
          val = substr(raw, RSTART+1, RLENGTH-2)
          raw = substr(raw, RSTART+RLENGTH)
          sub(/^[[:space:]]*,[[:space:]]*/, "", raw)
          out = (out == "" ? val : out US val)
        } else if (match(raw, /[^,]+/)) {
          val = substr(raw, RSTART, RLENGTH)
          raw = substr(raw, RSTART+RLENGTH)
          sub(/^[[:space:]]*,[[:space:]]*/, "", raw)
          gsub(/^[[:space:]]+|[[:space:]]+$/, "", val)
          if (val != "") out = (out == "" ? val : out US val)
        } else break
      }
      return out
    }
    BEGIN {
      section = ""
      cur_ref_id = ""; cur_ref_file = ""; cur_ref_topics = ""
      cur_task_id = ""; cur_task_desc = ""; cur_task_fp = ""; cur_task_ds = ""
          cur_req = ""; cur_opt = ""; cur_forbid = ""; cur_profile = ""; cur_priority = "0"
    }
    /^references:/ { section = "references"; next }
    /^task_types:/ {
      if (section == "references" && cur_ref_id != "") printf "REFMAP|%s|%s\n", cur_ref_id, cur_ref_file
      section = "task_types"
      cur_ref_id = ""; cur_ref_file = ""
      next
    }
    section == "references" && /^  - id:/ {
      if (cur_ref_id != "") printf "REFMAP|%s|%s\n", cur_ref_id, cur_ref_file
      cur_ref_id = $3; cur_ref_file = ""; next
    }
    section == "references" && /^    file:/ { cur_ref_file = $2; next }
    section == "task_types" && /^  - id:/ {
      if (cur_task_id != "") printf "TASK|%s|%s|%s|%s|%s|%s|%s|%s\n", cur_task_id, cur_task_fp, cur_task_ds, cur_req, cur_opt, cur_forbid, cur_profile, cur_priority
      cur_task_id = $3; cur_task_fp = ""; cur_task_ds = ""; cur_req = ""; cur_opt = ""; cur_forbid = ""; cur_profile = ""; cur_priority = "0"; next
    }
    section == "task_types" && /^    priority:/ { cur_priority = $2; next }
    section == "task_types" && /^      file_patterns:/ {
      if (index($0, "[") > 0) cur_task_fp = parse_inline_array($0)
      next
    }
    section == "task_types" && /^      diff_signals:/ {
      if (index($0, "[") > 0) cur_task_ds = parse_inline_array($0)
      next
    }
    section == "task_types" && /^    required_refs:/ {
      if (index($0, "[") > 0) cur_req = parse_inline_array($0)
      next
    }
    section == "task_types" && /^    optional_refs:/ {
      if (index($0, "[") > 0) cur_opt = parse_inline_array($0)
      next
    }
    section == "task_types" && /^    forbidden_refs:/ {
      if (index($0, "[") > 0) cur_forbid = parse_inline_array($0)
      next
    }
    section == "task_types" && /^    validation_profile:/ {
      cur_profile = $2
      next
    }
    END {
      if (section == "references" && cur_ref_id != "") printf "REFMAP|%s|%s\n", cur_ref_id, cur_ref_file
      if (section == "task_types" && cur_task_id != "") printf "TASK|%s|%s|%s|%s|%s|%s|%s|%s\n", cur_task_id, cur_task_fp, cur_task_ds, cur_req, cur_opt, cur_forbid, cur_profile, cur_priority
    }
  ' "$index"
}

resolve_schema1() {
  local included_ids=""
  while IFS='|' read -r kind id file always fp ds; do
    [[ "$kind" == "REF" ]] || continue
    local trigger=""
    if [[ "$always" == "1" ]]; then
      trigger="always"
    else
      if [[ -n "$fp" ]]; then
        IFS=$'\x1f' read -ra patterns <<<"$fp"
        for pat in "${patterns[@]}"; do
          [[ -z "$pat" ]] && continue
          for f in "${files[@]}"; do
            if match_glob "$pat" "$f"; then
              trigger="file_pattern:$pat"
              break 2
            fi
          done
        done
      fi
      if [[ -z "$trigger" && -n "$ds" ]]; then
        IFS=$'\x1f' read -ra signals <<<"$ds"
        for sig in "${signals[@]}"; do
          [[ -z "$sig" ]] && continue
          if match_signal "$sig"; then
            trigger="diff_signal:$sig"
            break
          fi
        done
      fi
    fi
    if [[ -n "$trigger" ]]; then
      case " $included_ids " in
        *" $id "*) ;;
        *)
          emit_ref "$id" "$trigger"
          included_ids="$included_ids $id"
          ;;
      esac
    fi
  done < <(parse_schema1_refs)
}

resolve_schema2() {
  local fallback_task_type
  fallback_task_type="$(awk '/fallback_task_type:/ { print $2; exit }' "$index")"
  [[ -n "$fallback_task_type" ]] || fallback_task_type="cross-cutting"

  local parse_tmp
  parse_tmp="$(mktemp)"
  trap 'rm -f "$parse_tmp"' RETURN
  parse_schema2 > "$parse_tmp"

  task_field() {
    local task="$1" field="$2"
    awk -F'|' -v target="$task" -v idx="$field" '$1 == "TASK" && $2 == target { print $idx; exit }' "$parse_tmp"
  }

  local selected_task=""
  local selected_trigger=""
  local best_score=-1

  while IFS='|' read -r kind task c3 c4 c5 c6 c7 c8 c9; do
    [[ "$kind" == "TASK" ]] || continue
    local matched=0
    local score=0
    local trigger=""

    local fp="$c3"
    if [[ -n "$fp" ]]; then
      IFS=$'\x1f' read -ra patterns <<<"$fp"
      for pat in "${patterns[@]}"; do
        [[ -z "$pat" ]] && continue
        for f in "${files[@]}"; do
          if match_glob "$pat" "$f"; then
            matched=1
            score=$((score + 10))
            trigger="file_pattern:$pat"
            break 2
          fi
        done
      done
    fi

    local ds="$c4"
    if [[ -n "$ds" ]]; then
      IFS=$'\x1f' read -ra signals <<<"$ds"
      for sig in "${signals[@]}"; do
        [[ -z "$sig" ]] && continue
        if match_signal "$sig"; then
          matched=1
          score=$((score + 3))
          if [[ -z "$trigger" ]]; then
            trigger="diff_signal:$sig"
          fi
          break
        fi
      done
    fi

    local priority="${c9:-0}"
    score=$((score + priority))

    if [[ "$matched" == "1" && "$score" -gt "$best_score" ]]; then
      best_score="$score"
      selected_task="$task"
      selected_trigger="$trigger"
    fi
  done < "$parse_tmp"

  if [[ -z "$selected_task" ]]; then
    selected_task="$fallback_task_type"
    selected_trigger="fallback"
  fi

  local req opt forbid profile
  req="$(task_field "$selected_task" 5)"
  opt="$(task_field "$selected_task" 6)"
  forbid="$(task_field "$selected_task" 7)"
  profile="$(task_field "$selected_task" 8)"

  if [[ "${RESOLVE_INCLUDE_METADATA:-0}" == "1" ]]; then
    printf 'META\ttask_type\t%s\n' "$selected_task"
    printf 'META\tvalidation_profile\t%s\n' "$profile"
    printf 'META\trequired_refs\t%s\n' "${req//$'\x1f'/,}"
    printf 'META\toptional_refs\t%s\n' "${opt//$'\x1f'/,}"
    printf 'META\tforbidden_refs\t%s\n' "${forbid//$'\x1f'/,}"
  fi

  local included_ids=""
  local base_refs
  base_refs="$(awk '
    /^defaults:/ { in_defaults=1; next }
    in_defaults && /^  base_required_refs:/ { in_base=1; next }
    in_defaults && in_base && /^    - / { print $2; next }
    in_defaults && in_base && !/^    - / { exit }
  ' "$index")"

  while IFS= read -r id; do
    [[ -n "$id" ]] || continue
    case " $included_ids " in
      *" $id "*) ;;
      *)
        emit_ref "$id" "base_required_ref"
        included_ids="$included_ids $id"
        ;;
    esac
  done <<<"$base_refs"

  IFS=$'\x1f' read -ra req_refs <<<"$req"
  for id in "${req_refs[@]}"; do
    [[ -n "$id" ]] || continue
    case " $included_ids " in
      *" $id "*) ;;
      *)
        emit_ref "$id" "task_type:${selected_task}:${selected_trigger}:required"
        included_ids="$included_ids $id"
        ;;
    esac
  done

  IFS=$'\x1f' read -ra opt_refs <<<"$opt"
  for id in "${opt_refs[@]}"; do
    [[ -n "$id" ]] || continue
    case " $included_ids " in
      *" $id "*) ;;
      *)
        emit_ref "$id" "task_type:${selected_task}:optional"
        included_ids="$included_ids $id"
        ;;
    esac
  done

  rm -f "$parse_tmp"
  trap - RETURN
}

case "$schema" in
  2) resolve_schema2 ;;
  *) resolve_schema1 ;;
esac

exit 0
