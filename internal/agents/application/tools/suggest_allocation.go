package tools

import (
	"context"
	"fmt"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/tool"
)

type SuggestAllocationInputItem struct {
	RootSlug    string `json:"rootSlug"`
	BasisPoints int    `json:"basisPoints"`
}

type SuggestAllocationInput struct {
	TotalCents  int64                        `json:"totalCents"`
	Allocations []SuggestAllocationInputItem `json:"allocations"`
}

type SuggestAllocationOutputItem struct {
	RootSlug     string `json:"rootSlug"`
	BasisPoints  int    `json:"basisPoints"`
	PlannedCents int64  `json:"plannedCents"`
}

type SuggestAllocationOutput struct {
	Allocations []SuggestAllocationOutputItem `json:"allocations"`
}

func BuildSuggestAllocationTool(planner interfaces.BudgetPlanner) tool.ToolHandle {
	in := llm.Schema{
		Name:   "suggest_allocation_input",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"totalCents":  map[string]any{"type": "integer"},
				"allocations": map[string]any{"type": "array"},
			},
			"required":             []string{"totalCents", "allocations"},
			"additionalProperties": false,
		},
	}
	out := llm.Schema{
		Name:   "suggest_allocation_output",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"allocations": map[string]any{"type": "array"},
			},
			"required":             []string{"allocations"},
			"additionalProperties": false,
		},
	}
	return tool.NewTool[SuggestAllocationInput, SuggestAllocationOutput]("suggest_allocation", "Sugere a distribuição em centavos para um orçamento dado o total e as alocações por categoria.", in, out, buildSuggestAllocationExec(planner))
}

func buildSuggestAllocationExec(planner interfaces.BudgetPlanner) func(context.Context, SuggestAllocationInput) (SuggestAllocationOutput, error) {
	return func(ctx context.Context, in SuggestAllocationInput) (SuggestAllocationOutput, error) {
		allocs := make([]interfaces.AllocationBP, len(in.Allocations))
		for i, a := range in.Allocations {
			allocs[i] = interfaces.AllocationBP{
				RootSlug:    a.RootSlug,
				BasisPoints: a.BasisPoints,
			}
		}
		result, err := planner.SuggestAllocation(ctx, in.TotalCents, allocs)
		if err != nil {
			return SuggestAllocationOutput{}, fmt.Errorf("suggest_allocation: %w", err)
		}
		out := make([]SuggestAllocationOutputItem, len(result))
		for i, r := range result {
			out[i] = SuggestAllocationOutputItem{
				RootSlug:     r.RootSlug,
				BasisPoints:  r.BasisPoints,
				PlannedCents: r.PlannedCents,
			}
		}
		return SuggestAllocationOutput{Allocations: out}, nil
	}
}
