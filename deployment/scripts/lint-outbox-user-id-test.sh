#!/usr/bin/env bash
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
script="${repo_root}/deployment/scripts/lint-outbox-user-id.sh"

tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT

allowlist_file="${tmpdir}/allowlist.go"
cat > "$allowlist_file" <<'GOEOF'
package outbox

var systemEventAllowlist = map[string]struct{}{
	"auth.failed": {},
}
GOEOF

scan_dir="${tmpdir}/scan"
mkdir -p "$scan_dir"

pass=0
fail=0

assert_exit() {
	local name="$1"
	local expected="$2"
	shift 2
	set +e
	"$@" >/dev/null 2>&1
	local actual=$?
	set -e
	if [ "$actual" -eq "$expected" ]; then
		echo "PASS: $name (exit=$actual)"
		pass=$((pass + 1))
	else
		echo "FAIL: $name (expected exit=$expected, got=$actual)"
		fail=$((fail + 1))
	fi
}

run_lint() {
	OUTBOX_ALLOWLIST_FILE="$allowlist_file" bash "$script" "$scan_dir"
}

rm -rf "$scan_dir" && mkdir -p "$scan_dir"
cat > "${scan_dir}/case_ok.go" <<'GOEOF'
package x

import "outbox"

func emit() {
	_ = outbox.Event{Type: "x.y", AggregateUserID: "u-1", AggregateType: "a", AggregateID: "id"}
}
GOEOF
assert_exit "valor não-vazio passa" 0 run_lint

rm -rf "$scan_dir" && mkdir -p "$scan_dir"
cat > "${scan_dir}/case_missing.go" <<'GOEOF'
package x

import "outbox"

func emit() {
	_ = outbox.Event{Type: "x.y", AggregateType: "a", AggregateID: "id"}
}
GOEOF
assert_exit "campo ausente falha" 1 run_lint

rm -rf "$scan_dir" && mkdir -p "$scan_dir"
cat > "${scan_dir}/case_empty_literal.go" <<'GOEOF'
package x

import "outbox"

func emit() {
	_ = outbox.Event{Type: "x.y", AggregateUserID: "", AggregateType: "a", AggregateID: "id"}
}
GOEOF
assert_exit "literal vazio falha" 1 run_lint

rm -rf "$scan_dir" && mkdir -p "$scan_dir"
cat > "${scan_dir}/case_allowlist_empty.go" <<'GOEOF'
package x

import "outbox"

func emit() {
	_ = outbox.Event{Type: "auth.failed", AggregateUserID: "", AggregateType: "a", AggregateID: "id"}
}
GOEOF
assert_exit "literal vazio em tipo da allowlist passa" 0 run_lint

rm -rf "$scan_dir" && mkdir -p "$scan_dir"
cat > "${scan_dir}/case_var.go" <<'GOEOF'
package x

import "outbox"

func emit(userID string) {
	_ = outbox.Event{Type: "x.y", AggregateUserID: userID, AggregateType: "a", AggregateID: "id"}
}
GOEOF
assert_exit "variável (não-literal) passa" 0 run_lint

rm -rf "$scan_dir" && mkdir -p "$scan_dir"
cat > "${scan_dir}/case_expr.go" <<'GOEOF'
package x

import "outbox"

type u struct{ id string }

func (x u) String() string { return x.id }

func emit(usr u) {
	_ = outbox.Event{Type: "x.y", AggregateUserID: usr.String(), AggregateType: "a", AggregateID: "id"}
}
GOEOF
assert_exit "expressão (não-literal) passa" 0 run_lint

rm -rf "$scan_dir" && mkdir -p "$scan_dir"
cat > "${scan_dir}/case_test_excluded.go" <<'GOEOF'
package x

import "outbox"

func emit() {
	_ = outbox.Event{}
}
GOEOF
assert_exit "outbox.Event{} vazio é ignorado (retorno de erro)" 0 run_lint

echo ""
echo "==> Resultado: ${pass} pass, ${fail} fail"
if [ "$fail" -gt 0 ]; then
	exit 1
fi
