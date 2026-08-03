# Bootstrap OpenTelemetry — Node.js / Python / Java

Trechos de bootstrap por linguagem. Em todas, garantir os mesmos atributos de recurso (`service.name`, `service.version`, `deployment.environment`) e priorizar auto-instrumentação, que já emite as métricas de semconv dos Golden Signals.

> Ajustar versões de pacote conforme o gerenciador de dependências do projeto. Endpoint OTLP via `OTEL_EXPORTER_OTLP_ENDPOINT` (Collector local, ex.: `http://localhost:4318` para HTTP ou `localhost:4317` para gRPC).

## Node.js / TypeScript

Instalação: `@opentelemetry/sdk-node`, `@opentelemetry/auto-instrumentations-node`, `@opentelemetry/exporter-trace-otlp-grpc`, `@opentelemetry/exporter-metrics-otlp-grpc`, `@opentelemetry/resources`, `@opentelemetry/semantic-conventions`.

```javascript
// instrumentation.js — carregar ANTES do app: node -r ./instrumentation.js app.js
const { NodeSDK } = require('@opentelemetry/sdk-node');
const { getNodeAutoInstrumentations } = require('@opentelemetry/auto-instrumentations-node');
const { OTLPTraceExporter } = require('@opentelemetry/exporter-trace-otlp-grpc');
const { OTLPMetricExporter } = require('@opentelemetry/exporter-metrics-otlp-grpc');
const { PeriodicExportingMetricReader } = require('@opentelemetry/sdk-metrics');
const { resourceFromAttributes } = require('@opentelemetry/resources');
const {
  ATTR_SERVICE_NAME, ATTR_SERVICE_VERSION,
} = require('@opentelemetry/semantic-conventions');

const sdk = new NodeSDK({
  resource: resourceFromAttributes({
    [ATTR_SERVICE_NAME]: 'meu-servico',        // OBRIGATÓRIO
    [ATTR_SERVICE_VERSION]: '1.0.0',
    'deployment.environment': process.env.NODE_ENV || 'dev',
  }),
  traceExporter: new OTLPTraceExporter(),
  metricReader: new PeriodicExportingMetricReader({
    exporter: new OTLPMetricExporter(),
    exportIntervalMillis: 15000,
  }),
  instrumentations: [getNodeAutoInstrumentations()], // emite http.server.request.duration
});
sdk.start();
```

## Python

Opção recomendada (zero-code): `opentelemetry-distro` + `opentelemetry-bootstrap -a install`, executando com `opentelemetry-instrument`.

```bash
pip install opentelemetry-distro opentelemetry-exporter-otlp
opentelemetry-bootstrap -a install

export OTEL_SERVICE_NAME=meu-servico
export OTEL_RESOURCE_ATTRIBUTES="service.version=1.0.0,deployment.environment=prod"
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4317

opentelemetry-instrument python app.py   # auto-instrumenta Flask/FastAPI/Django etc.
```

Bootstrap manual (quando necessário controle fino):

```python
from opentelemetry import trace, metrics
from opentelemetry.sdk.resources import Resource, SERVICE_NAME, SERVICE_VERSION
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import BatchSpanProcessor
from opentelemetry.exporter.otlp.proto.grpc.trace_exporter import OTLPSpanExporter

resource = Resource.create({
    SERVICE_NAME: "meu-servico",          # OBRIGATÓRIO
    SERVICE_VERSION: "1.0.0",
    "deployment.environment": "prod",
})
provider = TracerProvider(resource=resource)
provider.add_span_processor(BatchSpanProcessor(OTLPSpanExporter()))
trace.set_tracer_provider(provider)
```

## Java / JVM

Opção recomendada (zero-code): agente Java, sem alterar o código.

```bash
# baixar opentelemetry-javaagent.jar (releases oficiais do opentelemetry-java-instrumentation)
export OTEL_SERVICE_NAME=meu-servico
export OTEL_RESOURCE_ATTRIBUTES="service.version=1.0.0,deployment.environment=prod"
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4317

java -javaagent:opentelemetry-javaagent.jar -jar meu-servico.jar
# auto-instrumenta Spring/Servlet e emite http.server.request.duration
```

## Regras comuns
- `service.name` é obrigatório em todas as linguagens; padronizar também `service.version` e `deployment.environment`.
- Preferir auto-instrumentação para os Golden Signals de servidor; reservar código manual para métricas de negócio.
- Manter os mesmos nomes de semconv entre serviços para dashboards/alertas portáveis.
- Não colocar atributos de alta cardinalidade em métricas (ver `references/sampling-cost-efficiency.md`).
