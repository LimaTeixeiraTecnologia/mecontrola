package interfaces

import (
	"context"
	"errors"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
)

var ErrPendingEventNotFound = errors.New("budgets: evento pendente não encontrado")

var ErrPendingEventDuplicate = errors.New("budgets: evento pendente duplicado (event_id já existe)")

type PendingEventRepository interface {
	Insert(ctx context.Context, db database.DBTX, p entities.PendingEvent) error
	ListReady(ctx context.Context, db database.DBTX, limit int) ([]entities.PendingEvent, error)
	Transition(ctx context.Context, db database.DBTX, id uuid.UUID, to entities.PendingState, reason string) error
}
