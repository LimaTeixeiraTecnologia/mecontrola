package usecases

import (
	"context"
	"errors"
	"fmt"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/database/uow"
	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

var ErrGetSummaryInvalidUserID = errors.New("budgets: user_id inválido para resumo mensal")

var ErrGetSummaryInvalidCompetence = errors.New("budgets: competence inválida para resumo mensal")

type GetMonthlySummary struct {
	budgets  interfaces.BudgetRepository
	expenses interfaces.ExpenseRepository
	uow      uow.UnitOfWork[output.MonthlySummaryOutput]
	o11y     observability.Observability
}

func NewGetMonthlySummary(
	budgets interfaces.BudgetRepository,
	expenses interfaces.ExpenseRepository,
	u uow.UnitOfWork[output.MonthlySummaryOutput],
	o11y observability.Observability,
) *GetMonthlySummary {
	return &GetMonthlySummary{
		budgets:  budgets,
		expenses: expenses,
		uow:      u,
		o11y:     o11y,
	}
}

func (uc *GetMonthlySummary) Execute(ctx context.Context, userID string, competenceStr string) (output.MonthlySummaryOutput, error) {
	ctx, span := uc.o11y.Tracer().Start(ctx, "budgets.usecase.get_monthly_summary")
	defer span.End()

	uid, err := uuid.Parse(userID)
	if err != nil {
		return output.MonthlySummaryOutput{}, ErrGetSummaryInvalidUserID
	}

	competence, err := valueobjects.NewCompetence(competenceStr)
	if err != nil {
		return output.MonthlySummaryOutput{}, ErrGetSummaryInvalidCompetence
	}

	result, execErr := uc.uow.Do(ctx, func(ctx context.Context, tx database.DBTX) (output.MonthlySummaryOutput, error) {
		budget, findErr := uc.budgets.GetByUserCompetence(ctx, tx, uid, competence)
		if findErr != nil {
			if errors.Is(findErr, interfaces.ErrBudgetNotFound) {
				return output.MonthlySummaryOutput{}, interfaces.ErrBudgetNotFound
			}
			return output.MonthlySummaryOutput{}, fmt.Errorf("budgets.usecase.get_monthly_summary: buscar orçamento: %w", findErr)
		}

		spentByRoot, sumErr := uc.expenses.SumByRoot(ctx, tx, uid, competence)
		if sumErr != nil {
			return output.MonthlySummaryOutput{}, fmt.Errorf("budgets.usecase.get_monthly_summary: somar despesas: %w", sumErr)
		}

		allocByRoot := make(map[valueobjects.RootSlug]entities.Allocation, len(budget.Allocations()))
		for _, a := range budget.Allocations() {
			allocByRoot[a.RootSlug()] = a
		}

		canonical := valueobjects.CanonicalOrder()
		allocs := make([]output.AllocationSummary, 0, len(canonical))
		var totalSpent int64
		var totalPlanned int64
		var hasPlanned bool
		for _, root := range canonical {
			spent := spentByRoot[root]
			totalSpent += spent
			var plannedCents *int64
			var pct *float64
			if a, ok := allocByRoot[root]; ok {
				if !budget.AutoDraft() && (budget.IsActive() || a.PlannedCents() > 0) {
					pc := a.PlannedCents()
					plannedCents = &pc
					hasPlanned = true
					totalPlanned += pc
					if pc > 0 {
						v := float64(spent) / float64(pc) * 100
						pct = &v
					}
				}
			}
			allocs = append(allocs, output.AllocationSummary{
				RootSlug:        root.String(),
				PlannedCents:    plannedCents,
				SpentCents:      spent,
				PercentageSpent: pct,
			})
		}

		state := "draft"
		if budget.IsActive() {
			state = "active"
		}

		var totalCents *int64
		if !budget.AutoDraft() || budget.TotalCents() > 0 {
			tc := budget.TotalCents()
			totalCents = &tc
		}

		if budget.AutoDraft() {
			totalCents = nil
		}

		var totalPlannedCents *int64
		var percentageTotal *float64
		if !budget.AutoDraft() && hasPlanned {
			tp := totalPlanned
			totalPlannedCents = &tp
			if tp > 0 {
				v := float64(totalSpent) / float64(tp) * 100
				percentageTotal = &v
			}
		}

		return output.MonthlySummaryOutput{
			UserID:            uid.String(),
			Competence:        competence.String(),
			TotalCents:        totalCents,
			AutoDraft:         budget.AutoDraft(),
			State:             state,
			Allocations:       allocs,
			TotalSpentCents:   totalSpent,
			TotalPlannedCents: totalPlannedCents,
			PercentageTotal:   percentageTotal,
		}, nil
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
