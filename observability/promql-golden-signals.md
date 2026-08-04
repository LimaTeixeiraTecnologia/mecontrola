# Queries PromQL — Four Golden Signals (mecontrola-api / mecontrola-worker)

> Queries reais em uso pelo sistema (dashboards `deployment/dashboards/mecontrola-api.json` /
> `mecontrola-infra.json` e alertas `deployment/telemetry/grafana/provisioning/alerting/rules.yaml`),
> sobre a métrica de Semantic Convention `http.server.request.duration`, exportada para Prometheus
> como `http_server_request_duration_seconds_{bucket,sum,count}` (fonte canônica de RED — ADR-004).
> Ver `observability/STANDARD.md` para o documento completo. Nenhuma query abaixo é hipotética —
> todas foram extraídas literalmente do provisioning real ou confirmadas por query ao vivo contra um
> Prometheus alimentado por telemetria de produção (Tarefas 6.0/8.0).

Convenções:
- `job="mecontrola-api"` filtra o serviço HTTP; substituir por `job="mecontrola-worker"` quando
  aplicável (worker não expõe RED HTTP — usa `worker_heartbeat` para liveness, seção Saturação).
- Janela fixa `[5m]` usada nos alertas provisionados; em dashboards Grafana, `$__rate_interval` é
  aceito no lugar de qualquer janela fixa abaixo.
- **Nunca** usar média simples para latência — sempre `histogram_quantile`.

## Latência (P95 / P99)

```promql
# P99 (usada por mc-api-latency-p99)
histogram_quantile(0.99, sum by (le) (rate(http_server_request_duration_seconds_bucket{job="mecontrola-api"}[5m])))

# P95 (usada por mc-slo-latency-p95, ligada ao SLO de latência)
histogram_quantile(0.95, sum by (le) (rate(http_server_request_duration_seconds_bucket{job="mecontrola-api"}[5m])))

# P95 por rota (identifica endpoints lentos)
histogram_quantile(0.95, sum by (le, http_route) (rate(http_server_request_duration_seconds_bucket{job="mecontrola-api"}[5m])))
```

## Tráfego (RPS)

```promql
# Requisições por segundo (total)
sum(rate(http_server_request_duration_seconds_count{job="mecontrola-api"}[5m]))

# RPS por rota e método
sum by (http_route, http_request_method) (rate(http_server_request_duration_seconds_count{job="mecontrola-api"}[5m]))
```

## Erros (taxa de erro)

```promql
# Taxa de erro em % (usada por mc-api-5xx, RECONCILIACAO ADR-004/RF-08 — migrada do counter
# auxiliar http_server_request_count_total para a serie canonica do histograma)
100 * sum(rate(http_server_request_duration_seconds_count{job="mecontrola-api", http_response_status_code=~"5.."}[5m]))
  / clamp_min(sum(rate(http_server_request_duration_seconds_count{job="mecontrola-api"}[5m])), 1)

# Erros por rota (localiza o endpoint problemático)
sum by (http_route) (rate(http_server_request_duration_seconds_count{job="mecontrola-api", http_response_status_code=~"5.."}[5m]))
```

## Burn-rate do error budget de disponibilidade (SLO 99,9%, ADR-002)

Fórmula geral (`W` = janela): mesma razão de erro acima, avaliada em duas janelas simultâneas
(longa + curta) e comparada ao threshold `multiplicador * 0.001`.

```promql
# Janela longa 1h (mc-slo-burn-fast, threshold 0.0144 = 14.4 * 0.001)
sum(rate(http_server_request_duration_seconds_count{job="mecontrola-api", http_response_status_code=~"5.."}[1h]))
  / clamp_min(sum(rate(http_server_request_duration_seconds_count{job="mecontrola-api"}[1h])), 1)

# Janela curta 5m (confirma o burn em curso, mesmo threshold)
sum(rate(http_server_request_duration_seconds_count{job="mecontrola-api", http_response_status_code=~"5.."}[5m]))
  / clamp_min(sum(rate(http_server_request_duration_seconds_count{job="mecontrola-api"}[5m])), 1)
```

| uid | Janela longa | Janela curta | Threshold (multiplicador × 0.001) |
| --- | --- | --- | --- |
| `mc-slo-burn-fast` | `[1h]` | `[5m]` | `0.0144` (14,4×) |
| `mc-slo-burn-6x` | `[6h]` | `[30m]` | `0.006` (6×) |
| `mc-slo-burn-3x` | `[1d]` | `[2h]` | `0.003` (3×) |
| `mc-slo-burn-1x` | `[3d]` | `[6h]` | `0.001` (1×) |

## Saturação (recurso limitante — runtime Go)

Nomes confirmados ao vivo (Tarefa 6.0) contra Prometheus alimentado pela instrumentação real de
produção (`internal/platform/observability/runtimemetrics`, `go.opentelemetry.io/contrib/instrumentation/runtime`
v0.69.0) — não são exemplos ilustrativos, são as séries reais deste sistema:

```promql
# Goroutines (usada por mc-runtime-goroutine-growth, threshold 3000)
max(go_goroutine_count{job=~"mecontrola-.+"})

# Memória usada pelo runtime Go, por tipo (heap/stack)
go_memory_used_bytes{job=~"mecontrola-.+"}

# Alvo de heap do GC
go_memory_gc_goal_bytes{job=~"mecontrola-.+"}

# GOMAXPROCS efetivo
go_processor_limit{job=~"mecontrola-.+"}
```

`go_memory_limit_bytes` (limite `GOMEMLIMIT`), `go_memory_allocated_bytes_total`,
`go_memory_allocations_total` e `go_config_gogc_percent` existem no exporter mas não são usadas em
alerta/painel atual — ver `observability/STANDARD.md` seção 8.2 para a justificativa de cada uma
(a primeira por não ter `GOMEMLIMIT` configurado no deployment; as demais por não terem sido
necessárias para os alertas/dashboards desta iniciativa).

## Liveness (worker, RF-11)

```promql
# Ausência de heartbeat por 5 minutos (usada por mc-worker-down)
absent(worker_heartbeat{job="mecontrola-worker"})

# Ausência de métricas HTTP da API por 5 minutos (usada por mc-api-down)
absent(http_server_request_active{job="mecontrola-api"})
```
