#!/usr/bin/env bash

set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
vars_file="${repo_root}/taskfiles/vars.yml"
tools_bin_dir="${TOOLS_BIN_DIR:-${repo_root}/.tools/bin}"
binary="${tools_bin_dir}/golangci-lint"

if [[ ! -f "${vars_file}" ]]; then
  echo "taskfiles/vars.yml ausente; nao foi possivel resolver GOLANGCI_LINT_VERSION" >&2
  exit 1
fi

version="$(awk '/GOLANGCI_LINT_VERSION:/ {print $2; exit}' "${vars_file}" | tr -d "\"'")"

if [[ -z "${version}" ]]; then
  echo "GOLANGCI_LINT_VERSION ausente em taskfiles/vars.yml" >&2
  exit 1
fi

expected_version="${version#v}"

current_version=""
if [[ -x "${binary}" ]]; then
  current_version="$("${binary}" version 2>/dev/null | awk '{for (i = 1; i <= NF; i++) if ($i == "version") {print $(i + 1); exit}}')"
fi

if [[ "${current_version#v}" != "${expected_version}" ]]; then
  mkdir -p "${tools_bin_dir}"
  echo "==> Instalando golangci-lint ${version} em ${tools_bin_dir}"
  GOBIN="${tools_bin_dir}" go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@"${version}"
fi

installed_version="$("${binary}" version 2>/dev/null | awk '{for (i = 1; i <= NF; i++) if ($i == "version") {print $(i + 1); exit}}')"
if [[ "${installed_version#v}" != "${expected_version}" ]]; then
  echo "golangci-lint esperado ${version}, encontrado ${installed_version:-<ausente>}" >&2
  exit 1
fi
