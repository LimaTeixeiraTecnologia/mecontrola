package interfaces

import (
	"context"

	"github.com/google/uuid"
)

type BudgetPlanner interface {
	CreateBudget(ctx context.Context, in DraftBudget) (BudgetRef, error)
	DeleteDraftBudget(ctx context.Context, userID uuid.UUID, competence string) error
	ActivateBudget(ctx context.Context, userID uuid.UUID, competence string) error
	CreateRecurrence(ctx context.Context, userID uuid.UUID, competence string, months int) error
	EditCategoryPercentage(ctx context.Context, userID uuid.UUID, competence, rootSlug string, percentage int) error
	GetMonthlySummary(ctx context.Context, userID uuid.UUID, competence string) (BudgetSummary, error)
	ListAlerts(ctx context.Context, userID uuid.UUID) ([]Alert, error)
}
