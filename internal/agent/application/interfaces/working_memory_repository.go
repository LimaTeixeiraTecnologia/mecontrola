package interfaces

import (
	"context"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
)

type WorkingMemoryRepository interface {
	Get(ctx context.Context, userID uuid.UUID) (entities.WorkingMemory, bool, error)
	Upsert(ctx context.Context, wm entities.WorkingMemory) error
}

type WorkingMemoryRepositoryFactory interface {
	WorkingMemoryRepository(db database.DBTX) WorkingMemoryRepository
}
