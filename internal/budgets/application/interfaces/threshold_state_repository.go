package interfaces

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type ThresholdStateRepository interface {
	UpsertIfTransition(ctx context.Context, k entities.ThresholdKey, nowCrossed bool, committedAt time.Time) (transitioned bool, err error)
	GetCurrentlyCrossed(ctx context.Context, userID uuid.UUID, competence valueobjects.Competence, rootSlug valueobjects.RootSlug) (map[valueobjects.Threshold]bool, error)
}
