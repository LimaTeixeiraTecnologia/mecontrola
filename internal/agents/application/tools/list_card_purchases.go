package tools

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/tool"
)

type ListCardPurchasesInput struct {
	CardID   string `json:"cardId"`
	RefMonth string `json:"refMonth,omitempty"`
	Cursor   string `json:"cursor,omitempty"`
	Limit    int    `json:"limit,omitempty"`
}

type ListCardPurchasesEntryOutput struct {
	Kind        string    `json:"kind"`
	ID          string    `json:"id"`
	RefMonth    string    `json:"refMonth"`
	AmountCents int64     `json:"amountCents"`
	Direction   string    `json:"direction"`
	Description string    `json:"description"`
	CategoryID  string    `json:"categoryId"`
	OccurredAt  time.Time `json:"occurredAt"`
	CreatedAt   time.Time `json:"createdAt"`
}

type ListCardPurchasesOutput struct {
	Entries []ListCardPurchasesEntryOutput `json:"entries"`
}

func BuildListCardPurchasesTool(ledger interfaces.TransactionsLedger) tool.ToolHandle {
	in := llm.Schema{
		Name:   "list_card_purchases_input",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"cardId":   map[string]any{"type": "string"},
				"refMonth": map[string]any{"type": "string"},
				"cursor":   map[string]any{"type": "string"},
				"limit":    map[string]any{"type": "integer"},
			},
			"required":             []string{"cardId"},
			"additionalProperties": false,
		},
	}
	out := llm.Schema{
		Name:   "list_card_purchases_output",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"entries": map[string]any{"type": "array"},
			},
			"required":             []string{"entries"},
			"additionalProperties": false,
		},
	}
	return tool.NewTool[ListCardPurchasesInput, ListCardPurchasesOutput]("list_card_purchases", "Lista as compras no cartão para o mês informado.", in, out, buildListCardPurchasesExec(ledger))
}

func buildListCardPurchasesExec(ledger interfaces.TransactionsLedger) func(context.Context, ListCardPurchasesInput) (ListCardPurchasesOutput, error) {
	return func(ctx context.Context, in ListCardPurchasesInput) (ListCardPurchasesOutput, error) {
		cardID, err := uuid.Parse(in.CardID)
		if err != nil {
			return ListCardPurchasesOutput{}, fmt.Errorf("list_card_purchases: cardId inválido: %w", err)
		}
		refMonth := in.RefMonth
		if refMonth == "" {
			loc, locErr := time.LoadLocation("America/Sao_Paulo")
			if locErr != nil {
				loc = time.UTC
			}
			refMonth = time.Now().In(loc).Format("2006-01")
		}
		limit := in.Limit
		if limit <= 0 {
			limit = 20
		}
		entries, err := ledger.ListCardPurchases(ctx, cardID, refMonth, in.Cursor, limit)
		if err != nil {
			return ListCardPurchasesOutput{}, fmt.Errorf("list_card_purchases: %w", err)
		}
		out := make([]ListCardPurchasesEntryOutput, len(entries))
		for i, e := range entries {
			out[i] = ListCardPurchasesEntryOutput{
				Kind:        e.Kind,
				ID:          e.ID,
				RefMonth:    e.RefMonth,
				AmountCents: e.AmountCents,
				Direction:   e.Direction,
				Description: e.Description,
				CategoryID:  e.CategoryID,
				OccurredAt:  e.OccurredAt,
				CreatedAt:   e.CreatedAt,
			}
		}
		return ListCardPurchasesOutput{Entries: out}, nil
	}
}
