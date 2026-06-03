// Package observability compõe devkit-go/pkg/observability com OTLP gRPC para exportação
// de traces, métricas e logs correlacionados ao Grafana Cloud.
//
// # Contrato
//
//   - Toda telemetria é exportada via OTLP gRPC (padrão) ou HTTP, configurável via O11yConfig.
//   - Resource attributes obrigatórios: service.name=mecontrola, service.version, deployment.environment.
//   - Logs estruturados JSON via slog + otelslog; trace_id e span_id propagados automaticamente.
//   - Redaction de PII é OBRIGATÓRIA: nenhum trace, span ou log pode emitir phone, password, token,
//     card_number ou amount sem mascaramento. A lista de campos redatados está em redaction.go.
//   - Métricas custom da foundation: bootstrap_duration_seconds, events_published_total,
//     health_probe_status; definidas em metrics.go.
//
// # Invariantes
//
//   - NewProvider falha explicitamente se o endpoint OTLP não estiver configurado.
//   - shutdown(ctx) drena spans, métricas e logs pendentes; idempotente.
//   - Em production, conexões insecure são bloqueadas pelo devkit-go (validateSecurityConfig).
//   - Logs com campos PII são mascarados pela camada infra via NewRedactingSlogHandler (pii_handler.go).
//     Esta implementação local garante redaction de phone e amount que não constam em
//     devkit-go@v0.4.0/pkg/observability/otel.sensitiveKeysLower.
//
// # Uso do handler PII
//
//	handler := observability.NewRedactingSlogHandler(slog.NewJSONHandler(os.Stdout, nil))
//	logger := slog.New(handler)
//	logger.Info("pag", slog.String("phone", "+55...")) // → phone="[REDACTED]"
package observability
