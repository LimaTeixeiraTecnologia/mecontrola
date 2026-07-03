# Relatório de Orquestração — Evolução de Infraestrutura KVM 2

PRD: `infra-evolucao-kvm2-10k`
Iniciado: 2026-07-03T14:55:44Z
Concluído: 2026-07-03T15:05:52Z
Status final: **done**

## Snapshot Inicial

| Campo | Valor |
|-------|-------|
| Total de tarefas | 8 |
| Pendentes | 0 |
| Done | 8 |
| Falhas | 0 |

## Waves Executadas

| Wave | Tarefas | Status |
|------|---------|--------|
| F0 (paralelo) | 1.0, 7.0 | done |
| F0.1 | 2.0 (dep: 1.0) | done |
| F1 | 3.0 (dep: 7.0) | done |
| F2 | 4.0 | done |
| F3 | 5.0 | done |
| F3.1 | 6.0 (dep: 4.0, 5.0) | done |
| F4 | 8.0 | done |

## Tarefas Executadas

| # | Título | Status | Requisitos | Observações |
|---|--------|--------|-----------|-------------|
| 1.0 | Backup pgBackRest agendado, alertado e com guard de imagem | done | RF-01, RF-02, RF-03 | systemd-timer + métricas textfile + guard render-stack.py |
| 2.0 | Ensaio de restore PITR e restore de VPS com evidência | done | RF-04, RF-05, RF-06 | Runbooks atualizados; RPO ≤ 10 min / RTO PITR ≤ 45 min / RTO VPS ≤ 90 min |
| 3.0 | Remover runner do host e migrar deploy para GitHub-hosted SSH | done | RF-07, RF-08, RF-09, RF-10 | remove-runner.sh + ci-cd.yml SSH + docker-prune timer + alerta disco |
| 4.0 | Sampling de traces proporcional e ajuste do gate anti-storm | done | RF-11, RF-12, RF-13 | tail_sampling (errors 100% + probabilistic 10%) em otelcol; gate 6/6 |
| 5.0 | Orçamento de recursos, pool de conexões e alerta de saturação | done | RF-14, RF-15, RF-16 | memory limits ajustados; pool 20/30; alertas pgBouncer; orçamento documentado |
| 6.0 | Harness de carga k6 e prova dos envelopes A e B | done | RF-17, RF-18 | whatsapp-inbound.js + transactions-read.js; taskfile loadtest |
| 7.0 | Deploy da main em produção e alerta de drift de versão | done | RF-19, RF-20 | version-drift-check.yml (Telegram 24h) + alerta mc-version-skew (Grafana) |
| 8.0 | Endurecimento de superfície: rate-limit Meta e bind pg-tunnel | done | RF-21, RF-22 | allowlist 14 CIDRs Meta (Caddy + middleware Go); pg-tunnel → 127.0.0.1:15432 |

## Correções Aplicadas nesta Execução de Orquestração

| Requisito | Gap Detectado | Correção |
|-----------|--------------|----------|
| RF-11 | `otelcol-config.yaml` sem `tail_sampling` (relatório 4.0 afirmava ter adicionado mas não estava presente) | Adicionado `tail_sampling` com `errors-policy` (100%) e `probabilistic-policy` (10%) |
| RF-12 | `deploy-anti-storm.sh` ainda com 5/5 checks; check 5 exigia `"1"` sem validar otelcol | Atualizado para 6/6 checks; check 6 valida `tail_sampling` + policies em otelcol |
| RF-20 | `mc-version-skew` ausente de `rules.yaml` (relatório 7.0 afirmava ter adicionado) | Alerta `mc-version-skew` adicionado ao grupo `versao` em rules.yaml |

## Cobertura de Requisitos

| RF | Título | Status |
|----|--------|--------|
| RF-01 | pgBackRest agendado (systemd-timer/cron idempotente) | ✓ |
| RF-02 | Alerta de backup stale e archive-push falho | ✓ |
| RF-03 | Guard de imagem custom no deploy de produção | ✓ |
| RF-04 | Ensaio de restore PITR com evidência | ✓ |
| RF-05 | Ensaio de restore de VPS com evidência | ✓ |
| RF-06 | Runbooks com RPO/RTO reais e SLO | ✓ |
| RF-07 | Remoção total do runner self-hosted | ✓ |
| RF-08 | Jobs build/scan/sign em GitHub-hosted | ✓ |
| RF-09 | Deploy via SSH de GitHub-hosted | ✓ |
| RF-10 | Higiene de disco recorrente + alerta | ✓ |
| RF-11 | Sampling proporcional com error-bias (tail_sampling) | ✓ |
| RF-12 | Gate anti-storm ajustado (6/6 checks) | ✓ |
| RF-13 | SPOF single-node e retenção documentados | ✓ |
| RF-14 | Memory limits com margem em 8 GB | ✓ |
| RF-15 | Pool dimensionado + alerta de saturação | ✓ |
| RF-16 | Orçamento e gatilho KVM2→KVM4 documentados | ✓ |
| RF-17 | Harness k6 envelopes A e B | ✓ |
| RF-18 | Critérios de aprovação e relatório de evidência | ✓ |
| RF-19 | Deploy main + verificação de saúde | ✓ |
| RF-20 | Alerta de drift de versão (CI + Grafana) | ✓ |
| RF-21 | Allowlist CIDRs Meta no rate-limit | ✓ |
| RF-22 | pg-tunnel restrito ao loopback | ✓ |

## Validações Finais

- `go build ./...` → OK
- `go test ./internal/onboarding/infrastructure/http/server/middleware/...` → PASS (8/8)
- `bash scripts/ci/deploy-anti-storm.sh` → 6/6 OK
- `python3 yaml.safe_load otelcol-config.yaml` → OK; `tail_sampling` presente com 2 policies
- `python3 yaml.safe_load rules.yaml` → OK; 20 regras, 0 duplicados
- `bash -n deployment/scripts/deploy-swarm.sh` → syntax OK
