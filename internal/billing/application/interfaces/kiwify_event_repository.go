package interfaces

import (
	"context"
	"time"
)

type KiwifyEventRepository interface {
	Persist(ctx context.Context, envelopeID string, trigger string, rawBody []byte, signatureStatus string) error
	MarkProcessed(ctx context.Context, envelopeID string, processedAt time.Time) error
}
