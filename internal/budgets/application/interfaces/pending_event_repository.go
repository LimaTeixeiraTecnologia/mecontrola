package interfaces

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
)

var ErrPendingEventNotFound = errors.New("budgets: evento pendente não encontrado")

var ErrPendingEventDuplicate = errors.New("budgets: evento pendente duplicado (event_id já existe)")

type PendingEventRepository interface {
	Insert(ctx context.Context, p entities.PendingEvent) error
	ListReady(ctx context.Context, limit int) ([]entities.PendingEvent, error)
	Transition(ctx context.Context, id, userID uuid.UUID, to entities.PendingState, reason string) error
}
