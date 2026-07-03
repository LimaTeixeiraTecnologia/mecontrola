# Relatório de Execução — execute-all-tasks
# PRD: infra-evolucao-kvm2-10k
# Data: 2026-07-03

## Resultado Final

**Status:** done
**PRD:** `.specs/prd-infra-evolucao-kvm2-10k/prd.md` (spec-version 1, RF-01..22)
**Tarefas:** 8/8 done
**Conformidade:** 100% — 22/22 requisitos funcionais cobertos
**Desvios:** 0
**Lacunas:** 0 (3 gaps de implementação encontrados e corrigidos nesta orquestração)

## Sumário Executivo

O PRD fecha os gaps operacionais que impediam a stack Docker Swarm single-node na VPS Hostinger KVM 2 de ser production-ready para o envelope B (10k/dia). Todos os 22 requisitos funcionais foram implementados. Esta execução de orquestração identificou e corrigiu 3 gaps remanescentes das execuções de tarefa anteriores (RF-11, RF-12 e RF-20 — declarados como `done` nos execution reports mas parcialmente ausentes no código real).

## Tarefas Executadas

| # | Título | RF cobertos | Artefatos principais |
|---|--------|-------------|---------------------|
| 1.0 | Backup pgBackRest agendado e alertado | RF-01..03 | `deployment/scripts/pgbackrest-schedule.sh`, `pgbackrest-backup-metrics.sh`, `deploy-swarm.sh` (guard), `rules.yaml` (grupo backup) |
| 2.0 | Ensaio de restore PITR + VPS com evidência | RF-04..06 | `deployment/runbooks/restore-pitr.md`, `restore-vps.md`, `docs/runs/2026-07-03-evidencia-restore.md` |
| 3.0 | Remover runner + deploy GitHub-hosted SSH | RF-07..10 | `.github/workflows/ci-cd.yml`, `deployment/scripts/remove-runner.sh`, `docker-prune.sh/.service/.timer`, `rules.yaml` (alerta disco) |
| 4.0 | Sampling proporcional + gate anti-storm | RF-11..13 | `deployment/telemetry/grafana/otelcol-config.yaml` (tail_sampling), `scripts/ci/deploy-anti-storm.sh` (6/6), `deployment/runbooks/observabilidade-spof-retention.md` |
| 5.0 | Orçamento de recursos e pool | RF-14..16 | `deployment/compose/compose.swarm.yml` (limits), `deployment/pgbouncer/pgbouncer.ini`, `rules.yaml` (grupo pool), `docs/runs/2026-07-03-orcamento-recursos.md` |
| 6.0 | Harness k6 + prova envelopes A/B | RF-17..18 | `scripts/loadtest/whatsapp-inbound.js`, `transactions-read.js`, `taskfiles/loadtest.yml`, `Taskfile.yml` |
| 7.0 | Deploy main + alerta drift de versão | RF-19..20 | `.github/workflows/version-drift-check.yml`, `rules.yaml` (alerta mc-version-skew), `deployment/runbooks/deploy.md` |
| 8.0 | Endurecimento superfície: Meta + pg-tunnel | RF-21..22 | `deployment/caddy/Caddyfile`, `Caddyfile.ratelimit`, `internal/onboarding/infrastructure/http/server/middleware/rate_limit.go`, `deployment/compose/compose.swarm.yml` (pg-tunnel loopback), `deployment/runbooks/seguranca-webhook-pgtunnel.md` |

## Gaps Detectados e Corrigidos nesta Orquestração

### Gap 1 — RF-11: `tail_sampling` ausente no otelcol (task 4.0)

**Estado antes:** `deployment/telemetry/grafana/otelcol-config.yaml` sem processor `tail_sampling`; pipeline de traces usava apenas `[memory_limiter, batch]`.

**Correção:** Adicionado processor `tail_sampling` com:
- `errors-policy`: `status_code` → inclui 100% dos traces com status ERROR
- `probabilistic-policy`: `probabilistic` → amostra 10% dos demais traces
- Pipeline atualizada: `[memory_limiter, tail_sampling, batch]`

**Arquivo:** `deployment/telemetry/grafana/otelcol-config.yaml`

### Gap 2 — RF-12: `deploy-anti-storm.sh` ainda com 5/5 checks (task 4.0)

**Estado antes:** Check 5/5 exigia `OTEL_TRACE_SAMPLE_RATE="1"` sem validar a presença do `tail_sampling`; gate com label incorreto ("ate sampler parent-based").

**Correção:** Atualizado de 5/5 para 6/6 checks:
- Checks 1–4: inalterados (renumerados)
- Check 5/6: label atualizado para refletir que SDK envia tudo e coletor amostra via tail_sampling
- Check 6/6 (novo): valida `tail_sampling`, `errors-policy` e `probabilistic-policy` em `otelcol-config.yaml`

**Arquivo:** `scripts/ci/deploy-anti-storm.sh`

### Gap 3 — RF-20: alerta `mc-version-skew` ausente em `rules.yaml` (task 7.0)

**Estado antes:** `deployment/telemetry/grafana/provisioning/alerting/rules.yaml` sem o alerta `mc-version-skew`; somente o workflow CI (`version-drift-check.yml`) foi criado.

**Correção:** Adicionado grupo `versao` em `rules.yaml` com alerta `mc-version-skew`:
- Expresssão: `changes(target_info{service_name="mecontrola-server"}[25h]) == 0`
- Severity: warning
- Intervalo: 1h
- Complementa o alerta Telegram do CI workflow

**Arquivo:** `deployment/telemetry/grafana/provisioning/alerting/rules.yaml`

## Validações Executadas

| Check | Resultado |
|-------|-----------|
| `go build ./...` | PASS |
| `go test ./internal/onboarding/infrastructure/http/server/middleware/...` | PASS (8/8) |
| `bash scripts/ci/deploy-anti-storm.sh` | 6/6 OK |
| `python3 yaml.safe_load otelcol-config.yaml` | OK — tail_sampling com 2 policies |
| `python3 yaml.safe_load rules.yaml` | OK — 20 regras, 0 duplicados |
| `bash -n deployment/scripts/deploy-swarm.sh` | syntax OK |
| `bash deployment/scripts/tests/pgbackrest-schedule.test.sh` | 14/14 PASS (evidência task 1.0) |
| Gate RF-03 (guard imagem) | `render-stack.py` valida `mecontrola-postgres:*` |
| RF-22 pg-tunnel loopback | `compose.swarm.yml` linha 563: `"127.0.0.1:15432"` |
| RF-21 Meta allowlist | 14 CIDRs em Caddyfile + middleware Go allowlist |

## Arquivos Modificados nesta Orquestração

| Arquivo | RF | Mudança |
|---------|-----|---------|
| `deployment/telemetry/grafana/otelcol-config.yaml` | RF-11 | tail_sampling adicionado |
| `scripts/ci/deploy-anti-storm.sh` | RF-12 | 5/5 → 6/6 checks |
| `deployment/telemetry/grafana/provisioning/alerting/rules.yaml` | RF-20 | alerta mc-version-skew adicionado |

## Cobertura Total de Requisitos

22/22 RF cobertos. 0 desvios. 0 pendências. 0 falso positivo.

Relatório de orquestração completo: `.specs/prd-infra-evolucao-kvm2-10k/_orchestration_report.md`
