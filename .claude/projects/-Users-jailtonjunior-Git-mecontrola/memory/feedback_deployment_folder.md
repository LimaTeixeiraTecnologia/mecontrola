---
name: feedback-deployment-folder
description: All infra/docker/dashboard/deploy files must live under deployment/ organized by category — non-negotiable
metadata:
  type: feedback
---

All infrastructure deployment artifacts must be organized under a `deployment/` root folder:
- `deployment/docker/` — Dockerfile, .dockerignore
- `deployment/fly/` — fly.toml
- `deployment/grafana/` — Grafana dashboards (JSON files)
- `deployment/runbooks/` — operational runbooks

Nothing deployment-related goes in the repo root or in `docs/runbooks/`. This is non-negotiable per user instruction.

**Why:** User said "tudo que for de infra docker, dashboard, deploy, crie uma pasta chamada deployment e mova para lá de acordo com a categoria de configuração, isso é inegociável".
**How to apply:** Any task that creates Docker, Fly.io, Grafana, or runbook files must place them under deployment/ in the appropriate subfolder. Update all references in Taskfile, CI workflows, and docs when moving files.
