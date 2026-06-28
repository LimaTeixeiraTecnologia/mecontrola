package interfaces

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
)

var ErrProcessedEventAlreadyExists = errors.New("agent: processed event already exists")

type ProcessedEventRepository interface {
	IsProcessed(ctx context.Context, eventID uuid.UUID) (bool, error)
	MarkProcessed(ctx context.Context, eventID uuid.UUID, eventType string, userID uuid.UUID, processedAt time.Time) error
}

type ProcessedEventRepositoryFactory interface {
	ProcessedEventRepository(db database.DBTX) ProcessedEventRepository
}
