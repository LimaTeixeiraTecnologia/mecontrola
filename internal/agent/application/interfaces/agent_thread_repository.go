package interfaces

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
)

var ErrAgentThreadNotFound = errors.New("agent: thread not found")

type AgentThreadRepository interface {
	GetByUserAndChannel(ctx context.Context, userID uuid.UUID, channel string) (entities.Thread, bool, error)
	Upsert(ctx context.Context, thread entities.Thread) (entities.Thread, error)
}

type AgentThreadRepositoryFactory interface {
	AgentThreadRepository(db database.DBTX) AgentThreadRepository
}
