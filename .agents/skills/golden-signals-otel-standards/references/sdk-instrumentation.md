# Instrumentação de SDK OpenTelemetry

Regras de instrumentação para emitir os Four Golden Signals com OpenTelemetry, com Go como referência e trechos para Node.js, Python e Java. Detalhe de código completo em `assets/sdk-bootstrap-go.md` e `assets/sdk-bootstrap-polyglot.md`.

**Fontes:** OTel Specification (https://opentelemetry.io/docs/specs/otel/), Semantic Conventions (https://opentelemetry.io/docs/concepts/semantic-conventions/), Signals (https://opentelemetry.io/docs/concepts/signals/).

## Sinais OTel (o que instrumentar)

A Specification define quatro sinais: **traces**, **metrics**, **logs** e **baggage** (profiles em desenvolvimento).

- **Métricas** — obrigatórias para os Golden Signals. Cobrem Latência (histogram), Tráfego (count), Erros (count filtrado) e Saturação (runtime/host).
- **Traces** — respondem ao "porquê" (causa). Um span carrega nome, contexto, atributos, eventos, links e status.
- **Logs** — correlacionados ao trace context (mesmo `trace_id`/`span_id`) para navegação métrica → trace → log.
- **Baggage** — propaga contexto entre serviços; não usar para dados sensíveis nem como fonte de métricas de alta cardinalidade.

## Atributos de recurso (Resource) — obrigatórios

Todo serviço DEVE declarar, no mínimo:
- `service.name` — **obrigatório** (identidade do serviço).
- `service.version` — versão do build/deploy.
- `deployment.environment` (ou `deployment.environment.name`) — `dev`, `staging`, `prod`.

Sem `service.name` consistente, métricas ficam órfãs e não é possível filtrar por serviço nos dashboards/alertas.

## Auto-instrumentação vs. manual

1. **Priorizar auto-instrumentação oficial**: middlewares/bibliotecas que já emitem métricas de semconv (ex.: instrumentação HTTP gera `http.server.request.duration` automaticamente). Menor esforço, nomes canônicos garantidos.
2. **Instrumentação manual** apenas para métricas de negócio e spans de domínio que a auto-instrumentação não cobre.
3. **Não duplicar**: se auto e manual coexistirem para o mesmo sinal, deduplicar e preferir o nome de semconv.

## Instrumentos de métrica (OTel)

| Instrumento | Uso típico | Golden Signal |
| --- | --- | --- |
| Histogram | duração de requisição | Latência, Tráfego (`_count`), Erros (`_count` filtrado) |
| Counter | eventos monotônicos (ex.: mensagens processadas) | Tráfego/Erros de negócio |
| UpDownCounter | valores que sobem e descem (ex.: requisições ativas) | Saturação |
| Gauge (Observable) | leitura instantânea (ex.: uso de memória) | Saturação |

## Referência por linguagem

- **Go** (referência): `assets/sdk-bootstrap-go.md` — configuração completa de tracer, meter e logs com exporter OTLP, resource e semconv, mais middleware HTTP (`otelhttp`) que emite as métricas de servidor.
- **Node.js / Python / Java**: `assets/sdk-bootstrap-polyglot.md` — bootstrap por SDK + auto-instrumentação. Em todas as linguagens, garantir os mesmos atributos de recurso e as mesmas métricas de semconv, para que dashboards e alertas sejam portáveis entre serviços.

## Regras de cardinalidade na instrumentação

- **Nunca** usar como atributo de métrica: `user_id`, `request_id`, `session_id`, `trace_id`, ou a URL crua com parâmetros. Explodem a cardinalidade e o custo.
- Usar `http.route` (o template da rota, ex.: `/users/{id}`), não o path concreto.
- Dados de alta cardinalidade pertencem a **traces/logs** (via atributos de span), não a métricas.
