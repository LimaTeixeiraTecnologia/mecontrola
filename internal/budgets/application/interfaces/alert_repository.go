package interfaces

import (
	"context"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
)

type AlertRepository interface {
	Insert(ctx context.Context, a entities.Alert) error
	CountDelivered(ctx context.Context, k entities.ThresholdKey) (int64, error)
	ListForUser(ctx context.Context, userID uuid.UUID, q input.AlertQuery) ([]entities.Alert, string, error)
	PurgeOld(ctx context.Context, olderThan string, limit int) (int64, error)
}
