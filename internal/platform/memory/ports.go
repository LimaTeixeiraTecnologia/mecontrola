package memory

import (
	"context"

	"github.com/google/uuid"
)

type ThreadGateway interface {
	GetOrCreate(ctx context.Context, resourceID, threadID string) (Thread, error)
}

type MessageStore interface {
	Append(ctx context.Context, threadPK uuid.UUID, m Message) error
	Recent(ctx context.Context, threadPK uuid.UUID, limit int) ([]Message, error)
}

type WorkingMemory interface {
	Get(ctx context.Context, resourceID string) (string, error)
	Upsert(ctx context.Context, resourceID, content string) error
}

type SemanticRecall interface {
	Index(ctx context.Context, resourceID, threadID string, sourceMessageID uuid.UUID, content, model string, embedding []float32) error
	Recall(ctx context.Context, resourceID, query string, embedding []float32, k int) ([]RecallHit, error)
}

type Summarizer interface {
	Summarize(ctx context.Context, messages []Message) (string, error)
}

type MessageIndexPublisher interface {
	PublishIndex(ctx context.Context, p IndexMessagePayload) error
}
