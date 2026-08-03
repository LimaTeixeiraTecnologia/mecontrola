# Queries PromQL — Four Golden Signals

Queries canônicas dos quatro sinais sobre a métrica de Semantic Convention `http.server.request.duration`, exportada para Prometheus como `http_server_request_duration_seconds_{bucket,sum,count}`.

Convenções:
- Substituir `$service` pelo filtro real (ex.: `service_name="meu-servico"`).
- Usar `$__rate_interval` em Grafana; em Prometheus puro, usar uma janela fixa (ex.: `5m`).
- **Nunca** usar média simples para latência — sempre `histogram_quantile`.

## Latência (P50 / P95 / P99)

```promql
# P95
histogram_quantile(0.95,
  sum by (le) (rate(http_server_request_duration_seconds_bucket{$service}[$__rate_interval]))
)

# P99
histogram_quantile(0.99,
  sum by (le) (rate(http_server_request_duration_seconds_bucket{$service}[$__rate_interval]))
)

# P95 por rota (identifica endpoints lentos)
histogram_quantile(0.95,
  sum by (le, http_route) (rate(http_server_request_duration_seconds_bucket{$service}[$__rate_interval]))
)
```

## Tráfego (RPS)

```promql
# Requisições por segundo (total)
sum(rate(http_server_request_duration_seconds_count{$service}[$__rate_interval]))

# RPS por rota e método
sum by (http_route, http_request_method) (
  rate(http_server_request_duration_seconds_count{$service}[$__rate_interval])
)
```

## Erros (taxa de erro)

```promql
# Proporção de 5xx sobre o total
sum(rate(http_server_request_duration_seconds_count{$service,http_response_status_code=~"5.."}[$__rate_interval]))
/
sum(rate(http_server_request_duration_seconds_count{$service}[$__rate_interval]))

# Erros por rota (localiza o endpoint problemático)
sum by (http_route) (
  rate(http_server_request_duration_seconds_count{$service,http_response_status_code=~"5.."}[$__rate_interval])
)
```

## Saturação (recurso limitante)

A saturação depende do recurso realmente constrangido do serviço. Os nomes abaixo são **exemplos ilustrativos** (convenção de exporters de runtime/host — não fazem parte do HTTP Semantic Conventions). Confirmar o nome real emitido pela stack (OTel System/Process semconv, `runtime.*`, ou exporter Prometheus) antes de usar:

```promql
# CPU do processo (runtime)
avg(rate(process_cpu_seconds_total{$service}[$__rate_interval]))

# Memória residente (runtime) — comparar com o limite do container/host
process_resident_memory_bytes{$service}

# Requisições ativas (indicador de fila/saturação de concorrência)
sum(http_server_active_requests{$service})
```

Escolher a métrica do recurso limitante real (pool de conexões do banco, profundidade de fila, heap, etc.). Latência crescente é indicador antecipado de saturação — correlacionar com as queries de Latência acima.
