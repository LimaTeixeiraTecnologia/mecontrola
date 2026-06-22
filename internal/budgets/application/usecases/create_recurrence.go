package usecases

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/uow"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/commands"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type CreateRecurrence struct {
	factory   interfaces.RepositoryFactory
	uow       uow.UnitOfWork
	o11y      observability.Observability
	validator *services.RecurrenceSourceValidator
	cloner    *services.BudgetClonerForRecurrence
}

func NewCreateRecurrence(
	factory interfaces.RepositoryFactory,
	u uow.UnitOfWork,
	o11y observability.Observability,
) *CreateRecurrence {
	validator := services.NewRecurrenceSourceValidator()
	return &CreateRecurrence{
		factory:   factory,
		uow:       u,
		o11y:      o11y,
		validator: validator,
		cloner:    services.NewBudgetClonerForRecurrence(validator),
	}
}

func (uc *CreateRecurrence) Execute(ctx context.Context, in input.CreateRecurrenceInput) (output.RecurrenceResultOutput, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "budgets.usecase.create_recurrence")
	defer span.End()

	if err := in.Validate(); err != nil {
		return output.RecurrenceResultOutput{}, err
	}

	cmd, err := commands.NewCreateRecurrenceCommand(in.UserID, in.SourceCompetence, in.Months)
	if err != nil {
		return output.RecurrenceResultOutput{}, err
	}

	sourceBudget, sourceErr := uc.loadAndValidateSource(ctx, cmd)
	if sourceErr != nil {
		return output.RecurrenceResultOutput{}, sourceErr
	}

	results := make([]output.RecurrenceResultEntry, 0, cmd.Months)
	current := cmd.SourceCompetence
	for range cmd.Months {
		current = current.Next()
		results = append(results, uc.processMonth(ctx, cmd.UserID, current, sourceBudget))
	}

	return output.RecurrenceResultOutput{
		SourceCompetence: in.SourceCompetence,
		Results:          results,
	}, nil
}

func (uc *CreateRecurrence) loadAndValidateSource(ctx context.Context, cmd commands.CreateRecurrenceCommand) (entities.Budget, error) {
	var sourceBudget entities.Budget
	var validationErr error

	_, txErr := uow.Do(ctx, uc.uow, func(ctx context.Context, tx database.DBTX) (struct{}, error) {
		budgets := uc.factory.BudgetRepository(tx)
		b, findErr := budgets.GetByUserCompetence(ctx, cmd.UserID, cmd.SourceCompetence)
		if findErr != nil {
			validationErr = ErrRecurrenceSourceInvalid
			return struct{}{}, findErr
		}

		if validateErr := uc.validator.Validate(b); validateErr != nil {
			validationErr = validateErr
			return struct{}{}, validateErr
		}

		sourceBudget = b
		return struct{}{}, nil
	})

	if txErr != nil {
		if validationErr != nil {
			return entities.Budget{}, validationErr
		}
		return entities.Budget{}, txErr
	}
	return sourceBudget, nil
}

func (uc *CreateRecurrence) processMonth(
	ctx context.Context,
	userID uuid.UUID,
	comp valueobjects.Competence,
	source entities.Budget,
) output.RecurrenceResultEntry {
	var status output.RecurrenceStatus
	var reason string

	_, err := uow.Do(ctx, uc.uow, func(ctx context.Context, tx database.DBTX) (struct{}, error) {
		budgets := uc.factory.BudgetRepository(tx)
		existing, findErr := budgets.GetByUserCompetence(ctx, userID, comp)
		if findErr != nil && !errors.Is(findErr, interfaces.ErrBudgetNotFound) {
			return struct{}{}, fmt.Errorf("budgets.usecase.create_recurrence: buscar competência %s: %w", comp, findErr)
		}

		if findErr == nil {
			s, r, updateErr := uc.upgradeExisting(ctx, budgets, existing, source, comp)
			status = s
			reason = r
			return struct{}{}, updateErr
		}

		s, r, createErr := uc.createFromSource(ctx, budgets, userID, comp, source)
		status = s
		reason = r
		return struct{}{}, createErr
	})

	if err != nil && status == "" {
		status = output.RecurrenceStatusFailure
		reason = err.Error()
	}

	return output.RecurrenceResultEntry{
		Competence: comp.String(),
		Status:     status,
		Reason:     reason,
	}
}

func (uc *CreateRecurrence) upgradeExisting(
	ctx context.Context,
	budgets interfaces.BudgetRepository,
	existing entities.Budget,
	source entities.Budget,
	comp valueobjects.Competence,
) (output.RecurrenceStatus, string, error) {
	if existing.IsActive() {
		return output.RecurrenceStatusConflict, "orçamento já ativado", nil
	}

	updatedAllocs := uc.cloner.Rebase(existing, source)
	existing.SetAllocations(updatedAllocs)

	budget := entities.HydrateBudget(
		existing.ID(),
		existing.UserID(),
		existing.Competence(),
		source.TotalCents(),
		entities.BudgetStateDraft,
		nil,
		existing.AutoDraft(),
		updatedAllocs,
		existing.CreatedAt(),
		time.Now().UTC(),
	)

	if err := budgets.Activate(ctx, budget); err != nil {
		return "", "", fmt.Errorf("budgets.usecase.create_recurrence: atualizar rascunho %s: %w", comp, err)
	}

	if existing.AutoDraft() {
		return output.RecurrenceStatusCompletedFromDraft, "", nil
	}
	return output.RecurrenceStatusUpdated, "", nil
}

func (uc *CreateRecurrence) createFromSource(
	ctx context.Context,
	budgets interfaces.BudgetRepository,
	userID uuid.UUID,
	comp valueobjects.Competence,
	source entities.Budget,
) (output.RecurrenceStatus, string, error) {
	newBudget, cloneErr := uc.cloner.Clone(source, comp, userID, time.Now().UTC())
	if cloneErr != nil {
		return "", "", fmt.Errorf("budgets.usecase.create_recurrence: clonar %s: %w", comp, cloneErr)
	}

	if err := budgets.CreateDraft(ctx, newBudget); err != nil {
		if errors.Is(err, interfaces.ErrBudgetConflict) {
			return output.RecurrenceStatusConflict, "conflito ao criar orçamento", nil
		}
		return "", "", fmt.Errorf("budgets.usecase.create_recurrence: criar %s: %w", comp, err)
	}

	return output.RecurrenceStatusCreated, "", nil
}
