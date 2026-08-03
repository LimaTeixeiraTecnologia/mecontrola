package usecases

import (
	"context"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/commands"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/services"
)

type SyncFutureBudgets struct {
	factory   interfaces.RepositoryFactory
	uow       uow.UnitOfWork
	o11y      observability.Observability
	validator *services.RecurrenceSourceValidator
	cloner    *services.BudgetClonerForRecurrence
}

func NewSyncFutureBudgets(
	factory interfaces.RepositoryFactory,
	u uow.UnitOfWork,
	o11y observability.Observability,
) *SyncFutureBudgets {
	validator := services.NewRecurrenceSourceValidator()
	return &SyncFutureBudgets{
		factory:   factory,
		uow:       u,
		o11y:      o11y,
		validator: validator,
		cloner:    services.NewBudgetClonerForRecurrence(validator),
	}
}

func (uc *SyncFutureBudgets) Execute(ctx context.Context, in input.SyncFutureBudgetsInput) (output.SyncFutureBudgetsOutput, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "budgets.usecase.sync_future_budgets")
	defer span.End()

	if err := in.Validate(); err != nil {
		return output.SyncFutureBudgetsOutput{}, err
	}

	cmd, err := commands.NewSyncFutureBudgetsCommand(in.UserID, in.SourceCompetence)
	if err != nil {
		return output.SyncFutureBudgetsOutput{}, err
	}

	result, execErr := uow.Do(ctx, uc.uow, func(ctx context.Context, tx database.DBTX) (output.SyncFutureBudgetsOutput, error) {
		budgets := uc.factory.BudgetRepository(tx)

		source, sourceErr := budgets.GetActiveByUserCompetence(ctx, cmd.UserID, cmd.SourceCompetence)
		if sourceErr != nil {
			return output.SyncFutureBudgetsOutput{}, sourceErr
		}
		if validateErr := uc.validator.Validate(source); validateErr != nil {
			return output.SyncFutureBudgetsOutput{}, validateErr
		}

		future, listErr := budgets.ListFutureByUserCompetence(ctx, cmd.UserID, cmd.SourceCompetence)
		if listErr != nil {
			return output.SyncFutureBudgetsOutput{}, fmt.Errorf("budgets.usecase.sync_future_budgets: listar futuros: %w", listErr)
		}

		result := output.SyncFutureBudgetsOutput{
			UpdatedCompetences:       make([]string, 0, len(future)),
			SkippedActiveCompetences: make([]string, 0, len(future)),
		}
		for _, item := range future {
			if item.IsActive() {
				result.SkippedActiveCompetences = append(result.SkippedActiveCompetences, item.Competence().String())
				continue
			}

			updatedAllocs := uc.cloner.Rebase(item, source)
			updated := entities.HydrateBudget(
				item.ID(),
				item.UserID(),
				item.Competence(),
				source.TotalCents(),
				entities.BudgetStateDraft,
				nil,
				item.AutoDraft(),
				updatedAllocs,
				item.CreatedAt(),
				time.Now().UTC(),
			)

			if saveErr := budgets.Activate(ctx, updated); saveErr != nil {
				return output.SyncFutureBudgetsOutput{}, fmt.Errorf(
					"budgets.usecase.sync_future_budgets: salvar competência %s: %w",
					item.Competence().String(),
					saveErr,
				)
			}
			result.UpdatedCompetences = append(result.UpdatedCompetences, item.Competence().String())
		}

		return result, nil
	})
	if execErr != nil {
		span.RecordError(execErr)
		return output.SyncFutureBudgetsOutput{}, execErr
	}

	return result, nil
}
