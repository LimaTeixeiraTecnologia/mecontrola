package binding

import (
	"context"
	"errors"
	"fmt"
	"strings"

	appservices "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/services"
	budgetsinput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/input"
	budgetsoutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/output"
	budgetsinterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces"
	budgetsentities "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
)

type editCategoryPercentageUseCase interface {
	Execute(ctx context.Context, in budgetsinput.EditCategoryPercentageInput) (budgetsoutput.BudgetOutput, error)
}

type CategoryPercentageEditorAdapter struct {
	uc editCategoryPercentageUseCase
}

func NewCategoryPercentageEditorAdapter(uc editCategoryPercentageUseCase) *CategoryPercentageEditorAdapter {
	return &CategoryPercentageEditorAdapter{uc: uc}
}

func (a *CategoryPercentageEditorAdapter) Execute(ctx context.Context, in appservices.CategoryPercentageEditorInput) (appservices.CategoryPercentageEditorResult, error) {
	slug, ok := resolveRootSlug(in.CategoryName)
	if !ok {
		return appservices.CategoryPercentageEditorResult{}, fmt.Errorf("%w: %q", appservices.ErrCategoryPercentageUnknownCategory, in.CategoryName)
	}
	ctx = withWhatsAppPrincipal(ctx, in.UserID)
	out, err := a.uc.Execute(ctx, budgetsinput.EditCategoryPercentageInput{
		UserID:     in.UserID.String(),
		Competence: in.Competence,
		RootSlug:   slug,
		Percentage: in.Percentage,
	})
	if err != nil {
		if errors.Is(err, budgetsinterfaces.ErrBudgetNotFound) || errors.Is(err, budgetsentities.ErrBudgetNotActive) {
			return appservices.CategoryPercentageEditorResult{}, errors.Join(appservices.ErrCategoryPercentageNoBudget, err)
		}
		return appservices.CategoryPercentageEditorResult{}, fmt.Errorf("agent: category percentage editor: %w", err)
	}
	return appservices.CategoryPercentageEditorResult{
		Competence: out.Competence,
		RootSlug:   slug,
		Percentage: in.Percentage,
	}, nil
}

func resolveRootSlug(name string) (string, bool) {
	normalized := strings.ToLower(strings.TrimSpace(name))
	switch normalized {
	case "custo fixo", "custo_fixo", "expense.custo_fixo", "fixo", "custos fixos":
		return "expense.custo_fixo", true
	case "conhecimento", "expense.conhecimento", "educacao", "educação":
		return "expense.conhecimento", true
	case "prazeres", "expense.prazeres", "lazer", "prazer":
		return "expense.prazeres", true
	case "metas", "expense.metas", "meta", "objetivos":
		return "expense.metas", true
	case "liberdade financeira", "liberdade_financeira", "expense.liberdade_financeira", "liberdade", "investimentos":
		return "expense.liberdade_financeira", true
	default:
		return "", false
	}
}
