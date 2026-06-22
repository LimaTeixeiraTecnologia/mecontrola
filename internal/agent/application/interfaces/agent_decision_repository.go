package interfaces

import (
	"context"
	"errors"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
)

var (
	ErrAgentDecisionNotFound = errors.New("agent: decision not found")
	ErrAgentDecisionConflict = errors.New("agent: decision already exists for user, channel and message")
)

type AgentDecisionRepository interface {
	Insert(ctx context.Context, decision entities.AgentDecision) error
	UpdateSettlement(ctx context.Context, decision entities.AgentDecision) error
}

type AgentDecisionRepositoryFactory interface {
	AgentDecisionRepository(db database.DBTX) AgentDecisionRepository
}
