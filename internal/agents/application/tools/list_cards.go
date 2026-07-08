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

type ListCardsInput struct{}

type ListCardsCardOutput struct {
	ID              string    `json:"id"`
	Nickname        string    `json:"nickname"`
	Bank            string    `json:"bank"`
	DueDay          int       `json:"dueDay"`
	ClosingDay      int       `json:"closingDay"`
	BestPurchaseDay int       `json:"bestPurchaseDay"`
	CreatedAt       time.Time `json:"createdAt"`
}

type ListCardsOutput struct {
	Cards []ListCardsCardOutput `json:"cards"`
}

func BuildListCardsTool(cards interfaces.CardManager) tool.ToolHandle {
	in := llm.Schema{
		Name:   "list_cards_input",
		Strict: true,
		Schema: map[string]any{
			"type":                 "object",
			"properties":           map[string]any{},
			"required":             []string{},
			"additionalProperties": false,
		},
	}
	out := llm.Schema{
		Name:   "list_cards_output",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"cards": map[string]any{"type": "array"},
			},
			"required":             []string{"cards"},
			"additionalProperties": false,
		},
	}
	return tool.NewTool("list_cards", "Lista os cartões de crédito do usuário.", in, out, buildListCardsExec(cards))
}

func buildListCardsExec(cards interfaces.CardManager) func(context.Context, ListCardsInput) (ListCardsOutput, error) {
	return func(ctx context.Context, _ ListCardsInput) (ListCardsOutput, error) {
		resourceID, _, _, ok := agent.InboundIdentityFromContext(ctx)
		if !ok {
			return ListCardsOutput{}, fmt.Errorf("list_cards: identidade não disponível no contexto")
		}
		userID, err := uuid.Parse(resourceID)
		if err != nil {
			return ListCardsOutput{}, fmt.Errorf("list_cards: userId inválido: %w", err)
		}
		result, err := cards.ListCards(ctx, userID)
		if err != nil {
			return ListCardsOutput{}, fmt.Errorf("list_cards: %w", err)
		}
		out := make([]ListCardsCardOutput, len(result))
		for i, c := range result {
			out[i] = ListCardsCardOutput{
				ID:              c.ID,
				Nickname:        c.Nickname,
				Bank:            c.Bank,
				DueDay:          c.DueDay,
				ClosingDay:      c.ClosingDay,
				BestPurchaseDay: c.BestPurchaseDay,
				CreatedAt:       c.CreatedAt,
			}
		}
		return ListCardsOutput{Cards: out}, nil
	}
}
