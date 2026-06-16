package agent

import (
	"context"

	appservices "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/usecases"
	budgetsinput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/input"
	budgetusecases "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/usecases"
)

type expenseUpserterAdapter struct {
	uc *budgetusecases.UpsertExpense
}

func newExpenseUpserterAdapter(uc *budgetusecases.UpsertExpense) *expenseUpserterAdapter {
	return &expenseUpserterAdapter{uc: uc}
}

func (a *expenseUpserterAdapter) Execute(ctx context.Context, in usecases.ExpenseUpsertInput) (usecases.ExpenseUpsertResult, error) {
	out, err := a.uc.Execute(ctx, budgetsinput.UpsertExpenseInput{
		UserID:                in.UserID,
		Source:                in.Source,
		ExternalTransactionID: in.ExternalTransactionID,
		SubcategoryID:         in.SubcategoryID,
		Competence:            in.Competence,
		AmountCents:           in.AmountCents,
		OccurredAt:            in.OccurredAt,
	})
	if err != nil {
		return usecases.ExpenseUpsertResult{}, err
	}
	return usecases.ExpenseUpsertResult{
		ID:             out.ID,
		UserID:         out.UserID,
		SubcategoryID:  out.SubcategoryID,
		RootCategoryID: out.RootSlug,
		Competence:     out.Competence,
		AmountCents:    out.AmountCents,
		OccurredAt:     out.OccurredAt,
		Version:        out.Version,
	}, nil
}

type expenseLoggerAdapter struct {
	uc *usecases.LogExpenseFromAgent
}

func (a *expenseLoggerAdapter) Execute(ctx context.Context, in appservices.ExpenseLoggerInput) (appservices.ExpenseLoggerResult, error) {
	result, err := a.uc.Execute(ctx, usecases.LogExpenseFromAgentInput{
		UserID: in.UserID,
		Intent: in.Intent,
	})
	if err != nil {
		return appservices.ExpenseLoggerResult{}, err
	}
	return appservices.ExpenseLoggerResult{
		Persisted:      result.Persisted,
		SubcategoryID:  result.SubcategoryID,
		RootCategoryID: result.RootCategoryID,
		AmountCents:    result.AmountCents,
		Competence:     result.Competence,
		CategoryPath:   result.CategoryPath,
		OccurredAt:     result.OccurredAt,
	}, nil
}
