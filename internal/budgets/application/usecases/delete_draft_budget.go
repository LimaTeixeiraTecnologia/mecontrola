package usecases

import (
	"context"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/commands"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
)

type DeleteDraftBudget struct {
	factory interfaces.RepositoryFactory
	uow     uow.UnitOfWork
	o11y    observability.Observability
}

func NewDeleteDraftBudget(
	factory interfaces.RepositoryFactory,
	u uow.UnitOfWork,
	o11y observability.Observability,
) *DeleteDraftBudget {
	return &DeleteDraftBudget{factory: factory, uow: u, o11y: o11y}
}

func (uc *DeleteDraftBudget) Execute(ctx context.Context, in input.DeleteDraftInput) error {
	ctx, span := uc.o11y.Tracer().Start(ctx, "budgets.usecase.delete_draft_budget")
	defer span.End()

	cmd, err := commands.NewDeleteDraftBudgetCommand(in.UserID, in.Competence)
	if err != nil {
		return err
	}

	_, execErr := uow.Do(ctx, uc.uow, func(ctx context.Context, tx database.DBTX) (struct{}, error) {
		return uc.persist(ctx, tx, cmd)
	})
	if execErr != nil {
		span.RecordError(execErr)
		uc.logFailure(ctx, in, execErr)
		return execErr
	}

	return nil
}

func (uc *DeleteDraftBudget) persist(ctx context.Context, tx database.DBTX, cmd commands.DeleteDraftBudgetCommand) (struct{}, error) {
	budgets := uc.factory.BudgetRepository(tx)
	budget, err := budgets.GetByUserCompetence(ctx, cmd.UserID, cmd.Competence)
	if err != nil {
		return struct{}{}, err
	}

	if budget.IsActive() {
		return struct{}{}, entities.ErrBudgetAlreadyActive
	}

	if deleteErr := budgets.DeleteDraft(ctx, cmd.UserID, cmd.Competence); deleteErr != nil {
		return struct{}{}, fmt.Errorf("budgets.usecase.delete_draft_budget: excluir rascunho: %w", deleteErr)
	}

	return struct{}{}, nil
}

func (uc *DeleteDraftBudget) logFailure(ctx context.Context, in input.DeleteDraftInput, err error) {
	uc.o11y.Logger().Error(ctx, "budgets.usecase.delete_draft_budget.failed",
		observability.String("user_id", in.UserID),
		observability.String("competence", in.Competence),
		observability.Error(err),
	)
}
