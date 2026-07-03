package tools

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/tool"
)

type QueryPlanInput struct {
	Competence string `json:"competence,omitempty"`
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
	Competence        string                      `json:"competence"`
	TotalCents        *int64                      `json:"totalCents,omitempty"`
	State             string                      `json:"state"`
	AutoDraft         bool                        `json:"autoDraft"`
	TotalSpentCents   int64                       `json:"totalSpentCents"`
	TotalPlannedCents *int64                      `json:"totalPlannedCents,omitempty"`
	Allocations       []QueryPlanAllocationOutput `json:"allocations"`
	Alerts            []QueryPlanAlertOutput      `json:"alerts"`
}

func BuildQueryPlanTool(planner interfaces.BudgetPlanner) tool.ToolHandle {
	in := llm.Schema{
		Name:   "query_plan_input",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"competence": map[string]any{"type": "string"},
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
				"competence":        map[string]any{"type": "string"},
				"totalCents":        map[string]any{"type": "integer"},
				"state":             map[string]any{"type": "string"},
				"autoDraft":         map[string]any{"type": "boolean"},
				"totalSpentCents":   map[string]any{"type": "integer"},
				"totalPlannedCents": map[string]any{"type": "integer"},
				"allocations":       map[string]any{"type": "array"},
				"alerts":            map[string]any{"type": "array"},
			},
			"required":             []string{"competence", "state", "autoDraft", "totalSpentCents", "allocations", "alerts"},
			"additionalProperties": false,
		},
	}
	return tool.NewTool[QueryPlanInput, QueryPlanOutput]("query_plan", "Consulta o plano orçamentário mensal e alertas do usuário.", in, out, buildQueryPlanExec(planner))
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
		competence := in.Competence
		if competence == "" {
			loc, locErr := time.LoadLocation("America/Sao_Paulo")
			if locErr != nil {
				loc = time.UTC
			}
			competence = time.Now().In(loc).Format("2006-01")
		}
		summary, err := planner.GetMonthlySummary(ctx, userID, competence)
		if err != nil {
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
