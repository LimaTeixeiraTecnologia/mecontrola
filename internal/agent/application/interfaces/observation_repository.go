package interfaces

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
)

type ObservationRepository interface {
	Insert(ctx context.Context, obs entities.Observation) error
	ListRecent(ctx context.Context, userID uuid.UUID, channel string, limit int) ([]entities.Observation, error)
	DeleteExpired(ctx context.Context, before time.Time) (int64, error)
	DeleteOldestBeyondLimit(ctx context.Context, userID uuid.UUID, channel string, keep int) error
}

type ObservationRepositoryFactory interface {
	ObservationRepository(db database.DBTX) ObservationRepository
}
