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

const maxRecurrenceMonths = 12

type CreateRecurrence struct {
	budgets interfaces.BudgetRepository
	uow     uow.UnitOfWork[struct{}]
	o11y    observability.Observability
}

func NewCreateRecurrence(
	budgets interfaces.BudgetRepository,
	u uow.UnitOfWork[struct{}],
	o11y observability.Observability,
) *CreateRecurrence {
	return &CreateRecurrence{budgets: budgets, uow: u, o11y: o11y}
}

func (uc *CreateRecurrence) Execute(ctx context.Context, in input.CreateRecurrenceInput) (output.RecurrenceResultOutput, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "budgets.usecase.create_recurrence")
	defer span.End()

	userID, err := uuid.Parse(in.UserID)
	if err != nil {
		return output.RecurrenceResultOutput{}, ErrBudgetInvalidUserID
	}

	sourceComp, err := valueobjects.NewCompetence(in.SourceCompetence)
	if err != nil {
		return output.RecurrenceResultOutput{}, ErrBudgetInvalidCompetence
	}

	months := in.Months
	if months < 1 || months > maxRecurrenceMonths {
		return output.RecurrenceResultOutput{}, ErrRecurrenceInvalidMonths
	}

	var sourceBudget entities.Budget
	var sourceValidErr error

	_, preErr := uc.uow.Do(ctx, func(ctx context.Context, tx database.DBTX) (struct{}, error) {
		b, findErr := uc.budgets.GetByUserCompetence(ctx, tx, userID, sourceComp)
		if findErr != nil {
			sourceValidErr = ErrRecurrenceSourceInvalid
			return struct{}{}, findErr
		}

		if err := validateRecurrenceSource(b); err != nil {
			sourceValidErr = err
			return struct{}{}, err
		}

		sourceBudget = b
		return struct{}{}, nil
	})

	if preErr != nil {
		if sourceValidErr != nil {
			return output.RecurrenceResultOutput{}, sourceValidErr
		}
		return output.RecurrenceResultOutput{}, preErr
	}

	results := make([]output.RecurrenceResultEntry, 0, months)
	now := time.Now().UTC()

	current := sourceComp
	for range months {
		current = current.Next()
		comp := current

		entry := processRecurrenceMonth(ctx, uc, userID, comp, sourceBudget, now)
		results = append(results, entry)
	}

	return output.RecurrenceResultOutput{
		SourceCompetence: in.SourceCompetence,
		Results:          results,
	}, nil
}

func validateRecurrenceSource(b entities.Budget) error {
	if b.TotalCents() <= 0 {
		return ErrRecurrenceSourceNegativeTotal
	}

	if b.AutoDraft() {
		allocs := b.Allocations()
		if len(allocs) == 0 {
			return ErrRecurrenceSourceAutoDraftWithoutAllocs
		}
	}

	if b.IsDraft() {
		sum := 0
		for _, a := range b.Allocations() {
			sum += a.BasisPoints()
		}
		if sum != 10000 {
			return ErrRecurrenceSourceDraftWithoutFullAllocs
		}
	}

	return nil
}

func processRecurrenceMonth(
	ctx context.Context,
	uc *CreateRecurrence,
	userID uuid.UUID,
	comp valueobjects.Competence,
	source entities.Budget,
	now time.Time,
) output.RecurrenceResultEntry {
	var status output.RecurrenceStatus
	var reason string

	_, err := uc.uow.Do(ctx, func(ctx context.Context, tx database.DBTX) (struct{}, error) {
		existing, findErr := uc.budgets.GetByUserCompetence(ctx, tx, userID, comp)
		if findErr != nil && !errors.Is(findErr, interfaces.ErrBudgetNotFound) {
			return struct{}{}, fmt.Errorf("budgets.usecase.create_recurrence: buscar competência %s: %w", comp, findErr)
		}

		if findErr == nil {
			if existing.IsActive() {
				status = output.RecurrenceStatusConflict
				reason = "orçamento já ativado"
				return struct{}{}, nil
			}

			updatedAllocs := buildAllocsFromSource(existing.ID(), source)
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
				now,
			)

			if activateErr := uc.budgets.Activate(ctx, tx, budget); activateErr != nil {
				return struct{}{}, fmt.Errorf("budgets.usecase.create_recurrence: atualizar rascunho %s: %w", comp, activateErr)
			}

			if existing.AutoDraft() {
				status = output.RecurrenceStatusCompletedFromDraft
			} else {
				status = output.RecurrenceStatusUpdated
			}
			return struct{}{}, nil
		}

		allocInputs := make([]services.AllocationInput, 0, len(source.Allocations()))
		for _, a := range source.Allocations() {
			allocInputs = append(allocInputs, services.AllocationInput{
				RootSlug:    a.RootSlug(),
				BasisPoints: a.BasisPoints(),
			})
		}
		distributed := services.Distribute(source.TotalCents(), allocInputs)

		newBudget := entities.NewBudget(userID, comp, source.TotalCents(), now)
		allocs := make([]entities.Allocation, 0, len(distributed))
		for _, r := range distributed {
			allocs = append(allocs, entities.NewAllocation(newBudget.ID(), r.RootSlug, r.BasisPoints, r.PlannedCents))
		}
		newBudget.SetAllocations(allocs)

		if createErr := uc.budgets.CreateDraft(ctx, tx, newBudget); createErr != nil {
			if errors.Is(createErr, interfaces.ErrBudgetConflict) {
				status = output.RecurrenceStatusConflict
				reason = "conflito ao criar orçamento"
				return struct{}{}, nil
			}
			return struct{}{}, fmt.Errorf("budgets.usecase.create_recurrence: criar %s: %w", comp, createErr)
		}

		status = output.RecurrenceStatusCreated
		return struct{}{}, nil
	})

	if err != nil {
		if status == "" {
			status = output.RecurrenceStatusFailure
			reason = err.Error()
		}
	}

	return output.RecurrenceResultEntry{
		Competence: comp.String(),
		Status:     status,
		Reason:     reason,
	}
}

func buildAllocsFromSource(budgetID uuid.UUID, source entities.Budget) []entities.Allocation {
	allocInputs := make([]services.AllocationInput, 0, len(source.Allocations()))
	for _, a := range source.Allocations() {
		allocInputs = append(allocInputs, services.AllocationInput{
			RootSlug:    a.RootSlug(),
			BasisPoints: a.BasisPoints(),
		})
	}
	distributed := services.Distribute(source.TotalCents(), allocInputs)
	allocs := make([]entities.Allocation, 0, len(distributed))
	for _, r := range distributed {
		allocs = append(allocs, entities.NewAllocation(budgetID, r.RootSlug, r.BasisPoints, r.PlannedCents))
	}
	return allocs
}
