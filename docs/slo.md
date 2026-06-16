# SLO / SLI — MeControla MVP

**Última revisão:** 2026-06-15
**Periodo de medição:** janelas rolantes de 30 dias.
**Stakeholders:** time de plataforma + dono do produto.
**Cross-reference:** `deployment/monitoring/prometheus-rules.yaml` (alertas operacionais).

---

## Princípio

SLO é o **compromisso de qualidade observável** com o cliente. SLIs são as métricas que
medem o SLO. Error budget é o quanto podemos "queimar" antes do SLO falhar — orçamento de
risco para releases, manutenções e mudanças.

**MeControla é MVP single-region single-VPS.** SLOs aqui são conservadores e refletem a
realidade operacional: 1 VPS Hostinger, deploy SSH-based, sem multi-AZ. Aumentar com a maturidade
operacional (HA, multi-region) é trabalho pós-MVP.

---

## SLO-1 · Disponibilidade da API (`/api/v1/*`)

| Atributo | Valor |
|----------|-------|
| **Objetivo** | 99.5% mensal (downtime tolerado: 3h36min/mês) |
| **SLI** | `sum(rate(http_requests_total{route=~"/api/v1/.+",code!~"5.."}[5m])) / sum(rate(http_requests_total{route=~"/api/v1/.+"}[5m]))` |
| **Janela** | 30 dias rolling |
| **Error budget** | 0.5% (43.2 minutos/mês acima do mínimo) |
| **Burn rate alertas** | 14.4× (rápido, 1h); 6× (médio, 6h) |
| **Como manter** | Healthchecks por serviço, graceful shutdown, panic recovery (P0-API1), readiness probe (P1-API2) |

**Exclusões:** janelas planejadas de manutenção anunciadas com 24h antecedência.

## SLO-2 · Latência do Gateway Auth (HMAC verify + DB lookup)

| Atributo | Valor |
|----------|-------|
| **Objetivo** | p99 < 200ms |
| **SLI** | `histogram_quantile(0.99, sum by (le) (rate(identity_gateway_auth_duration_seconds_bucket[5m])))` |
| **Janela** | 30 dias rolling |
| **Threshold de alerta** | p99 > 300ms por 10 min |
| **Como manter** | pgbouncer pool, cache em memória de secrets (CURRENT/NEXT), HMAC em CPU |

## SLO-3 · Lag do Outbox Dispatcher

| Atributo | Valor |
|----------|-------|
| **Objetivo** | p95 < 60 segundos entre `outbox_messages.created_at` e `attempted_at` |
| **SLI** | `histogram_quantile(0.95, sum by (le) (rate(outbox_dispatch_lag_seconds_bucket[5m])))` |
| **Janela** | 30 dias rolling |
| **Threshold de alerta** | p95 > 120s por 5 min; queued > 1000 por 10 min |
| **Como manter** | Tick 500ms, batch 50, retries com exponential backoff base 2s/max 5m. Requer P1-INFRA2 (gauge `outbox_events_pending_count`) |

## SLO-4 · Webhook Kiwify — taxa de aceitação

| Atributo | Valor |
|----------|-------|
| **Objetivo** | > 99% das tentativas válidas resultam em 202 Accepted |
| **SLI** | `sum(rate(kiwify_webhook_total{result="accepted"}[5m])) / sum(rate(kiwify_webhook_total{}[5m]))` |
| **Janela** | 30 dias rolling |
| **Exclusões** | Assinaturas inválidas (401) — não contam contra o SLO, são scan/probe |
| **Threshold de alerta** | < 95% por 15 min OU 5 falhas 5xx consecutivas |
| **Como manter** | Idempotência por `event_id`, retries lado Kiwify, DLQ silenciosa em `kiwify_processed_events` |

## SLO-5 · Backup PITR — sucesso e RPO

| Atributo | Valor |
|----------|-------|
| **Objetivo** | 100% dos backups full semanais + diff diários completos com sucesso; RPO ≤ 5min |
| **SLI** | `pgbackrest_backup_last_success_timestamp_seconds` (exportado por cron */5) |
| **Janela** | 30 dias rolling |
| **Threshold de alerta** | tempo desde último backup > 25h (full+diff) — já configurado em `prometheus-rules.yaml:90-97` |
| **Como manter** | crontab pgBackRest + S3 + restore smoke test trimestral |
| **Restore RTO esperado** | < 20 minutos (documentado em `deployment/runbooks/restore-vps.md`) |

## SLO-6 · Entrega de alertas pró-ativos (canal Telegram/WhatsApp)

⚠️ **Bloqueado por P0-API5 e P0-API7** — sem consumer de alertas + abstração de canal, esse
SLO é inaplicável. Adicionar quando ativar.

| Atributo | Valor (proposto) |
|----------|------------------|
| **Objetivo** | p95 < 2 minutos entre detecção do threshold e mensagem entregue ao canal |
| **SLI** | `histogram_quantile(0.95, sum by (le) (rate(alert_delivery_lag_seconds_bucket[5m])))` |
| **Janela** | 30 dias rolling |
| **Exclusões** | Throttling intencional (1 alerta/user/categoria/dia) |

---

## Error Budget Policy

| Saldo | Política |
|-------|----------|
| > 50% remanescente | Velocidade normal de release; experimentos permitidos. |
| 25–50% | Releases somente em janela de baixo tráfego (madrugada SP). Code freeze em features não críticas. |
| 0–25% | Freeze total de features. Foco em reduzir queima. Postmortem obrigatório por incidente. |
| Negativo | Rollback até o último estado verde + revisão de roadmap. Pause em pull requests não-crítical-fix. |

---

## Alertas operacionais existentes (não-SLO)

Já configurados em `deployment/monitoring/prometheus-rules.yaml`:

| Alerta | Severidade | Trigger |
|--------|------------|---------|
| `DiskSpaceLow` | critical | < 20% disponível por 5min |
| `MemoryPressure` | critical | < 10% disponível por 5min |
| `PostgreSQLDown` | critical | scrape failure por 2min |
| `PostgresCacheHitRatioLow` | warning | hit ratio < 90% por 15min |
| `APIErrorRateHigh` | critical | 5xx > 5% por 5min |
| `SSLCertExpiring` | warning | cert expira em < 30 dias |
| `BackupStale` | critical | > 25h sem backup success |

---

## Como evoluir os SLOs

1. **Após 30 dias em produção:** validar se os objetivos são realistas (não conservadores
   demais nem agressivos demais). Ajustar conforme dados reais.
2. **Após primeiro incidente:** documentar em postmortem o impacto no SLO/error budget;
   considerar adicionar SLI mais específico se gap for detectado.
3. **Evolução de SLOs:** revisar trimestralmente. Sempre subir threshold gradualmente
   (99.5% → 99.7% → 99.9%) só depois de provar que pode sustentar.

---

## Referências

- Google SRE Workbook — capítulo "Implementing SLOs": https://sre.google/workbook/implementing-slos/
- Prometheus rules: `deployment/monitoring/prometheus-rules.yaml`
- Dashboard Grafana: `deployment/grafana/mecontrola-platform.json`
- Restore runbook: `deployment/runbooks/restore-vps.md`
