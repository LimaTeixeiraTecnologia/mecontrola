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

type EditCategoryPercentage struct {
	factory interfaces.RepositoryFactory
	uow     uow.UnitOfWork
	o11y    observability.Observability
}

func NewEditCategoryPercentage(
	factory interfaces.RepositoryFactory,
	u uow.UnitOfWork,
	o11y observability.Observability,
) *EditCategoryPercentage {
	return &EditCategoryPercentage{factory: factory, uow: u, o11y: o11y}
}

func (uc *EditCategoryPercentage) Execute(ctx context.Context, in input.EditCategoryPercentageInput) (output.BudgetOutput, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "budgets.usecase.edit_category_percentage")
	defer span.End()

	if err := in.Validate(); err != nil {
		return output.BudgetOutput{}, err
	}

	cmd, err := commands.NewEditCategoryPercentageCommand(in.UserID, in.Competence, in.RootSlug, in.Percentage)
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
			errors.Is(execErr, entities.ErrBudgetAllocationSumMustBe10000) ||
			errors.Is(execErr, services.ErrCategoryPercentageTargetNotFound) ||
			errors.Is(execErr, services.ErrCategoryPercentageNoAllocations) ||
			errors.Is(execErr, services.ErrCategoryPercentageSumInvalid) ||
			errors.Is(execErr, interfaces.ErrBudgetConflict) {
			return output.BudgetOutput{}, execErr
		}
		uc.logFailure(ctx, in, execErr)
		return output.BudgetOutput{}, execErr
	}

	return mappers.M.Budget(budget), nil
}

func (uc *EditCategoryPercentage) persist(ctx context.Context, tx database.DBTX, cmd commands.EditCategoryPercentageCommand) (entities.Budget, error) {
	budgets := uc.factory.BudgetRepository(tx)
	budget, err := budgets.GetByUserCompetence(ctx, cmd.UserID, cmd.Competence)
	if err != nil {
		return entities.Budget{}, err
	}

	if !budget.IsActive() {
		return entities.Budget{}, entities.ErrBudgetNotActive
	}

	current := make([]services.CategoryPercentageInput, 0, len(budget.Allocations()))
	for _, a := range budget.Allocations() {
		current = append(current, services.CategoryPercentageInput{
			RootSlug:    a.RootSlug(),
			BasisPoints: a.BasisPoints(),
		})
	}

	rebalanced, decideErr := services.DecideEditCategoryPercentage(current, cmd.RootSlug, cmd.BasisPoints)
	if decideErr != nil {
		return entities.Budget{}, decideErr
	}

	allocInputs := make([]services.AllocationInput, 0, len(rebalanced))
	for _, r := range rebalanced {
		allocInputs = append(allocInputs, services.AllocationInput(r))
	}

	distributed := services.AllocationDistributor{}.Distribute(budget.TotalCents(), allocInputs)
	updatedAllocs := make([]entities.Allocation, 0, len(distributed))
	for _, r := range distributed {
		updatedAllocs = append(updatedAllocs, entities.NewAllocation(budget.ID(), r.RootSlug, r.BasisPoints, r.PlannedCents))
	}

	if rebalanceErr := budget.RebalanceAllocations(updatedAllocs, time.Now().UTC()); rebalanceErr != nil {
		return entities.Budget{}, rebalanceErr
	}

	if saveErr := budgets.Activate(ctx, budget); saveErr != nil {
		if errors.Is(saveErr, interfaces.ErrBudgetConflict) {
			return entities.Budget{}, interfaces.ErrBudgetConflict
		}
		return entities.Budget{}, fmt.Errorf("budgets.usecase.edit_category_percentage: salvar rebalanceamento: %w", saveErr)
	}

	return budget, nil
}

func (uc *EditCategoryPercentage) logFailure(ctx context.Context, in input.EditCategoryPercentageInput, err error) {
	uc.o11y.Logger().Error(ctx, "budgets.usecase.edit_category_percentage.failed",
		observability.String("user_id", in.UserID),
		observability.String("competence", in.Competence),
		observability.String("root_slug", in.RootSlug),
		observability.Error(err),
	)
}
