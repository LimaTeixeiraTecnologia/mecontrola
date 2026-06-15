package dispatcher

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	budgetsinput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/input"
	budgetsoutput "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type ListBudgetsAlertsUseCase interface {
	Execute(ctx context.Context, in budgetsinput.ListAlertsInput) (budgetsoutput.ListAlertsOutput, error)
}

type BudgetsAdapter struct {
	listUseCase ListBudgetsAlertsUseCase
}

func NewBudgetsAdapter(listUseCase ListBudgetsAlertsUseCase) *BudgetsAdapter {
	return &BudgetsAdapter{listUseCase: listUseCase}
}

type budgetsListFilters struct {
	Month string `json:"month"`
}

func (a *BudgetsAdapter) List(ctx context.Context, userID uuid.UUID, rawFilters json.RawMessage) (string, error) {
	if a.listUseCase == nil {
		return "", fmt.Errorf("agent.llm.dispatcher.budgets.list: %w", ErrIntentUnsupported)
	}
	competence := defaultBudgetsCompetence()
	if len(rawFilters) > 0 {
		var parsed budgetsListFilters
		if err := json.Unmarshal(rawFilters, &parsed); err == nil && strings.TrimSpace(parsed.Month) != "" {
			competence = strings.TrimSpace(parsed.Month)
		}
	}

	competenceVO, err := valueobjects.NewCompetence(competence)
	if err != nil {
		return "", fmt.Errorf("budgets.list: invalid competence %q: %w", competence, err)
	}

	in := budgetsinput.ListAlertsInput{
		UserID:     userID.String(),
		Competence: &competenceVO,
		Limit:      10,
	}

	out, err := a.listUseCase.Execute(ctx, in)
	if err != nil {
		return "", fmt.Errorf("budgets.list: %w", err)
	}
	if len(out.Alerts) == 0 {
		return fmt.Sprintf("Nenhum alerta de orcamento em %s.", competence), nil
	}

	parts := make([]string, 0, len(out.Alerts))
	for _, a := range out.Alerts {
		state := strings.ToLower(a.State)
		parts = append(parts, fmt.Sprintf("%s %d%% (%s)", a.RootSlug, a.Threshold, state))
	}
	return fmt.Sprintf("Em %s voce tem %d alerta(s) de orcamento: %s.",
		competence, len(out.Alerts), strings.Join(parts, "; ")), nil
}

func defaultBudgetsCompetence() string {
	return time.Now().UTC().Format("2006-01")
}
