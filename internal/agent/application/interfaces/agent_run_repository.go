package interfaces

import (
	"context"
	"errors"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
)

var ErrAgentRunNotFound = errors.New("agent: run not found")

type AgentRunRepository interface {
	Insert(ctx context.Context, run entities.Run) error
	UpdateOnFinish(ctx context.Context, run entities.Run) error
}

type AgentRunRepositoryFactory interface {
	AgentRunRepository(db database.DBTX) AgentRunRepository
}
