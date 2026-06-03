package outbox

import "time"

// Stats agrega as métricas de gauge do Outbox usadas para instrumentação OTel
// (outbox.deliveries.pending, outbox.deliveries.dead_letter).
// Produzida por Storage.Stats e consumida pelo loop de coleta de métricas do Subsystem.
type Stats struct {
	// Pending mapeia subscription_name → contagem de deliveries pendentes.
	Pending map[SubscriptionName]int64
	// DeadLetter mapeia subscription_name → contagem de deliveries em DLQ.
	DeadLetter map[SubscriptionName]int64
	// OldestPendingAt é o OccurredAt do evento pendente mais antigo,
	// ou zero-value se não houver pendentes.
	OldestPendingAt time.Time
}
