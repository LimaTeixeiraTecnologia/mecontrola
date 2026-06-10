package usecases

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/database/uow"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type ActivateBudget struct {
	budgets interfaces.BudgetRepository
	uow     uow.UnitOfWork[entities.Budget]
	o11y    observability.Observability
}

func NewActivateBudget(
	budgets interfaces.BudgetRepository,
	u uow.UnitOfWork[entities.Budget],
	o11y observability.Observability,
) *ActivateBudget {
	return &ActivateBudget{budgets: budgets, uow: u, o11y: o11y}
}

func (uc *ActivateBudget) Execute(ctx context.Context, in input.ActivateBudgetInput) (output.BudgetOutput, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "budgets.usecase.activate_budget")
	defer span.End()

	userID, err := uuid.Parse(in.UserID)
	if err != nil {
		return output.BudgetOutput{}, ErrBudgetInvalidUserID
	}

	competence, err := valueobjects.NewCompetence(in.Competence)
	if err != nil {
		return output.BudgetOutput{}, ErrBudgetInvalidCompetence
	}

	now := time.Now().UTC()

	b, execErr := uc.uow.Do(ctx, func(ctx context.Context, tx database.DBTX) (entities.Budget, error) {
		budget, findErr := uc.budgets.GetByUserCompetence(ctx, tx, userID, competence)
		if findErr != nil {
			return entities.Budget{}, findErr
		}

		if budget.IsActive() {
			return entities.Budget{}, entities.ErrBudgetAlreadyActive
		}

		allocInputs := make([]services.AllocationInput, 0, len(budget.Allocations()))
		for _, a := range budget.Allocations() {
			allocInputs = append(allocInputs, services.AllocationInput{
				RootSlug:    a.RootSlug(),
				BasisPoints: a.BasisPoints(),
			})
		}

		distributed := services.Distribute(budget.TotalCents(), allocInputs)

		updatedAllocs := make([]entities.Allocation, 0, len(distributed))
		for _, r := range distributed {
			updatedAllocs = append(updatedAllocs, entities.NewAllocation(budget.ID(), r.RootSlug, r.BasisPoints, r.PlannedCents))
		}
		budget.SetAllocations(updatedAllocs)

		if activateErr := budget.Activate(now); activateErr != nil {
			return entities.Budget{}, activateErr
		}

		if saveErr := uc.budgets.Activate(ctx, tx, budget); saveErr != nil {
			if errors.Is(saveErr, interfaces.ErrBudgetConflict) {
				return entities.Budget{}, interfaces.ErrBudgetConflict
			}
			return entities.Budget{}, fmt.Errorf("budgets.usecase.activate_budget: salvar ativação: %w", saveErr)
		}

		return budget, nil
	})

	if execErr != nil {
		span.RecordError(execErr)
		if errors.Is(execErr, interfaces.ErrBudgetNotFound) ||
			errors.Is(execErr, entities.ErrBudgetAlreadyActive) ||
			errors.Is(execErr, entities.ErrBudgetTotalMustBePositive) ||
			errors.Is(execErr, entities.ErrBudgetAllocationSumMustBe10000) {
			return output.BudgetOutput{}, execErr
		}
		uc.o11y.Logger().Error(ctx, "budgets.usecase.activate_budget.failed",
			observability.String("user_id", in.UserID),
			observability.String("competence", in.Competence),
			observability.Error(execErr),
		)
		return output.BudgetOutput{}, execErr
	}

	return mapBudgetOutput(b), nil
}
