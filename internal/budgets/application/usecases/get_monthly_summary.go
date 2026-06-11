package usecases

import (
	"context"
	"errors"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/database/uow"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/mappers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/commands"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type GetMonthlySummary struct {
	factory interfaces.RepositoryFactory
	uow     uow.UnitOfWork[output.MonthlySummaryOutput]
	o11y    observability.Observability
}

func NewGetMonthlySummary(
	factory interfaces.RepositoryFactory,
	u uow.UnitOfWork[output.MonthlySummaryOutput],
	o11y observability.Observability,
) *GetMonthlySummary {
	return &GetMonthlySummary{
		factory: factory,
		uow:     u,
		o11y:    o11y,
	}
}

func (uc *GetMonthlySummary) Execute(ctx context.Context, userID string, competenceStr string) (output.MonthlySummaryOutput, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "budgets.usecase.get_monthly_summary")
	defer span.End()

	cmd, err := commands.NewGetMonthlySummaryCommand(userID, competenceStr)
	if err != nil {
		span.RecordError(err)
		if errors.Is(err, commands.ErrCommandInvalidUserID) {
			return output.MonthlySummaryOutput{}, ErrGetSummaryInvalidUserID
		}
		return output.MonthlySummaryOutput{}, ErrGetSummaryInvalidCompetence
	}

	result, execErr := uc.uow.Do(ctx, func(ctx context.Context, tx database.DBTX) (output.MonthlySummaryOutput, error) {
		budgets := uc.factory.BudgetRepository(tx)
		expenses := uc.factory.ExpenseRepository(tx)
		budget, findErr := budgets.GetByUserCompetence(ctx, cmd.UserID, cmd.Competence)
		if findErr != nil {
			if errors.Is(findErr, interfaces.ErrBudgetNotFound) {
				return output.MonthlySummaryOutput{}, interfaces.ErrBudgetNotFound
			}
			return output.MonthlySummaryOutput{}, fmt.Errorf("budgets.usecase.get_monthly_summary: buscar orçamento: %w", findErr)
		}

		spentByRoot, sumErr := expenses.SumByRoot(ctx, cmd.UserID, cmd.Competence)
		if sumErr != nil {
			return output.MonthlySummaryOutput{}, fmt.Errorf("budgets.usecase.get_monthly_summary: somar despesas: %w", sumErr)
		}
		return uc.buildMonthlySummary(cmd.UserID.String(), cmd.Competence.String(), budget, spentByRoot), nil
	})

	if execErr != nil {
		span.RecordError(execErr)
		if errors.Is(execErr, interfaces.ErrBudgetNotFound) {
			return output.MonthlySummaryOutput{}, execErr
		}
		uc.o11y.Logger().Warn(ctx, "budgets.usecase.get_monthly_summary.failed",
			observability.String("user_id", userID),
			observability.String("competence", competenceStr),
			observability.Error(execErr),
		)
		return output.MonthlySummaryOutput{}, execErr
	}

	return result, nil
}

func (uc *GetMonthlySummary) buildMonthlySummary(
	userID string,
	competence string,
	budget entities.Budget,
	spentByRoot map[valueobjects.RootSlug]int64,
) output.MonthlySummaryOutput {
	allocations, totalSpent, totalPlanned, hasPlanned := uc.buildAllocationSummaries(budget, spentByRoot)
	return mappers.M.MonthlySummary(mappers.MonthlySummaryInput{
		UserID:            userID,
		Competence:        competence,
		TotalCents:        uc.totalCentsPointer(budget),
		AutoDraft:         budget.AutoDraft(),
		State:             uc.budgetState(budget),
		Allocations:       allocations,
		TotalSpentCents:   totalSpent,
		TotalPlannedCents: uc.totalPlannedPointer(budget, totalPlanned, hasPlanned),
		PercentageTotal:   uc.percentageTotal(budget, totalSpent, totalPlanned, hasPlanned),
	})
}

func (uc *GetMonthlySummary) buildAllocationSummaries(
	budget entities.Budget,
	spentByRoot map[valueobjects.RootSlug]int64,
) ([]output.AllocationSummary, int64, int64, bool) {
	allocationsByRoot := make(map[valueobjects.RootSlug]entities.Allocation, len(budget.Allocations()))
	for _, allocation := range budget.Allocations() {
		allocationsByRoot[allocation.RootSlug()] = allocation
	}

	allocationSummaries := make([]output.AllocationSummary, 0, len(valueobjects.CanonicalOrder()))
	var totalSpent int64
	var totalPlanned int64
	var hasPlanned bool
	for _, rootSlug := range valueobjects.CanonicalOrder() {
		summary, spent, planned, plannedExists := uc.buildAllocationSummary(budget, allocationsByRoot, spentByRoot, rootSlug)
		allocationSummaries = append(allocationSummaries, summary)
		totalSpent += spent
		totalPlanned += planned
		hasPlanned = hasPlanned || plannedExists
	}

	return allocationSummaries, totalSpent, totalPlanned, hasPlanned
}

func (uc *GetMonthlySummary) buildAllocationSummary(
	budget entities.Budget,
	allocationsByRoot map[valueobjects.RootSlug]entities.Allocation,
	spentByRoot map[valueobjects.RootSlug]int64,
	rootSlug valueobjects.RootSlug,
) (output.AllocationSummary, int64, int64, bool) {
	spent := spentByRoot[rootSlug]
	plannedCents, plannedExists := uc.plannedCents(budget, allocationsByRoot[rootSlug])
	plannedValue := int64(0)
	var percentageSpent *float64
	if plannedCents != nil {
		plannedValue = *plannedCents
		if plannedValue > 0 {
			percentage := float64(spent) / float64(plannedValue) * 100
			percentageSpent = &percentage
		}
	}

	return output.AllocationSummary{
		RootSlug:        rootSlug.String(),
		PlannedCents:    plannedCents,
		SpentCents:      spent,
		PercentageSpent: percentageSpent,
	}, spent, plannedValue, plannedExists
}

func (uc *GetMonthlySummary) plannedCents(budget entities.Budget, allocation entities.Allocation) (*int64, bool) {
	if allocation.RootSlug().String() == "" {
		return nil, false
	}
	if budget.AutoDraft() || (!budget.IsActive() && allocation.PlannedCents() <= 0) {
		return nil, false
	}

	plannedCents := allocation.PlannedCents()
	return &plannedCents, true
}

func (uc *GetMonthlySummary) totalCentsPointer(budget entities.Budget) *int64 {
	if budget.AutoDraft() {
		return nil
	}
	totalCents := budget.TotalCents()
	return &totalCents
}

func (uc *GetMonthlySummary) totalPlannedPointer(budget entities.Budget, totalPlanned int64, hasPlanned bool) *int64 {
	if budget.AutoDraft() || !hasPlanned {
		return nil
	}
	return &totalPlanned
}

func (uc *GetMonthlySummary) percentageTotal(budget entities.Budget, totalSpent int64, totalPlanned int64, hasPlanned bool) *float64 {
	if budget.AutoDraft() || !hasPlanned || totalPlanned <= 0 {
		return nil
	}
	percentage := float64(totalSpent) / float64(totalPlanned) * 100
	return &percentage
}

func (uc *GetMonthlySummary) budgetState(budget entities.Budget) string {
	if budget.IsActive() {
		return "active"
	}
	return "draft"
}
