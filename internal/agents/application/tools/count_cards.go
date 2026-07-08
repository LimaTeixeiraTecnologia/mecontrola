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

type CountCardsInput struct{}

type CountCardsOutput struct {
	Count int64 `json:"count"`
}

func BuildCountCardsTool(cards interfaces.CardManager) tool.ToolHandle {
	in := llm.Schema{
		Name:   "count_cards_input",
		Strict: true,
		Schema: map[string]any{
			"type":                 "object",
			"properties":           map[string]any{},
			"required":             []string{},
			"additionalProperties": false,
		},
	}
	out := llm.Schema{
		Name:   "count_cards_output",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"count": map[string]any{"type": "integer"},
			},
			"required":             []string{"count"},
			"additionalProperties": false,
		},
	}
	return tool.NewTool("count_cards", "Conta o número de cartões de crédito do usuário.", in, out, buildCountCardsExec(cards))
}

func buildCountCardsExec(cards interfaces.CardManager) func(context.Context, CountCardsInput) (CountCardsOutput, error) {
	return func(ctx context.Context, _ CountCardsInput) (CountCardsOutput, error) {
		resourceID, _, _, ok := agent.InboundIdentityFromContext(ctx)
		if !ok {
			return CountCardsOutput{}, fmt.Errorf("count_cards: identidade não disponível no contexto")
		}
		userID, err := uuid.Parse(resourceID)
		if err != nil {
			return CountCardsOutput{}, fmt.Errorf("count_cards: userId inválido: %w", err)
		}
		count, err := cards.CountCards(ctx, userID)
		if err != nil {
			return CountCardsOutput{}, fmt.Errorf("count_cards: %w", err)
		}
		return CountCardsOutput{Count: count}, nil
	}
}
