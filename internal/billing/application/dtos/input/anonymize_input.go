package input

import "time"

// AnonymizeInput parametriza o job diário de anonimização de webhook_events (RF-49).
type AnonymizeInput struct {
	OlderThan time.Time
	BatchSize int
}
