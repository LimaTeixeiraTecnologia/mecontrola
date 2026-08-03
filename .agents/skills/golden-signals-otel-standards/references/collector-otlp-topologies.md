# Coleta e Transporte: OTLP + OpenTelemetry Collector

Como transportar telemetria via OTLP e desenhar a topologia do Collector por porte. Configs prontas em `assets/otel-collector.{small,medium,large}.yaml`.

**Fontes:** OTLP Specification (https://opentelemetry.io/docs/specs/otlp/), OpenTelemetry Collector docs.

## OTLP — o protocolo

> "The OpenTelemetry Protocol (OTLP) specification describes the encoding, transport, and delivery mechanism of telemetry data between telemetry sources, intermediate nodes such as collectors and telemetry backends."

Transportes:
- **gRPC** — request/response com Protocol Buffers. **Porta default 4317.**
- **HTTP + Protobuf binário** — POST HTTP/1.1 ou HTTP/2. **Porta default 4318.**
- **HTTP + JSON** — proto3 JSON; trace/span IDs como strings hex.

Confiabilidade (a instrumentação/Collector como client DEVE respeitar):
- **Retry**: interpretar status gRPC como retryable/não-retryable; usar **backoff exponencial com jitter**.
- **Backpressure/throttling**: "If the server is unable to keep up with the pace of data it receives from the client then it SHOULD signal that fact to the client." O servidor sinaliza com `Unavailable` + `RetryInfo`; o client deve respeitar o tempo de recuperação.
- **Partial success**: a resposta pode indicar sucesso total, parcial ou falha.

O **Collector** atua como **client e server** ao mesmo tempo: recebe telemetria (receiver) e a encaminha (exporter) para o backend.

## Anatomia de um pipeline do Collector

- **receivers** — entrada (ex.: `otlp` em 4317/4318).
- **processors** — transformação em cadeia (`memory_limiter`, `batch`, `tail_sampling`, `attributes`, `resource`).
- **exporters** — saída (ex.: `otlp`/`otlphttp` para o backend; `debug` local).
- **service.pipelines** — liga receivers → processors → exporters por sinal (`traces`, `metrics`, `logs`).

Ordem recomendada de processors: `memory_limiter` primeiro, `batch` por último antes do exporter.

## Topologia por porte

### Pequeno porte (`otel-collector.small.yaml`)
- **Agent único** (sidecar/DaemonSet ou processo local), sem gateway.
- Pipeline enxuto: `otlp` → `memory_limiter` (opcional) → `batch` → exporter OTLP.
- Sem tail sampling. Head sampling na aplicação, se necessário.
- Objetivo: simplicidade e custo mínimo de operação.

### Médio porte (`otel-collector.medium.yaml`)
- **Agent** com `memory_limiter` obrigatório, `batch`, e `sending_queue` + `retry_on_failure` no exporter.
- Head sampling parametrizável.
- Objetivo: robustez (não perder dados em picos) mantendo footprint moderado.

### Grande porte (`otel-collector.large.yaml`)
- **Agent (coleta) + Gateway (agregação)**. Agentes enviam para uma camada de gateway.
- Gateway com **`tail_sampling`** (decisão de amostragem com o trace completo), balanceamento de carga entre instâncias e filas persistentes.
- Objetivo: escala horizontal, decisão de sampling informada e resiliência.

## Regras transversais
- Sempre aplicar `batch` (reduz número de requisições e custo de rede).
- Usar `memory_limiter` a partir do médio porte para evitar OOM sob picos.
- Métricas **não** passam por tail sampling (são agregadas); tail sampling é só para traces.
- Confirmar o exporter final conforme o backend do Passo 1 (Grafana/Prometheus, Coralogix ou OTLP genérico).
