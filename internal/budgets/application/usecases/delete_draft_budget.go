package usecases

import (
	"context"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/database/uow"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type DeleteDraftBudget struct {
	budgets interfaces.BudgetRepository
	uow     uow.UnitOfWork[struct{}]
	o11y    observability.Observability
}

func NewDeleteDraftBudget(
	budgets interfaces.BudgetRepository,
	u uow.UnitOfWork[struct{}],
	o11y observability.Observability,
) *DeleteDraftBudget {
	return &DeleteDraftBudget{budgets: budgets, uow: u, o11y: o11y}
}

func (uc *DeleteDraftBudget) Execute(ctx context.Context, in input.DeleteDraftInput) error {
	ctx, span := uc.o11y.Tracer().Start(ctx, "budgets.usecase.delete_draft_budget")
	defer span.End()

	userID, err := uuid.Parse(in.UserID)
	if err != nil {
		return ErrBudgetInvalidUserID
	}

	competence, err := valueobjects.NewCompetence(in.Competence)
	if err != nil {
		return ErrBudgetInvalidCompetence
	}

	_, execErr := uc.uow.Do(ctx, func(ctx context.Context, tx database.DBTX) (struct{}, error) {
		budget, findErr := uc.budgets.GetByUserCompetence(ctx, tx, userID, competence)
		if findErr != nil {
			return struct{}{}, findErr
		}

		if budget.IsActive() {
			return struct{}{}, entities.ErrBudgetAlreadyActive
		}

		if deleteErr := uc.budgets.DeleteDraft(ctx, tx, userID, competence); deleteErr != nil {
			return struct{}{}, fmt.Errorf("budgets.usecase.delete_draft_budget: excluir rascunho: %w", deleteErr)
		}

		return struct{}{}, nil
	})

	if execErr != nil {
		span.RecordError(execErr)
		uc.o11y.Logger().Error(ctx, "budgets.usecase.delete_draft_budget.failed",
			observability.String("user_id", in.UserID),
			observability.String("competence", in.Competence),
			observability.Error(execErr),
		)
		return execErr
	}

	return nil
}
