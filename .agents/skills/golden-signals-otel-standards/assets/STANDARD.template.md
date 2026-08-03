# Padrão de Telemetria — {{service.name}}

> Documento de padrão de observabilidade baseado nos Four Golden Signals (Google SRE) + OpenTelemetry (Specification, OTLP, Semantic Conventions). Preencher todos os campos `{{...}}` e remover instruções entre colchetes.

## 1. Contexto do serviço

| Campo | Valor |
| --- | --- |
| `service.name` | {{service.name}} |
| Porte | {{pequeno \| médio \| grande}} |
| Tipo | {{REST \| gRPC \| worker \| híbrido}} |
| Linguagem principal | {{Go \| Node.js \| Python \| Java}} |
| Plataforma | {{Kubernetes \| ECS \| VM \| Serverless}} |
| Volume (RPS) | {{...}} |
| SLO disponibilidade | {{99,9%}} |
| SLO latência P95 | {{< 300ms}} |
| Dependências | {{banco, cache, fila, upstream}} |
| Backend de telemetria | {{Grafana \| Coralogix \| OTLP genérico}} |

## 2. Four Golden Signals → OpenTelemetry

[Preencher a partir de references/golden-signals-otel-mapping.md]

| Sinal | Métrica semconv | Atributos | Query PromQL |
| --- | --- | --- | --- |
| Latência | `http.server.request.duration` | `http.request.method`, `http.route`, `http.response.status_code` | `histogram_quantile(0.95, sum by (le) (rate(http_server_request_duration_seconds_bucket[$__rate_interval])))` |
| Tráfego | `http.server.request.duration` (`_count`) | idem | `sum(rate(http_server_request_duration_seconds_count[$__rate_interval]))` |
| Erros | idem, filtrado | `http.response.status_code`, `error.type` | `sum(rate(...count{http_response_status_code=~"5.."}[...])) / sum(rate(...count[...]))` |
| Saturação | {{métrica de runtime/host do recurso limitante}} | {{...}} | {{...}} |

Buckets de latência (Stable): `[0.005, 0.01, 0.025, 0.05, 0.075, 0.1, 0.25, 0.5, 0.75, 1, 2.5, 5, 7.5, 10]`.

## 3. Instrumentação

- Atributos de recurso obrigatórios: `service.name`, `service.version`, `deployment.environment`.
- Estratégia: {{auto-instrumentação (bibliotecas oficiais) + manual para métricas de negócio}}.
- Sinais habilitados: métricas (obrigatório), traces {{sim/não}}, logs correlacionados {{sim/não}}.
- Bootstrap: ver `observability/sdk-bootstrap-*` (linguagem {{...}}).

## 4. Coleta e transporte (OTLP + Collector)

- OTLP: gRPC `4317`, HTTP `4318`.
- Topologia (por porte): {{agent único \| agent + queue \| agent + gateway com tail sampling}}.
- Arquivo: `observability/otel-collector.yaml`.
- Processors: `memory_limiter` {{sim/não}}, `batch` (sim), retry/queue {{...}}.

## 5. Sampling, cardinalidade e custo

- Sampling de traces: {{100% \| head X% \| tail (erros/lentos 100%)}}.
- Métricas: sem sampling (agregadas).
- Regras de cardinalidade: proibidos `user_id`, `request_id`, URL crua; usar `http.route`.
- Decisões de custo/economia: {{...}}.

## 6. Alertas e SLO

- Error budget: `1 − {{SLO}}` = {{...}}.
- Alertas por sintoma + burn rate multi-janela (curta + longa).
- Arquivo: `observability/alert-rules.yaml`.
- Thresholds derivados do SLO: {{...}}.

## 7. Referências oficiais

- Google SRE Book — Monitoring Distributed Systems: https://sre.google/sre-book/monitoring-distributed-systems/
- OpenTelemetry Specification: https://opentelemetry.io/docs/specs/otel/
- OTLP: https://opentelemetry.io/docs/specs/otlp/
- Semantic Conventions: https://opentelemetry.io/docs/specs/semconv/http/http-metrics/
