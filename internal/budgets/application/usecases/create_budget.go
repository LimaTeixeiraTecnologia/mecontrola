package usecases

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/database/uow"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/mappers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/commands"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
)

type CreateBudget struct {
	factory interfaces.RepositoryFactory
	uow     uow.UnitOfWork[entities.Budget]
	o11y    observability.Observability
}

func NewCreateBudget(
	factory interfaces.RepositoryFactory,
	u uow.UnitOfWork[entities.Budget],
	o11y observability.Observability,
) *CreateBudget {
	return &CreateBudget{factory: factory, uow: u, o11y: o11y}
}

func (uc *CreateBudget) Execute(ctx context.Context, in input.CreateBudgetInput) (output.BudgetOutput, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "budgets.usecase.create_budget")
	defer span.End()

	cmd, err := commands.NewCreateBudgetCommand(in.UserID, in.Competence, in.TotalCents, convertAllocations(in.Allocations))
	if err != nil {
		return output.BudgetOutput{}, err
	}

	budget, execErr := uc.uow.Do(ctx, func(ctx context.Context, tx database.DBTX) (entities.Budget, error) {
		return uc.persist(ctx, tx, cmd)
	})
	if execErr != nil {
		span.RecordError(execErr)
		if errors.Is(execErr, interfaces.ErrBudgetConflict) {
			return output.BudgetOutput{}, execErr
		}
		uc.logFailure(ctx, in, execErr)
		return output.BudgetOutput{}, execErr
	}

	return mappers.M.Budget(budget), nil
}

func (uc *CreateBudget) persist(ctx context.Context, tx database.DBTX, cmd commands.CreateBudgetCommand) (entities.Budget, error) {
	now := time.Now().UTC()
	budgets := uc.factory.BudgetRepository(tx)
	budget := entities.NewBudget(cmd.UserID, cmd.Competence, cmd.TotalCents, now)

	for _, a := range cmd.Allocations {
		budget.AddAllocation(entities.NewAllocation(budget.ID(), a.RootSlug, a.BasisPoints.Int(), 0))
	}

	if err := budgets.CreateDraft(ctx, budget); err != nil {
		if errors.Is(err, interfaces.ErrBudgetConflict) {
			return entities.Budget{}, interfaces.ErrBudgetConflict
		}
		return entities.Budget{}, fmt.Errorf("budgets.usecase.create_budget: criar rascunho: %w", err)
	}
	return budget, nil
}

func (uc *CreateBudget) logFailure(ctx context.Context, in input.CreateBudgetInput, err error) {
	uc.o11y.Logger().Error(ctx, "budgets.usecase.create_budget.failed",
		observability.String("user_id", in.UserID),
		observability.String("competence", in.Competence),
		observability.Error(err),
	)
}

func convertAllocations(ins []input.AllocationInput) []commands.AllocationCommandInput {
	out := make([]commands.AllocationCommandInput, 0, len(ins))
	for _, a := range ins {
		out = append(out, commands.AllocationCommandInput{
			RootSlug:    a.RootSlug,
			BasisPoints: a.BasisPoints,
		})
	}
	return out
}
