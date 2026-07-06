package tools

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/tool"
)

type AdjustAllocationInput struct {
	Competence string `json:"competence"`
	RootSlug   string `json:"rootSlug"`
	Percentage int    `json:"percentage"`
}

type AdjustAllocationOutput struct {
	Competence string `json:"competence"`
	RootSlug   string `json:"rootSlug"`
	Percentage int    `json:"percentage"`
	OK         bool   `json:"ok"`
}

func BuildAdjustAllocationTool(planner interfaces.BudgetPlanner) tool.ToolHandle {
	in := llm.Schema{
		Name:   "adjust_allocation_input",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"competence": map[string]any{"type": "string"},
				"rootSlug":   map[string]any{"type": "string"},
				"percentage": map[string]any{"type": "integer"},
			},
			"required":             []string{"competence", "rootSlug", "percentage"},
			"additionalProperties": false,
		},
	}
	out := llm.Schema{
		Name:   "adjust_allocation_output",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"competence": map[string]any{"type": "string"},
				"rootSlug":   map[string]any{"type": "string"},
				"percentage": map[string]any{"type": "integer"},
				"ok":         map[string]any{"type": "boolean"},
			},
			"required":             []string{"competence", "rootSlug", "percentage", "ok"},
			"additionalProperties": false,
		},
	}
	return tool.NewTool[AdjustAllocationInput, AdjustAllocationOutput]("adjust_allocation", "Ajusta a porcentagem de alocação de uma categoria no orçamento mensal do usuário.", in, out, buildAdjustAllocationExec(planner))
}

func buildAdjustAllocationExec(planner interfaces.BudgetPlanner) func(context.Context, AdjustAllocationInput) (AdjustAllocationOutput, error) {
	return func(ctx context.Context, in AdjustAllocationInput) (AdjustAllocationOutput, error) {
		resourceID, _, _, ok := agent.InboundIdentityFromContext(ctx)
		if !ok {
			return AdjustAllocationOutput{}, fmt.Errorf("adjust_allocation: identidade inbound ausente")
		}
		userID, err := uuid.Parse(resourceID)
		if err != nil {
			return AdjustAllocationOutput{}, fmt.Errorf("adjust_allocation: userId inválido: %w", err)
		}
		if err := planner.EditCategoryPercentage(ctx, userID, in.Competence, in.RootSlug, in.Percentage); err != nil {
			return AdjustAllocationOutput{}, fmt.Errorf("adjust_allocation: %w", err)
		}
		return AdjustAllocationOutput{
			Competence: in.Competence,
			RootSlug:   in.RootSlug,
			Percentage: in.Percentage,
			OK:         true,
		}, nil
	}
}
