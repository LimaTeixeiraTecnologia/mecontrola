package outbox

import (
	"log/slog"
	"time"
)

// logFields modela o allowlist canônico de campos permitidos em logs do pacote outbox.
// Implementa RF-23 e RF-31: apenas os 6 campos abaixo (mais extensões documentadas)
// podem aparecer em chamadas slog.*. O campo payload JAMAIS deve ser incluído (RF-24).
//
// Campos canônicos (RF-31):
//   - event_id
//   - event_type
//   - subscription_name
//   - attempt
//   - correlation_id
//   - error_class
//
// Extensões documentadas (fora do allowlist por design, usadas em transições específicas):
//   - latency_ms     — duração de processamento em ms (outbox.delivery.processed)
//   - next_retry_at  — próxima tentativa ISO-8601 (outbox.delivery.failed)
//   - total_attempts — total de tentativas ao atingir DLQ (outbox.delivery.dlq)
type logFields struct {
	EventID          string
	EventType        string
	SubscriptionName string
	Attempt          uint8
	CorrelationID    string
	ErrorClass       string

	// Extensões documentadas — usadas apenas nas transições que as requerem.
	LatencyMs     int64
	NextRetryAt   time.Time
	TotalAttempts uint8
}

// Attrs retorna apenas os atributos não-vazios como []slog.Attr.
// Nunca inclui payload, Payload, json.RawMessage ou equivalente (R-SEC-001).
func (f logFields) Attrs() []slog.Attr {
	attrs := make([]slog.Attr, 0, 8)

	if f.EventID != "" {
		attrs = append(attrs, slog.String("event_id", f.EventID))
	}
	if f.EventType != "" {
		attrs = append(attrs, slog.String("event_type", f.EventType))
	}
	if f.SubscriptionName != "" {
		attrs = append(attrs, slog.String("subscription_name", f.SubscriptionName))
	}
	if f.Attempt > 0 {
		attrs = append(attrs, slog.Int("attempt", int(f.Attempt)))
	}
	if f.CorrelationID != "" {
		attrs = append(attrs, slog.String("correlation_id", f.CorrelationID))
	}
	if f.ErrorClass != "" {
		attrs = append(attrs, slog.String("error_class", f.ErrorClass))
	}
	if f.LatencyMs > 0 {
		attrs = append(attrs, slog.Int64("latency_ms", f.LatencyMs))
	}
	if !f.NextRetryAt.IsZero() {
		attrs = append(attrs, slog.String("next_retry_at", f.NextRetryAt.UTC().Format(time.RFC3339)))
	}
	if f.TotalAttempts > 0 {
		attrs = append(attrs, slog.Int("total_attempts", int(f.TotalAttempts)))
	}

	return attrs
}

// logFieldsFromClaim cria logFields a partir de um Claim.
// O campo correlation_id é extraído dos Headers do evento (chave "correlation_id").
func logFieldsFromClaim(claim Claim) logFields {
	correlationID := ""
	if v, ok := claim.Event.Headers().Get("correlation_id"); ok {
		correlationID = v
	}
	return logFields{
		EventID:          claim.Event.ID().String(),
		EventType:        claim.Event.Type().String(),
		SubscriptionName: claim.SubscriptionName.String(),
		Attempt:          claim.Attempt.Value(),
		CorrelationID:    correlationID,
	}
}
