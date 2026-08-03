# Bootstrap OpenTelemetry — Go (referência)

Configuração completa de OTel em Go: resource com semconv, exporter OTLP (gRPC), tracer, meter e middleware HTTP que emite automaticamente `http.server.request.duration` (Latência/Tráfego/Erros).

> Ajustar versões dos módulos conforme o `go.mod` do projeto. Os nomes de pacotes/tipos abaixo seguem a API estável do OTel Go.

## Dependências (go.mod)

```
go.opentelemetry.io/otel
go.opentelemetry.io/otel/sdk
go.opentelemetry.io/otel/sdk/metric
go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc
go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc
go.opentelemetry.io/otel/semconv/v1.26.0
go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp
```

## Inicialização (resource + tracer + meter)

```go
package observability

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// Setup configura tracer e meter globais com exporter OTLP.
// Retorna uma função de shutdown que deve ser chamada no encerramento do processo.
func Setup(ctx context.Context, serviceName, serviceVersion, env string) (func(context.Context) error, error) {
	// service.name é OBRIGATÓRIO. Sempre incluir version e environment.
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion(serviceVersion),
			// Chave literal para robustez entre versões de semconv
			// (o helper de deployment.environment mudou de nome entre versões).
			attribute.String("deployment.environment", env),
		),
	)
	if err != nil {
		return nil, err
	}

	// Exporters OTLP/gRPC (porta padrão 4317). Endpoint via OTEL_EXPORTER_OTLP_ENDPOINT.
	traceExp, err := otlptracegrpc.New(ctx)
	if err != nil {
		return nil, err
	}
	metricExp, err := otlpmetricgrpc.New(ctx)
	if err != nil {
		return nil, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithBatcher(traceExp),
		// Head sampling: ajustar ao porte. Pequeno: AlwaysSample; médio: TraceIDRatioBased(0.1).
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(0.1))),
	)
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExp,
			sdkmetric.WithInterval(15*time.Second))),
	)

	otel.SetTracerProvider(tp)
	otel.SetMeterProvider(mp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{}, propagation.Baggage{},
	))

	return func(ctx context.Context) error {
		_ = tp.Shutdown(ctx)
		return mp.Shutdown(ctx)
	}, nil
}
```

## Middleware HTTP (emite os Golden Signals automaticamente)

```go
import "go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

// otelhttp emite http.server.request.duration (histogram) e http.server.active_requests,
// já com atributos http.request.method, http.route, http.response.status_code.
handler := otelhttp.NewHandler(mux, "http.server")
// Usar WithRouteTag para gravar http.route (template) em vez do path cru:
//   mux.Handle("/users/{id}", otelhttp.WithRouteTag("/users/{id}", usersHandler))
```

## Métrica de negócio manual (exemplo)

```go
meter := otel.Meter("meu-servico")
processed, _ := meter.Int64Counter(
	"orders.processed",
	metric.WithDescription("Pedidos processados"),
	metric.WithUnit("{order}"),
)
// NÃO adicionar atributos de alta cardinalidade (user_id, order_id) na métrica.
processed.Add(ctx, 1, metric.WithAttributes(attribute.String("status", "ok")))
```

## Regras
- `service.name` sempre presente; `service.version` e `deployment.environment` também.
- Preferir `otelhttp`/`otelgrpc` oficiais para os Golden Signals de servidor.
- Ajustar o sampler ao porte (ver `references/sampling-cost-efficiency.md`).
- Definir o endpoint via `OTEL_EXPORTER_OTLP_ENDPOINT` (aponta para o Collector local, `localhost:4317`).
