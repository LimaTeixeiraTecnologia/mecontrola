# Observabilidade MVP — fim dos erros OTLP + retencao 30d (metrics/traces/logs)

Data: 2026-06-17
Escopo: local + producao (VPS Hostinger KVM 2 — 2 vCPU / 8 GB / 100 GB / 8 TB)

## Problema original

App (server/worker) inundava stderr com dois erros de exportacao OTLP:

- `Unimplemented: unknown service opentelemetry.proto.collector.logs.v1.LogsService`
- `tls: first record does not look like a TLS handshake` (traces e metrics)

Antes era apenas mascarado com `grep -v` no runbook de deploy.

### Causa-raiz

1. **Logs `Unimplemented`** — devkit-go v0.5.0 (`pkg/observability/otel`) sempre abre os tres
   exporters OTLP (traces, metrics, logs); nao ha flag para desligar logs. Em producao o app
   apontava para o `otelcol` standalone cujo `config.prod.yml` so tinha pipelines de metrics e
   traces — sem pipeline de logs, o servico gRPC `LogsService` nunca era registrado.
2. **TLS handshake** — `server`/`worker`/`migrate` em prod so sobrescreviam o ENDPOINT e herdavam
   `OTEL_EXPORTER_OTLP_INSECURE`/`_PROTOCOL` do `.env` do VPS. Se o `.env` tivesse insecure ausente/false,
   devkit-go usava TLS contra um collector plaintext.
3. Traces iam para o exporter `debug` (descartados) e o Grafana de prod so tinha datasource Prometheus.

## Solucao

Consolidacao no container unico `grafana/otel-lgtm:0.7.5` (collector + Prometheus + Tempo + Loki +
Grafana) em local e producao. Removidos: `otelcol` standalone, `promtail`, e a stack de componentes
(prometheus/grafana/alertmanager/node-exporter/postgres-exporter/blackbox). Logs passam a fluir por um
unico caminho OTLP -> collector -> Loki (com correlacao `trace_id`); promtail e os segredos
`LOKI_URL`/`LOKI_USER_ID`/`LOKI_API_KEY` foram removidos.

### Contrato TLS/insecure (deterministico)

`compose.prod.yml` fixa inline em server/worker/migrate (imune ao `.env` do VPS):

```yaml
OTEL_EXPORTER_OTLP_ENDPOINT: otel-lgtm:4317
OTEL_EXPORTER_OTLP_PROTOCOL: grpc
OTEL_EXPORTER_OTLP_INSECURE: "true"   # plaintext interno; TLS termina no Caddy
```

server/worker tambem fixam `OTEL_TRACE_SAMPLE_RATE: "0.1"` (head-sampling 10% em prod; 1.0 local).

### Retencao 30d obrigatoria com auto-purge (todos os sinais)

O otel-lgtm cru NAO purga logs (Loki sem compactor = retencao infinita). Configurado 30d para os tres
sinais via configs montados em `deployment/telemetry/grafana/`:

| Sinal | Backend | Como | Valor runtime |
|-------|---------|------|---------------|
| Metrics | Prometheus | `run-prometheus.sh` (override) `--storage.tsdb.retention.time=30d` | `storage.tsdb.retention.time: 30d` |
| Traces | Tempo | `tempo-config.yaml` `compactor.compaction.block_retention: 720h` | `block_retention: 720h0m0s` |
| Logs | Loki | `loki-config.yaml` `limits_config.retention_period: 720h` + `compactor.retention_enabled: true` | `retention_period: 30d` |

> Nota de manutencao: `run-prometheus.sh` e uma copia do script embutido do otel-lgtm + flag de
> retencao (Prometheus so aceita retencao via CLI). Ao subir a versao da imagem otel-lgtm, revalidar
> que o script base nao mudou.

### Persistencia (sobrevive a restart)

Volumes nomeados: `otel-lgtm-grafana:/otel-lgtm/grafana/data`, `otel-lgtm-prometheus:/data/prometheus`,
`otel-lgtm-loki:/loki`, `otel-lgtm-tempo:/tmp/tempo`.

### Seguranca

Grafana do otel-lgtm habilita admin anonimo internamente; por isso a porta 3000 fica SO em
`127.0.0.1` (acesso via tunel SSH) e nao ha rota `/grafana` no Caddy. `OTEL_LGTM_ADMIN_PASSWORD`
e obrigatoria em producao (compose falha sem ela).

## Arquivos alterados

- `deployment/compose/compose.yml` — removidos promtail + stack de componentes + volumes orfaos.
- `deployment/compose/compose.prod.yml` — otel-lgtm unico; contrato OTLP fixo; depends_on healthy; sampling 0.1.
- `deployment/compose/compose.local.yml` — volumes de persistencia + mounts de retencao.
- `deployment/telemetry/grafana/otelcol-config.yaml` — `memory_limiter` nos 3 pipelines.
- `deployment/telemetry/grafana/loki-config.yaml` — NOVO (retencao 30d).
- `deployment/telemetry/grafana/tempo-config.yaml` — NOVO (block_retention 30d).
- `deployment/telemetry/grafana/run-prometheus.sh` — NOVO (retencao 30d).
- `deployment/scripts/deploy.sh` — gate `otelcol` -> `otel-lgtm` (container `mecontrola-otel-lgtm-1`).
- `.env.example` — removidos LOKI_*; observabilidade documentada.
- Removidos: `deployment/promtail/config.yml`, `deployment/telemetry/otelcol/config.prod.yml`.

## Verificacao (executada em 2026-06-17, ambiente local — todos PASS)

```bash
# subir stack
docker compose --env-file .env -f deployment/compose/compose.yml -f deployment/compose/compose.local.yml up -d postgres otel-lgtm
# rodar app
DB_HOST=localhost DB_PORT=5432 go run ./cmd migrate
DB_HOST=localhost DB_PORT=5432 go run ./cmd server   # >75s p/ 1 ciclo de metricas

# 1) ZERO erros OTLP no stderr do app
grep -cE "tls: first record|Unimplemented|traces export|failed to upload metrics" <log>   # => 0

# 2) sinais chegam
curl -sG localhost:3100/loki/api/v1/query_range --data-urlencode 'query={service_name="mecontrola-api"}'   # logs
curl -sG localhost:3200/api/search --data-urlencode 'tags=service.name=mecontrola-api'                      # traces
curl -sG localhost:9090/api/v1/query --data-urlencode 'query=count({job="mecontrola-api"})'                 # metrics (99 series)
#   ATENCAO: metrics OTLP mapeiam service.name -> label `job` (nao `service_name`).

# 3) retencao 30d ativa (runtime, nao so config)
curl -s localhost:9090/api/v1/status/flags | grep retention.time     # "30d"
curl -s localhost:3200/status/config | grep block_retention          # 720h0m0s
curl -s localhost:3100/config | grep -E 'retention_period|retention_enabled'   # 30d / true

# 4) collector sem falhas de export
docker exec mecontrola-otel-lgtm-1 curl -s localhost:8888/metrics | grep send_failed_metric_points   # 0
```

Resultados: erros OTLP = 0; logs/traces/metrics presentes; retencao 30d confirmada nos 3 backends;
`otelcol_exporter_send_failed_metric_points{exporter="otlphttp/metrics"} 0`.

## Capacidade (VPS KVM 2)

Em repouso pos-mudanca: ~10% RAM, 2% CPU, 8/100 GB disco. otel-lgtm reserva 256M / limita 1G.
Folga ampla para crescimento de usuarios em 6 meses; retencao de 30d com auto-purge limita o disco.
