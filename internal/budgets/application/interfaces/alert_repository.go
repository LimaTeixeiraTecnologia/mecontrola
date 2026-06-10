package interfaces

import (
	"context"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
)

type AlertRepository interface {
	Insert(ctx context.Context, db database.DBTX, a entities.Alert) error
	CountDelivered(ctx context.Context, db database.DBTX, k entities.ThresholdKey) (int64, error)
	ListForUser(ctx context.Context, db database.DBTX, userID uuid.UUID, q input.AlertQuery) ([]entities.Alert, string, error)
	PurgeOld(ctx context.Context, db database.DBTX, olderThan string, limit int) (int64, error)
}
