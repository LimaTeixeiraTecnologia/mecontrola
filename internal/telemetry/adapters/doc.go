// Package adapters implementa os ports declarados em application para o módulo telemetry.
//
// Responsabilidades: implementações concretas de TelemetryRepository (Postgres),
// MetricsSink (Prometheus/OTLP), handlers HTTP de consulta de métricas de produto
// e adaptadores de eventbus. Este pacote PODE importar domain, application e
// infrastructure.
package adapters
