package interfaces

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/entities"
)

type RecurringTemplateRepository interface {
	Create(ctx context.Context, t *entities.RecurringTemplate) error
	UpdateWithVersion(ctx context.Context, t *entities.RecurringTemplate, expectedVersion int64) error
	SoftDelete(ctx context.Context, id, userID uuid.UUID, expectedVersion int64, now time.Time) error
	GetByID(ctx context.Context, id, userID uuid.UUID) (*entities.RecurringTemplate, error)
	List(ctx context.Context, userID uuid.UUID, activeOnly bool, cursor Cursor, limit int) ([]*entities.RecurringTemplate, Cursor, error)
	FindActiveByDayOfMonth(ctx context.Context, day int, asOf time.Time, cursor Cursor, batchSize int) ([]*entities.RecurringTemplate, Cursor, error)
}
