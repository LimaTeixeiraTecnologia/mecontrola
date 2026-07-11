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

type GetCardInput struct {
	CardID string `json:"cardId"`
}

type GetCardOutput struct {
	ID              string    `json:"id"`
	Nickname        string    `json:"nickname"`
	Bank            string    `json:"bank"`
	DueDay          int       `json:"dueDay"`
	ClosingDay      int       `json:"closingDay"`
	BestPurchaseDay int       `json:"bestPurchaseDay"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
}

func BuildGetCardTool(cards interfaces.CardManager) tool.ToolHandle {
	in := llm.Schema{
		Name:   "get_card_input",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"cardId": map[string]any{"type": "string"},
			},
			"required":             []string{"cardId"},
			"additionalProperties": false,
		},
	}
	out := llm.Schema{
		Name:   "get_card_output",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id":              map[string]any{"type": "string"},
				"nickname":        map[string]any{"type": "string"},
				"bank":            map[string]any{"type": "string"},
				"dueDay":          map[string]any{"type": "integer"},
				"closingDay":      map[string]any{"type": "integer"},
				"bestPurchaseDay": map[string]any{"type": "integer"},
				"createdAt":       map[string]any{"type": "string"},
				"updatedAt":       map[string]any{"type": "string"},
			},
			"required":             []string{"id", "nickname", "bank", "dueDay", "closingDay", "bestPurchaseDay", "createdAt", "updatedAt"},
			"additionalProperties": false,
		},
	}
	return tool.NewTool("get_card", "Retorna os detalhes de um 💳 de crédito do usuário.", in, out, buildGetCardExec(cards))
}

func buildGetCardExec(cards interfaces.CardManager) func(context.Context, GetCardInput) (GetCardOutput, error) {
	return func(ctx context.Context, in GetCardInput) (GetCardOutput, error) {
		resourceID, _, _, ok := agent.InboundIdentityFromContext(ctx)
		if !ok {
			return GetCardOutput{}, fmt.Errorf("get_card: identidade não disponível no contexto")
		}
		userID, err := uuid.Parse(resourceID)
		if err != nil {
			return GetCardOutput{}, fmt.Errorf("get_card: userId inválido: %w", err)
		}
		cardID, err := uuid.Parse(in.CardID)
		if err != nil {
			return GetCardOutput{}, fmt.Errorf("get_card: cardId inválido: %w", err)
		}
		c, err := cards.GetCard(ctx, cardID, userID)
		if err != nil {
			return GetCardOutput{}, fmt.Errorf("get_card: %w", err)
		}
		return GetCardOutput{
			ID:              c.ID,
			Nickname:        c.Nickname,
			Bank:            c.Bank,
			DueDay:          c.DueDay,
			ClosingDay:      c.ClosingDay,
			BestPurchaseDay: c.BestPurchaseDay,
			CreatedAt:       c.CreatedAt,
			UpdatedAt:       c.UpdatedAt,
		}, nil
	}
}
