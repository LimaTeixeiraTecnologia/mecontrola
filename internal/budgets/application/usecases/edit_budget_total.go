package usecases

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/mappers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/commands"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/services"
)

type EditBudgetTotal struct {
	factory interfaces.RepositoryFactory
	uow     uow.UnitOfWork
	o11y    observability.Observability
}

func NewEditBudgetTotal(
	factory interfaces.RepositoryFactory,
	u uow.UnitOfWork,
	o11y observability.Observability,
) *EditBudgetTotal {
	return &EditBudgetTotal{factory: factory, uow: u, o11y: o11y}
}

func (uc *EditBudgetTotal) Execute(ctx context.Context, in input.EditBudgetTotalInput) (output.BudgetOutput, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "budgets.usecase.edit_budget_total")
	defer span.End()

	if err := in.Validate(); err != nil {
		return output.BudgetOutput{}, err
	}

	cmd, err := commands.NewEditBudgetTotalCommand(in.UserID, in.Competence, in.TotalCents)
	if err != nil {
		return output.BudgetOutput{}, err
	}

	budget, execErr := uow.Do(ctx, uc.uow, func(ctx context.Context, tx database.DBTX) (entities.Budget, error) {
		return uc.persist(ctx, tx, cmd)
	})
	if execErr != nil {
		span.RecordError(execErr)
		if errors.Is(execErr, interfaces.ErrBudgetNotFound) ||
			errors.Is(execErr, entities.ErrBudgetNotActive) ||
			errors.Is(execErr, entities.ErrBudgetTotalMustBePositive) ||
			errors.Is(execErr, entities.ErrBudgetAllocationSumMustBe10000) ||
			errors.Is(execErr, interfaces.ErrBudgetConflict) {
			return output.BudgetOutput{}, execErr
		}
		uc.logFailure(ctx, in, execErr)
		return output.BudgetOutput{}, execErr
	}

	return mappers.M.Budget(budget), nil
}

func (uc *EditBudgetTotal) persist(ctx context.Context, tx database.DBTX, cmd commands.EditBudgetTotalCommand) (entities.Budget, error) {
	budgets := uc.factory.BudgetRepository(tx)
	budget, err := budgets.GetByUserCompetence(ctx, cmd.UserID, cmd.Competence)
	if err != nil {
		return entities.Budget{}, err
	}

	if !budget.IsActive() {
		return entities.Budget{}, entities.ErrBudgetNotActive
	}

	current := make([]services.AllocationInput, 0, len(budget.Allocations()))
	for _, a := range budget.Allocations() {
		current = append(current, services.AllocationInput{
			RootSlug:    a.RootSlug(),
			BasisPoints: a.BasisPoints(),
		})
	}

	distributed := services.AllocationDistributor{}.Distribute(cmd.TotalCents, current)
	updatedAllocs := make([]entities.Allocation, 0, len(distributed))
	for _, r := range distributed {
		updatedAllocs = append(updatedAllocs, entities.NewAllocation(budget.ID(), r.RootSlug, r.BasisPoints, r.PlannedCents))
	}

	if changeErr := budget.ChangeTotal(cmd.TotalCents, updatedAllocs, time.Now().UTC()); changeErr != nil {
		return entities.Budget{}, changeErr
	}

	if saveErr := budgets.Activate(ctx, budget); saveErr != nil {
		if errors.Is(saveErr, interfaces.ErrBudgetConflict) {
			return entities.Budget{}, interfaces.ErrBudgetConflict
		}
		return entities.Budget{}, fmt.Errorf("budgets.usecase.edit_budget_total: salvar novo total: %w", saveErr)
	}

	return budget, nil
}

func (uc *EditBudgetTotal) logFailure(ctx context.Context, in input.EditBudgetTotalInput, err error) {
	uc.o11y.Logger().Error(ctx, "budgets.usecase.edit_budget_total.failed",
		observability.String("user_id", in.UserID),
		observability.String("competence", in.Competence),
		observability.Error(err),
	)
}
