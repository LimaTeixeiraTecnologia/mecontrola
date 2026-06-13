#!/usr/bin/env bash
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
cd "$repo_root"

python3 - "$repo_root" << 'PYEOF'
import sys
import os
import re
import glob

repo_root = sys.argv[1]
allowlist_file = os.path.join(repo_root, "internal/platform/outbox/system_event_allowlist.go")

def load_allowlist(path):
    if not os.path.exists(path):
        return set()
    with open(path, "r") as f:
        content = f.read()
    return set(re.findall(r'"([^"]+)"\s*:\s*\{\}', content))

ALLOWLIST = load_allowlist(allowlist_file)

STRUCT_PATTERN = re.compile(r'outbox\.(EventInput|Event)\{')

def find_struct_literal(content, start_pos):
    brace_pos = content.index('{', start_pos)
    depth = 0
    i = brace_pos
    while i < len(content):
        c = content[i]
        if c == '{':
            depth += 1
        elif c == '}':
            depth -= 1
            if depth == 0:
                return brace_pos, i
        i += 1
    return brace_pos, len(content) - 1

def line_number(content, pos):
    return content[:pos].count('\n') + 1

def find_go_files():
    files = []
    for root, dirs, filenames in os.walk(os.path.join(repo_root, "internal")):
        dirs[:] = [d for d in dirs if d != "mocks"]
        for fname in filenames:
            if fname.endswith(".go") and not fname.endswith("_test.go"):
                files.append(os.path.join(root, fname))
    return sorted(files)

violations = []

for filepath in find_go_files():
    with open(filepath, "r", errors="replace") as f:
        content = f.read()

    rel_path = os.path.relpath(filepath, repo_root)

    for m in STRUCT_PATTERN.finditer(content):
        try:
            brace_start, brace_end = find_struct_literal(content, m.start())
        except (ValueError, IndexError):
            continue

        struct_body = content[brace_start:brace_end + 1]

        if struct_body.strip() == "{}":
            continue

        if not struct_body.strip().startswith("{"):
            continue

        has_aggregate_user_id = bool(re.search(r'\bAggregateUserID\s*:', struct_body))

        if has_aggregate_user_id:
            continue

        type_match = re.search(r'\bType\s*:\s*"([^"]+)"', struct_body)
        if type_match:
            type_value = type_match.group(1)
        else:
            type_match2 = re.search(r'\bType\s*:\s*(\S+)', struct_body)
            type_value = type_match2.group(1).strip('",') if type_match2 else ""

        if type_value in ALLOWLIST:
            continue

        lnum = line_number(content, m.start())
        violations.append(f"  {rel_path}:{lnum}: {m.group(0)}... sem AggregateUserID: (Type: '{type_value}' nao esta na allowlist)")

if violations:
    print("FAIL lint:outbox-user-id — outbox.EventInput/Event sem AggregateUserID em codigo de producao (ADR-004 violado):", file=sys.stderr)
    for v in violations:
        print(v, file=sys.stderr)
    print("", file=sys.stderr)
    print(f"Politica: todo construtor de outbox.EventInput/Event DEVE popular AggregateUserID,", file=sys.stderr)
    print(f"exceto event types em internal/platform/outbox/system_event_allowlist.go.", file=sys.stderr)
    print("Ver .specs/prd-outbox-aggregate-user-id/prd.md RF-16 e ADR-004.", file=sys.stderr)
    sys.exit(1)

allowlist_str = ", ".join(sorted(ALLOWLIST)) if ALLOWLIST else "<vazio>"
print(f"PASS lint:outbox-user-id — todos os construtores de outbox.EventInput/Event populam AggregateUserID ({len(ALLOWLIST)} tipo(s) na allowlist: {allowlist_str})")
PYEOF
