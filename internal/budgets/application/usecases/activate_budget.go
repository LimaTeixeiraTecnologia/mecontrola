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

type ActivateBudget struct {
	factory   interfaces.RepositoryFactory
	publisher interfaces.BudgetActivatedPublisher
	uow       uow.UnitOfWork
	o11y      observability.Observability
}

func NewActivateBudget(
	factory interfaces.RepositoryFactory,
	publisher interfaces.BudgetActivatedPublisher,
	u uow.UnitOfWork,
	o11y observability.Observability,
) *ActivateBudget {
	return &ActivateBudget{factory: factory, publisher: publisher, uow: u, o11y: o11y}
}

func (uc *ActivateBudget) Execute(ctx context.Context, in input.ActivateBudgetInput) (output.BudgetOutput, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "budgets.usecase.activate_budget")
	defer span.End()

	if err := in.Validate(); err != nil {
		return output.BudgetOutput{}, err
	}

	cmd, err := commands.NewActivateBudgetCommand(in.UserID, in.Competence)
	if err != nil {
		return output.BudgetOutput{}, err
	}

	budget, execErr := uow.Do(ctx, uc.uow, func(ctx context.Context, tx database.DBTX) (entities.Budget, error) {
		return uc.persist(ctx, tx, cmd)
	})
	if execErr != nil {
		span.RecordError(execErr)
		if errors.Is(execErr, interfaces.ErrBudgetNotFound) ||
			errors.Is(execErr, entities.ErrBudgetAlreadyActive) ||
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

func (uc *ActivateBudget) persist(ctx context.Context, tx database.DBTX, cmd commands.ActivateBudgetCommand) (entities.Budget, error) {
	budgets := uc.factory.BudgetRepository(tx)
	budget, err := budgets.GetByUserCompetence(ctx, cmd.UserID, cmd.Competence)
	if err != nil {
		return entities.Budget{}, err
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

	distributed := services.AllocationDistributor{}.Distribute(budget.TotalCents(), allocInputs)
	updatedAllocs := make([]entities.Allocation, 0, len(distributed))
	for _, r := range distributed {
		updatedAllocs = append(updatedAllocs, entities.NewAllocation(budget.ID(), r.RootSlug, r.BasisPoints, r.PlannedCents))
	}
	budget.SetAllocations(updatedAllocs)

	if activateErr := budget.Activate(time.Now().UTC()); activateErr != nil {
		return entities.Budget{}, activateErr
	}

	if saveErr := budgets.Activate(ctx, budget); saveErr != nil {
		if errors.Is(saveErr, interfaces.ErrBudgetConflict) {
			return entities.Budget{}, interfaces.ErrBudgetConflict
		}
		return entities.Budget{}, fmt.Errorf("budgets.usecase.activate_budget: salvar ativação: %w", saveErr)
	}

	occurredAt := budget.UpdatedAt()
	if budget.ActivatedAt() != nil {
		occurredAt = budget.ActivatedAt().UTC()
	}
	if publishErr := uc.publisher.Publish(ctx, tx, budget, occurredAt); publishErr != nil {
		return entities.Budget{}, fmt.Errorf("budgets.usecase.activate_budget: publicar budget_activated: %w", publishErr)
	}

	return budget, nil
}

func (uc *ActivateBudget) logFailure(ctx context.Context, in input.ActivateBudgetInput, err error) {
	uc.o11y.Logger().Error(ctx, "budgets.usecase.activate_budget.failed",
		observability.String("user_id", in.UserID),
		observability.String("competence", in.Competence),
		observability.Error(err),
	)
}
