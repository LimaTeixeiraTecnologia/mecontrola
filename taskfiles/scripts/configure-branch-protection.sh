#!/usr/bin/env bash
# configure-branch-protection.sh
# Configura branch protection na main via GitHub API.
# ADR-013: require CODEOWNER review + status checks + linear history + signed commits.
#
# Uso: GITHUB_TOKEN=<pat> bash configure-branch-protection.sh
# Requer escopo: repo (ou admin:repo para require_signed_commits)

set -euo pipefail

OWNER="${GITHUB_OWNER:-LimaTeixeiraTecnologia}"
REPO="${GITHUB_REPO:-mecontrola}"
BRANCH="${GITHUB_BRANCH:-main}"
TOKEN="${GITHUB_TOKEN:?GITHUB_TOKEN é obrigatório}"

API="https://api.github.com/repos/${OWNER}/${REPO}/branches/${BRANCH}/protection"

echo "==> Configurando branch protection em ${OWNER}/${REPO}:${BRANCH}"

curl -s -X PUT "${API}" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Accept: application/vnd.github+json" \
  -H "X-GitHub-Api-Version: 2022-11-28" \
  -d '{
    "required_status_checks": {
      "strict": true,
      "contexts": [
        "Lint",
        "Unit Tests",
        "Integration Tests",
        "Security",
        "Governance",
        "Coverage Comment"
      ]
    },
    "enforce_admins": true,
    "required_pull_request_reviews": {
      "require_code_owner_reviews": true,
      "required_approving_review_count": 1,
      "dismiss_stale_reviews": true
    },
    "restrictions": null,
    "required_linear_history": true,
    "allow_force_pushes": false,
    "allow_deletions": false,
    "block_creations": false,
    "required_conversation_resolution": true
  }' | jq .

echo "==> Ativando require_signed_commits (endpoint separado)"

curl -s -X POST \
  "https://api.github.com/repos/${OWNER}/${REPO}/branches/${BRANCH}/protection/required_signatures" \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Accept: application/vnd.github+json" \
  -H "X-GitHub-Api-Version: 2022-11-28" | jq .

echo "==> Branch protection configurada com sucesso"
