package tools

import (
	"context"
	"fmt"
	"time"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/tool"
)

type GetTransactionInput struct {
	TxID string `json:"txId"`
}

type GetTransactionOutput struct {
	Kind                 string    `json:"kind"`
	ID                   string    `json:"id"`
	UserID               string    `json:"userId"`
	Direction            string    `json:"direction"`
	PaymentMethod        string    `json:"paymentMethod"`
	AmountCents          int64     `json:"amountCents"`
	Description          string    `json:"description"`
	CategoryID           string    `json:"categoryId"`
	CategoryNameSnapshot string    `json:"categoryNameSnapshot"`
	RefMonth             string    `json:"refMonth"`
	OccurredAt           time.Time `json:"occurredAt"`
	Version              int64     `json:"version"`
	CreatedAt            time.Time `json:"createdAt"`
	UpdatedAt            time.Time `json:"updatedAt"`
}

func BuildGetTransactionTool(ledger interfaces.TransactionsLedger) tool.ToolHandle {
	in := llm.Schema{
		Name:   "get_transaction_input",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"txId": map[string]any{"type": "string"},
			},
			"required":             []string{"txId"},
			"additionalProperties": false,
		},
	}
	out := llm.Schema{
		Name:   "get_transaction_output",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"kind":                 map[string]any{"type": "string"},
				"id":                   map[string]any{"type": "string"},
				"userId":               map[string]any{"type": "string"},
				"direction":            map[string]any{"type": "string"},
				"paymentMethod":        map[string]any{"type": "string"},
				"amountCents":          map[string]any{"type": "integer"},
				"description":          map[string]any{"type": "string"},
				"categoryId":           map[string]any{"type": "string"},
				"categoryNameSnapshot": map[string]any{"type": "string"},
				"refMonth":             map[string]any{"type": "string"},
				"occurredAt":           map[string]any{"type": "string"},
				"version":              map[string]any{"type": "integer"},
				"createdAt":            map[string]any{"type": "string"},
				"updatedAt":            map[string]any{"type": "string"},
			},
			"required":             []string{"kind", "id", "userId", "direction", "paymentMethod", "amountCents", "description", "categoryId", "categoryNameSnapshot", "refMonth", "occurredAt", "version", "createdAt", "updatedAt"},
			"additionalProperties": false,
		},
	}
	return tool.NewTool[GetTransactionInput, GetTransactionOutput]("get_transaction", "Retorna os detalhes de um lançamento de transação pelo ID.", in, out, buildGetTransactionExec(ledger))
}

func buildGetTransactionExec(ledger interfaces.TransactionsLedger) func(context.Context, GetTransactionInput) (GetTransactionOutput, error) {
	return func(ctx context.Context, in GetTransactionInput) (GetTransactionOutput, error) {
		entry, err := ledger.GetTransaction(ctx, in.TxID)
		if err != nil {
			return GetTransactionOutput{}, fmt.Errorf("get_transaction: %w", err)
		}
		return GetTransactionOutput{
			Kind:                 entry.Kind.String(),
			ID:                   entry.ID,
			UserID:               entry.UserID,
			Direction:            entry.Direction,
			PaymentMethod:        entry.PaymentMethod,
			AmountCents:          entry.AmountCents,
			Description:          entry.Description,
			CategoryID:           entry.CategoryID,
			CategoryNameSnapshot: entry.CategoryNameSnapshot,
			RefMonth:             entry.RefMonth,
			OccurredAt:           entry.OccurredAt,
			Version:              entry.Version,
			CreatedAt:            entry.CreatedAt,
			UpdatedAt:            entry.UpdatedAt,
		}, nil
	}
}
