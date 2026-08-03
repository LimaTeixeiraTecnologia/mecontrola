package usecases

import (
	"context"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/commands"
)

type ListFutureBudgets struct {
	factory interfaces.RepositoryFactory
	uow     uow.UnitOfWork
	o11y    observability.Observability
}

func NewListFutureBudgets(
	factory interfaces.RepositoryFactory,
	u uow.UnitOfWork,
	o11y observability.Observability,
) *ListFutureBudgets {
	return &ListFutureBudgets{factory: factory, uow: u, o11y: o11y}
}

func (uc *ListFutureBudgets) Execute(ctx context.Context, userID string, competence string) (output.ListFutureBudgetsOutput, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "budgets.usecase.list_future_budgets")
	defer span.End()

	cmd, err := commands.NewGetMonthlySummaryCommand(userID, competence)
	if err != nil {
		span.RecordError(err)
		return output.ListFutureBudgetsOutput{}, err
	}

	result, execErr := uow.Do(ctx, uc.uow, func(ctx context.Context, tx database.DBTX) (output.ListFutureBudgetsOutput, error) {
		budgets := uc.factory.BudgetRepository(tx)
		items, listErr := budgets.ListFutureByUserCompetence(ctx, cmd.UserID, cmd.Competence)
		if listErr != nil {
			return output.ListFutureBudgetsOutput{}, fmt.Errorf("budgets.usecase.list_future_budgets: listar futuros: %w", listErr)
		}

		out := output.ListFutureBudgetsOutput{Budgets: make([]output.FutureBudgetOutput, 0, len(items))}
		for _, item := range items {
			state := "draft"
			if item.IsActive() {
				state = "active"
			}
			out.Budgets = append(out.Budgets, output.FutureBudgetOutput{
				Competence: item.Competence().String(),
				State:      state,
			})
		}
		return out, nil
	})
	if execErr != nil {
		span.RecordError(execErr)
		return output.ListFutureBudgetsOutput{}, execErr
	}
	return result, nil
}
