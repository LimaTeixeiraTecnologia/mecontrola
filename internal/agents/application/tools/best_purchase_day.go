package tools

import (
	"context"
	"fmt"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/tool"
)

type BestPurchaseDayInput struct {
	Bank   string `json:"bank"`
	DueDay int    `json:"dueDay"`
}

type BestPurchaseDayOutput struct {
	ClosingDay      int `json:"closingDay"`
	BestPurchaseDay int `json:"bestPurchaseDay"`
}

func BuildBestPurchaseDayTool(cards interfaces.CardManager) tool.ToolHandle {
	in := llm.Schema{
		Name:   "best_purchase_day_input",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"bank":   map[string]any{"type": "string"},
				"dueDay": map[string]any{"type": "integer"},
			},
			"required":             []string{"bank", "dueDay"},
			"additionalProperties": false,
		},
	}
	out := llm.Schema{
		Name:   "best_purchase_day_output",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"closingDay":      map[string]any{"type": "integer"},
				"bestPurchaseDay": map[string]any{"type": "integer"},
			},
			"required":             []string{"closingDay", "bestPurchaseDay"},
			"additionalProperties": false,
		},
	}
	return tool.NewTool("best_purchase_day", "Calcula o melhor dia para fazer compras em um cartão dado o banco e o dia de vencimento.", in, out, buildBestPurchaseDayExec(cards))
}

func buildBestPurchaseDayExec(cards interfaces.CardManager) func(context.Context, BestPurchaseDayInput) (BestPurchaseDayOutput, error) {
	return func(ctx context.Context, in BestPurchaseDayInput) (BestPurchaseDayOutput, error) {
		result, err := cards.BestPurchaseDay(ctx, in.Bank, in.DueDay)
		if err != nil {
			return BestPurchaseDayOutput{}, fmt.Errorf("best_purchase_day: %w", err)
		}
		return BestPurchaseDayOutput{
			ClosingDay:      result.ClosingDay,
			BestPurchaseDay: result.BestPurchaseDay,
		}, nil
	}
}
