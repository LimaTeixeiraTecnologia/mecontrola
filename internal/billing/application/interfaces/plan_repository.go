package interfaces

import (
	"context"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/domain/valueobjects"
)

type PlanRepository interface {
	FindByKiwifyProductID(ctx context.Context, kiwifyProductID string) (valueobjects.Plan, error)
	FindByCode(ctx context.Context, code valueobjects.PlanCode) (valueobjects.Plan, error)
	ConfigureProductIDs(ctx context.Context, productIDs map[valueobjects.PlanCode]string) error
}
