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
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type CreateBudget struct {
	budgets interfaces.BudgetRepository
	uow     uow.UnitOfWork[entities.Budget]
	o11y    observability.Observability
}

func NewCreateBudget(
	budgets interfaces.BudgetRepository,
	u uow.UnitOfWork[entities.Budget],
	o11y observability.Observability,
) *CreateBudget {
	return &CreateBudget{budgets: budgets, uow: u, o11y: o11y}
}

func (uc *CreateBudget) Execute(ctx context.Context, in input.CreateBudgetInput) (output.BudgetOutput, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "budgets.usecase.create_budget")
	defer span.End()

	userID, err := uuid.Parse(in.UserID)
	if err != nil {
		return output.BudgetOutput{}, ErrBudgetInvalidUserID
	}

	competence, err := valueobjects.NewCompetence(in.Competence)
	if err != nil {
		return output.BudgetOutput{}, ErrBudgetInvalidCompetence
	}

	allocs, err := parseAllocInputs(in.Allocations)
	if err != nil {
		return output.BudgetOutput{}, err
	}

	now := time.Now().UTC()

	b, execErr := uc.uow.Do(ctx, func(ctx context.Context, tx database.DBTX) (entities.Budget, error) {
		budget := entities.NewBudget(userID, competence, in.TotalCents, now)

		for _, a := range allocs {
			budget.SetAllocations(append(budget.Allocations(), entities.NewAllocation(budget.ID(), a.rootSlug, a.basisPoints, 0)))
		}

		if createErr := uc.budgets.CreateDraft(ctx, tx, budget); createErr != nil {
			if errors.Is(createErr, interfaces.ErrBudgetConflict) {
				return entities.Budget{}, interfaces.ErrBudgetConflict
			}
			return entities.Budget{}, fmt.Errorf("budgets.usecase.create_budget: criar rascunho: %w", createErr)
		}
		return budget, nil
	})

	if execErr != nil {
		span.RecordError(execErr)
		if errors.Is(execErr, interfaces.ErrBudgetConflict) {
			return output.BudgetOutput{}, execErr
		}
		uc.o11y.Logger().Error(ctx, "budgets.usecase.create_budget.failed",
			observability.String("user_id", in.UserID),
			observability.String("competence", in.Competence),
			observability.Error(execErr),
		)
		return output.BudgetOutput{}, execErr
	}

	return mapBudgetOutput(b), nil
}

type parsedAlloc struct {
	rootSlug    valueobjects.RootSlug
	basisPoints int
}

func parseAllocInputs(ins []input.AllocationInput) ([]parsedAlloc, error) {
	result := make([]parsedAlloc, 0, len(ins))
	sum := 0
	for _, a := range ins {
		slug, err := valueobjects.ParseRootSlug(a.RootSlug)
		if err != nil {
			return nil, ErrBudgetInvalidAllocationRootSlug
		}
		if a.BasisPoints < 0 || a.BasisPoints > 10000 {
			return nil, ErrBudgetAllocationBasisPointsInvalid
		}
		sum += a.BasisPoints
		result = append(result, parsedAlloc{rootSlug: slug, basisPoints: a.BasisPoints})
	}
	if sum > 10000 {
		return nil, ErrBudgetAllocationSumExceeds10000
	}
	return result, nil
}

func mapBudgetOutput(b entities.Budget) output.BudgetOutput {
	allocs := make([]output.AllocationOutput, 0, len(b.Allocations()))
	for _, a := range b.Allocations() {
		allocs = append(allocs, output.AllocationOutput{
			RootSlug:     a.RootSlug().String(),
			BasisPoints:  a.BasisPoints(),
			PlannedCents: a.PlannedCents(),
		})
	}
	state := "draft"
	if b.IsActive() {
		state = "active"
	}
	return output.BudgetOutput{
		ID:          b.ID().String(),
		UserID:      b.UserID().String(),
		Competence:  b.Competence().String(),
		TotalCents:  b.TotalCents(),
		State:       state,
		AutoDraft:   b.AutoDraft(),
		ActivatedAt: b.ActivatedAt(),
		Allocations: allocs,
		CreatedAt:   b.CreatedAt(),
		UpdatedAt:   b.UpdatedAt(),
	}
}
