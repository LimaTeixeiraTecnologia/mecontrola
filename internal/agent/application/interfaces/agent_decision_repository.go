package interfaces

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
)

var (
	ErrAgentDecisionNotFound = errors.New("agent: decision not found")
	ErrAgentDecisionConflict = errors.New("agent: decision already exists for user, channel and message")
)

type AgentDecisionSnapshot struct {
	Status           string
	RedactedResponse json.RawMessage
}

type AgentDecisionRepository interface {
	Insert(ctx context.Context, decision entities.AgentDecision) error
	UpdateSettlement(ctx context.Context, decision entities.AgentDecision) error
	FindByMessage(ctx context.Context, userID uuid.UUID, channel, messageID string, stepIndex int) (AgentDecisionSnapshot, bool, error)
}

type AgentDecisionRepositoryFactory interface {
	AgentDecisionRepository(db database.DBTX) AgentDecisionRepository
}
