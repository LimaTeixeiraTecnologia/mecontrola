package binding

import (
	"context"
	"fmt"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/tools"
	budgetsinput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/input"
	budgetsoutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/output"
)

type createRecurrenceUseCase interface {
	Execute(ctx context.Context, in budgetsinput.CreateRecurrenceInput) (budgetsoutput.RecurrenceResultOutput, error)
}

type BudgetRecurrenceCreatorAdapter struct {
	uc createRecurrenceUseCase
}

func NewBudgetRecurrenceCreatorAdapter(uc createRecurrenceUseCase) *BudgetRecurrenceCreatorAdapter {
	return &BudgetRecurrenceCreatorAdapter{uc: uc}
}

func (a *BudgetRecurrenceCreatorAdapter) Execute(ctx context.Context, in tools.BudgetRecurrenceCreatorInput) (tools.BudgetRecurrenceCreatorResult, error) {
	ctx = withWhatsAppPrincipal(ctx, in.UserID)
	out, err := a.uc.Execute(ctx, budgetsinput.CreateRecurrenceInput{
		UserID:           in.UserID.String(),
		SourceCompetence: in.SourceCompetence,
		Months:           in.Months,
	})
	if err != nil {
		return tools.BudgetRecurrenceCreatorResult{}, fmt.Errorf("agent: budget recurrence creator: %w", err)
	}
	created := countCreated(out.Results)
	return tools.BudgetRecurrenceCreatorResult{
		SourceCompetence: out.SourceCompetence,
		MonthsCreated:    created,
	}, nil
}

func countCreated(results []budgetsoutput.RecurrenceResultEntry) int {
	n := 0
	for _, r := range results {
		if r.Status == budgetsoutput.RecurrenceStatusCreated ||
			r.Status == budgetsoutput.RecurrenceStatusUpdated ||
			r.Status == budgetsoutput.RecurrenceStatusCompletedFromDraft {
			n++
		}
	}
	return n
}
