package interfaces

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
)

var (
	ErrAgentSessionNotFound = errors.New("agent: session not found")
	ErrAgentSessionConflict = errors.New("agent: session already exists for user and channel")
)

type AgentSessionRecord struct {
	ID            uuid.UUID
	UserID        uuid.UUID
	Channel       string
	PendingAction []byte
	RecentTurns   []byte
	CreatedAt     time.Time
	UpdatedAt     time.Time
	ExpiresAt     time.Time
}

type AgentSessionRepository interface {
	Create(ctx context.Context, record AgentSessionRecord) error
	GetByUserAndChannel(ctx context.Context, userID uuid.UUID, channel string) (AgentSessionRecord, error)
	Update(ctx context.Context, record AgentSessionRecord) error
	DeleteExpired(ctx context.Context, before time.Time) (int64, error)
}

type AgentSessionRepositoryFactory interface {
	AgentSessionRepository(db database.DBTX) AgentSessionRepository
}
