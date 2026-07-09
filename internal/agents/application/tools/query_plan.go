package tools

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	budgetsvo "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/tool"
)

const (
	queryPlanOutcomeOK       = "ok"
	queryPlanOutcomeClarify  = "clarify"
	queryPlanOutcomeNotFound = "not_found"
)

type QueryPlanInput struct {
	MonthRefKind string `json:"monthRefKind,omitempty"`
	Year         int    `json:"year,omitempty"`
	Month        int    `json:"month,omitempty"`
}

type QueryPlanAlertOutput struct {
	ID           string `json:"id"`
	Competence   string `json:"competence"`
	RootSlug     string `json:"rootSlug"`
	Threshold    int    `json:"threshold"`
	State        string `json:"state"`
	SpentCents   int64  `json:"spentCents"`
	PlannedCents int64  `json:"plannedCents"`
}

type QueryPlanAllocationOutput struct {
	RootSlug        string   `json:"rootSlug"`
	PlannedCents    *int64   `json:"plannedCents,omitempty"`
	SpentCents      int64    `json:"spentCents"`
	PercentageSpent *float64 `json:"percentageSpent,omitempty"`
}

type QueryPlanOutput struct {
	Outcome           string                      `json:"outcome"`
	Competence        string                      `json:"competence"`
	TotalCents        *int64                      `json:"totalCents,omitempty"`
	State             string                      `json:"state"`
	AutoDraft         bool                        `json:"autoDraft"`
	TotalSpentCents   int64                       `json:"totalSpentCents"`
	TotalPlannedCents *int64                      `json:"totalPlannedCents,omitempty"`
	Allocations       []QueryPlanAllocationOutput `json:"allocations"`
	Alerts            []QueryPlanAlertOutput      `json:"alerts"`
	ClarifyPrompt     string                      `json:"clarifyPrompt,omitempty"`
	OfferCreatePrompt string                      `json:"offerCreatePrompt,omitempty"`
}

func BuildQueryPlanTool(planner interfaces.BudgetPlanner) tool.ToolHandle {
	in := llm.Schema{
		Name:   "query_plan_input",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"monthRefKind": map[string]any{
					"type":        "string",
					"enum":        []string{"current", "previous", "next", "explicit", "named_without_year", "unknown"},
					"description": "Classificação da referência de mês citada pelo usuário. Use named_without_year sempre que um nome de mês (ex.: junho, março) for citado SEM um ano junto — mesmo que esse mês já tenha passado no ano corrente. Use explicit sempre que o usuário citar mês E ano juntos (numérico ou nomeado), incluindo em retrospectivas de mês específico ('como foi meu mês de maio de 2026?' → explicit, year=2026, month=5) — NUNCA use current quando um nome de mês foi citado, mesmo sem ano, e NUNCA use current para um mês específico já citado explicitamente na conversa.",
				},
				"year":  map[string]any{"type": "integer", "description": "Ano numérico, apenas quando o usuário citou explicitamente o ano junto ao mês (monthRefKind=explicit). Omitir para named_without_year."},
				"month": map[string]any{"type": "integer", "minimum": 1, "maximum": 12, "description": "Mês numérico (1-12), quando monthRefKind=explicit ou named_without_year."},
			},
			"required":             []string{},
			"additionalProperties": false,
		},
	}
	out := llm.Schema{
		Name:   "query_plan_output",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"outcome":           map[string]any{"type": "string"},
				"competence":        map[string]any{"type": "string"},
				"totalCents":        map[string]any{"type": "integer"},
				"state":             map[string]any{"type": "string"},
				"autoDraft":         map[string]any{"type": "boolean"},
				"totalSpentCents":   map[string]any{"type": "integer"},
				"totalPlannedCents": map[string]any{"type": "integer"},
				"allocations":       map[string]any{"type": "array"},
				"alerts":            map[string]any{"type": "array"},
				"clarifyPrompt":     map[string]any{"type": "string"},
				"offerCreatePrompt": map[string]any{"type": "string"},
			},
			"required":             []string{"outcome", "competence", "state", "autoDraft", "totalSpentCents", "allocations", "alerts"},
			"additionalProperties": false,
		},
	}
	return tool.NewTool("query_plan", "Consulta o plano orçamentário mensal e alertas do usuário.", in, out, buildQueryPlanExec(planner))
}

func buildQueryPlanExec(planner interfaces.BudgetPlanner) func(context.Context, QueryPlanInput) (QueryPlanOutput, error) {
	return func(ctx context.Context, in QueryPlanInput) (QueryPlanOutput, error) {
		resourceID, _, _, ok := agent.InboundIdentityFromContext(ctx)
		if !ok {
			return QueryPlanOutput{}, fmt.Errorf("query_plan: identidade não disponível no contexto")
		}
		userID, err := uuid.Parse(resourceID)
		if err != nil {
			return QueryPlanOutput{}, fmt.Errorf("query_plan: userId inválido: %w", err)
		}

		competence, clarifyReason, err := resolveCompetenceReference(in.MonthRefKind, in.Year, in.Month)
		if err != nil {
			return QueryPlanOutput{}, fmt.Errorf("query_plan: resolver competência: %w", err)
		}
		if clarifyReason != budgetsvo.ClarifyNone {
			return QueryPlanOutput{
				Outcome:       queryPlanOutcomeClarify,
				ClarifyPrompt: competenceReferenceClarifyPrompt(clarifyReason),
			}, nil
		}
		competenceStr := competence.String()
		if competenceStr == "" {
			competenceStr = currentCompetenceFallback()
		}

		summary, err := planner.GetMonthlySummary(ctx, userID, competenceStr)
		if err != nil {
			if errors.Is(err, interfaces.ErrBudgetNotFound) {
				return QueryPlanOutput{
					Outcome:           queryPlanOutcomeNotFound,
					Competence:        competenceStr,
					OfferCreatePrompt: budgetNotFoundOfferPrompt(competenceStr),
				}, nil
			}
			return QueryPlanOutput{}, fmt.Errorf("query_plan: resumo orçamentário: %w", err)
		}
		alerts, err := planner.ListAlerts(ctx, userID)
		if err != nil {
			return QueryPlanOutput{}, fmt.Errorf("query_plan: alertas: %w", err)
		}
		allocations := make([]QueryPlanAllocationOutput, len(summary.Allocations))
		for i, a := range summary.Allocations {
			allocations[i] = QueryPlanAllocationOutput{
				RootSlug:        a.RootSlug,
				PlannedCents:    a.PlannedCents,
				SpentCents:      a.SpentCents,
				PercentageSpent: a.PercentageSpent,
			}
		}
		mappedAlerts := make([]QueryPlanAlertOutput, len(alerts))
		for i, a := range alerts {
			mappedAlerts[i] = QueryPlanAlertOutput{
				ID:           a.ID,
				Competence:   a.Competence,
				RootSlug:     a.RootSlug,
				Threshold:    a.Threshold,
				State:        a.State,
				SpentCents:   a.SpentCents,
				PlannedCents: a.PlannedCents,
			}
		}
		return QueryPlanOutput{
			Outcome:           queryPlanOutcomeOK,
			Competence:        summary.Competence,
			TotalCents:        summary.TotalCents,
			State:             summary.State,
			AutoDraft:         summary.AutoDraft,
			TotalSpentCents:   summary.TotalSpentCents,
			TotalPlannedCents: summary.TotalPlannedCents,
			Allocations:       allocations,
			Alerts:            mappedAlerts,
		}, nil
	}
}
