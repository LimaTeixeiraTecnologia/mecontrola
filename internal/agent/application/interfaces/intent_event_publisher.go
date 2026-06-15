package interfaces

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/valueobjects"
)

type IntentEvent struct {
	EventID          uuid.UUID
	UserID           uuid.UUID
	Channel          string
	Outcome          string
	Module           string
	Action           string
	ProviderUsed     valueobjects.ModelSlug
	Reason           string
	ResponseHint     string
	LatencyMS        int64
	PromptTokens     int
	CompletionTokens int
	OccurredAt       time.Time
}

type IntentEventPublisher interface {
	PublishExecuted(ctx context.Context, ev IntentEvent) error
	PublishRejected(ctx context.Context, ev IntentEvent) error
}
