# Dashboards Grafana — MeControla

Dashboards para o stack OpenTelemetry consolidado no `grafana/otel-lgtm`
(Prometheus + Tempo + Loki + Grafana). Veja `docs/runbooks/2026-06-17-otel-observability-fix.md`.

## Arquivos

| Arquivo | Foco | Datasources |
|---------|------|-------------|
| `mecontrola-api.json` | RED da API (rate, errors, duration) + DB client | Prometheus |
| `mecontrola-ops.json` | Saúde operacional: pool/tx do DB, outbox, onboarding, throughput de negócio, logs | Prometheus + Loki |

## Como importar

Grafana (túnel SSH em `http://localhost:3001`) → **Dashboards → New → Import** →
*Upload JSON file* (ou cole o conteúdo) → ao importar, selecione os datasources
**Prometheus** e **Loki** quando solicitado (variáveis de template `datasource` / `loki`).

> Os dashboards usam variáveis de datasource (não hardcodam UID/URL), então funcionam
> tanto no otel-lgtm quanto em qualquer Grafana com Prometheus/Loki.

## Variáveis de template

- `datasource` — datasource Prometheus.
- `loki` — datasource Loki (só no `mecontrola-ops`).
- `job` — serviço (`mecontrola-api` no RED; multi `api`+`worker` no ops).

## Métricas usadas (OTel → Prometheus)

Nomes confirmados no Prometheus da VPS. Atenção às unidades: latência do **HTTP server
em segundos**, **client/DB em milissegundos**.

**HTTP server (RED):**
- `http_server_request_count_total` — counter; labels `http_route`, `http_request_method`, `http_response_status_code`.
- `http_server_request_duration_seconds_bucket|_sum|_count` — histograma (s).
- `http_server_request_active` — gauge (requests em voo).
- Erro = filtro `http_response_status_code=~"5.."` (não há counter de erro dedicado quando não há erros).

**HTTP client (saída):** `http_client_request_count_total`, `http_client_request_duration_milliseconds_*`.

**Banco (devkit-go manager):**
- `database_pool_connections_open` / `database_pool_connections_idle` — gauges.
- `database_pool_wait_count_total`, `database_pool_wait_duration_ms_milliseconds_total` — counters.
- `database_query_duration_ms_milliseconds_*`, `database_tx_duration_ms_milliseconds_*` — histogramas (ms).
- `database_tx_committed_total` — counter.

**Negócio:**
- Outbox: `outbox_events_inserted_total`.
- Onboarding: `onboarding_tokens_paid_total` (worker), `onboarding_tokens_consumed_total` (api),
  `onboarding_tokens_paid_unconsumed_ratio` (gauge — fila de pagos sem consumir),
  `onboarding_checkout_sessions_created_total`, `onboarding_email_sent_total`,
  `onboarding_activation_email_dispatched_total`, `onboarding_paid_to_consumed_seconds_*`.
- Billing/canais: `billing_webhooks_received_total`, `meta_inbound_messages_total`, `whatsapp_dispatcher_route_total`.

**Logs (Loki):** stream `{service_name=~"mecontrola-.+", detected_level=~"error|warn"}`.

## Provisionamento automático (ativo)

Dashboards **e alertas** sobem sozinhos a cada deploy — não precisa importar à mão.
O `otel-lgtm` monta (ver `compose.*.yml`):

- `../dashboards` → `/etc/dashboards` + provider `dashboards-provider.yaml` → dashboards aparecem na pasta **MeControla**.
- `../telemetry/grafana/provisioning/alerting/rules.yaml` → 6 regras de alerta (pasta **MeControla Alerts**).

## Alertas proativos

Regras provisionadas via arquivo (`provisioning/alerting/rules.yaml`):

| Grupo | Regra | Dispara quando |
|-------|-------|----------------|
| tecnico | API sem métricas (down) | sem `http_server_request_active` há >5min |
| tecnico | API erro 5xx alto | 5xx > 5% por 5min |
| tecnico | API latência p99 alta | p99 > 1s por 10min |
| tecnico | Postgres pool com espera | >1 wait/s por 5min |
| negocio | Tokens pagos não consumidos | `onboarding_tokens_paid_unconsumed_ratio` > 25% por 15min |
| plataforma | Collector falhando export | `otelcol_exporter_send_failed_metric_points` > 0 por 5min |

> Alertas de **queda de volume** (ex.: "webhooks pararam") foram omitidos de propósito —
> num produto novo de baixo tráfego eles geram falso-positivo. Reavaliar quando houver baseline.
> **Disco** precisa de node-exporter (removido no MVP) — adicionar exporter antes de alertar disco.

### Notificação no Telegram (contém segredo → via API, não em arquivo)

O contact point + notification policy do Telegram são criados por
`deployment/telemetry/grafana/setup-alerting-telegram.sh` (idempotente). O Grafana 11 coage
`chatid` numérico a number quando interpolado de env em arquivo, então o segredo entra pela API
(onde controlamos o tipo string). No deploy isso roda automático se `ALERT_TELEGRAM_*` estiverem no `.env`.

Manual (na VPS, com otel-lgtm no ar):

```bash
cd /opt/mecontrola && set -a && . ./.env && set +a
GRAFANA_ADMIN_PASSWORD="$OTEL_LGTM_ADMIN_PASSWORD" \
  bash deployment/telemetry/grafana/setup-alerting-telegram.sh
```

Variáveis no `.env`: `ALERT_TELEGRAM_BOT_TOKEN`, `ALERT_TELEGRAM_CHAT_ID`. O script envia uma
mensagem de teste ao final. Sem as variáveis, os alertas apenas avaliam no painel (Alerting).
