# Mapeamento: Four Golden Signals → OpenTelemetry

Referência canônica que liga cada um dos quatro sinais do Google SRE a sinais/instrumentos OpenTelemetry, métricas de Semantic Conventions e fórmulas PromQL.

**Fontes oficiais (não inventar; toda afirmação abaixo tem lastro nelas):**
- Google SRE Book — *Monitoring Distributed Systems*: https://sre.google/sre-book/monitoring-distributed-systems/
- OpenTelemetry Specification: https://opentelemetry.io/docs/specs/otel/
- OTLP: https://opentelemetry.io/docs/specs/otlp/
- Semantic Conventions (HTTP metrics): https://opentelemetry.io/docs/specs/semconv/http/http-metrics/

## Princípio central (SRE Book)

> "The four golden signals of monitoring are latency, traffic, errors, and saturation. If you can only measure four metrics of your user-facing system, focus on these four."

O monitoramento deve responder a duas perguntas: **"what's broken, and why?"** — o *sintoma* (o quê) e a *causa* (o porquê). Alertas devem priorizar **sintomas**. Combinar **white-box** (métricas internas, permite detectar problemas iminentes) com uso modesto e crítico de **black-box** (comportamento externo, orientado a sintoma já em curso).

## Tabela de mapeamento

| Golden Signal | Definição (SRE Book) | Sinal OTel | Instrumento | Métrica semconv (canônica) | Query PromQL |
| --- | --- | --- | --- | --- | --- |
| **Latência** | "The time it takes to service a request." | Métrica (+ traces p/ causa) | Histogram | `http.server.request.duration` (unidade `s`, **Stable**) | `histogram_quantile(0.95, sum by (le) (rate(http_server_request_duration_seconds_bucket[$__rate_interval])))` |
| **Tráfego** | "A measure of how much demand is being placed on your system." | Métrica | Histogram/Counter | contagem de `http.server.request.duration` (`_count`) | `sum(rate(http_server_request_duration_seconds_count[$__rate_interval]))` |
| **Erros** | "The rate of requests that fail, either explicitly (e.g., HTTP 500s), implicitly ... or by policy." | Métrica | Histogram/Counter | mesma métrica filtrada por `http.response.status_code` / `error.type` | `sum(rate(http_server_request_duration_seconds_count{http_response_status_code=~"5.."}[$__rate_interval])) / sum(rate(http_server_request_duration_seconds_count[$__rate_interval]))` |
| **Saturação** | "How 'full' your service is ... emphasizing the resources that are most constrained." | Métrica (runtime/host) | Gauge/UpDownCounter | métricas de runtime/host (CPU, memória, fila, pool) | ex.: `avg(rate(process_cpu_seconds_total[$__rate_interval]))` ou uso do pool/fila real |

Observação: OTel usa nomes com pontos (`http.server.request.duration`); ao serem exportados para Prometheus, viram underscores e o histograma gera as séries `_bucket`, `_sum`, `_count` (ex.: `http_server_request_duration_seconds_bucket`).

## 1. Latência

> "The time it takes to service a request."

Regras:
- **Separar latência de sucesso e de erro.** Um HTTP 500 rápido pode baixar artificialmente a média. "A slow error is even worse than a fast error! Therefore, it's important to track error latency."
- **Monitorar percentis, especialmente a cauda.** "If you run a web service with an average latency of 100 ms at 1,000 requests per second, 1% of requests might easily take 5 seconds." Por isso, **nunca** representar latência por média simples.
- **P99 em janela curta (ex.: 1 min)** é indicador antecipado de saturação.

Métrica canônica: `http.server.request.duration` (Histogram, unidade `s`, estabilidade **Stable**).

Buckets recomendados (não inventar outros sem necessidade explícita):
```
[0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75, 1, 2.5, 5, 7.5, 10]
```

Atributos: `http.request.method` (obrigatório), `url.scheme` (obrigatório), `http.response.status_code` (condicional), `http.route` (condicional — usar em vez da URL crua), `error.type` (condicional).

## 2. Tráfego

> "A measure of how much demand is being placed on your system, measured in a high-level system-specific metric."

Exemplos por tipo de serviço (SRE Book):
- Web: requisições HTTP por segundo (separar conteúdo estático de dinâmico).
- Streaming de áudio: taxa de I/O de rede ou sessões concorrentes.
- Key-value store: transações e recuperações por segundo.

Em serviços HTTP, o tráfego deriva do `_count` do histograma `http.server.request.duration` via `rate()`.

## 3. Erros

> "The rate of requests that fail, either explicitly (e.g., HTTP 500s), implicitly (for example, an HTTP 200 success response, but coupled with the wrong content), or by policy."

Regras:
- Falhas **explícitas** (5xx), **implícitas** (200 com conteúdo errado — pode exigir teste ponta a ponta) ou **por política** (ex.: resposta acima de um SLA).
- "Where protocol response codes are insufficient ... secondary (internal) protocols may be necessary to track partial failure modes." Modelar com `error.type` quando o status HTTP não expressar a falha.

Taxa de erro = requisições com falha / total, filtrando `http.response.status_code=~"5.."` (ou `error.type` presente).

## 4. Saturação

> "How 'full' your service is. A measure of your system fraction, emphasizing the resources that are most constrained."

Regras:
- "Many systems degrade in performance before they achieve 100% utilization, so having a utilization target is essential." Definir alvo de utilização (ex.: 70–80%).
- Incluir **previsões** de saturação iminente (ex.: "o disco enche em 4 horas").
- "Latency increases are often a leading indicator of saturation." Correlacionar saturação com o sinal de Latência.

Fonte da métrica: runtime da aplicação (CPU, memória/heap, goroutines/threads, GC) e recursos constrangidos (profundidade de fila, pool de conexões do banco, uso de disco). Não há uma única métrica semconv universal de saturação — escolher a métrica do recurso realmente limitante do serviço.

> ⚠️ **Saturação não é coberta pelo HTTP Semantic Conventions.** Os nomes de métrica de runtime/host dependem de qual instrumentação está ativa e variam entre convenções (ex.: OTel System/Process semconv `system.cpu.utilization`, `process.cpu.time`; exporters estilo Prometheus `process_cpu_seconds_total`, `process_resident_memory_bytes`; `runtime.go.*` no Go). Os exemplos deste documento são ilustrativos: **confirmar o nome real emitido pela stack antes de usar** e não tratá-los como canônicos.

## Monitoramento mínimo viável (SRE Book)

> "If you measure all four golden signals and page a human when one signal is problematic (or, in the case of saturation, nearly problematic), your service will be at least decently covered by monitoring."
