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

type QueryCardInvoiceInput struct {
	CardID   string `json:"cardId"`
	RefMonth string `json:"refMonth,omitempty"`
}

type QueryCardInvoiceItemOutput struct {
	ID               string `json:"id"`
	RefMonth         string `json:"refMonth"`
	InstallmentIndex int    `json:"installmentIndex"`
	AmountCents      int64  `json:"amountCents"`
	InvoiceID        string `json:"invoiceId"`
}

type QueryCardInvoiceOutput struct {
	ID              string                       `json:"id"`
	CardID          string                       `json:"cardId"`
	RefMonth        string                       `json:"refMonth"`
	ClosingAt       time.Time                    `json:"closingAt"`
	DueAt           time.Time                    `json:"dueAt"`
	ItemsTotalCents int64                        `json:"itemsTotalCents"`
	Items           []QueryCardInvoiceItemOutput `json:"items"`
}

func BuildQueryCardInvoiceTool(ledger interfaces.TransactionsLedger) tool.ToolHandle {
	in := llm.Schema{
		Name:   "query_card_invoice_input",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"cardId":   map[string]any{"type": "string"},
				"refMonth": map[string]any{"type": "string"},
			},
			"required":             []string{"cardId"},
			"additionalProperties": false,
		},
	}
	out := llm.Schema{
		Name:   "query_card_invoice_output",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"id":              map[string]any{"type": "string"},
				"cardId":          map[string]any{"type": "string"},
				"refMonth":        map[string]any{"type": "string"},
				"closingAt":       map[string]any{"type": "string"},
				"dueAt":           map[string]any{"type": "string"},
				"itemsTotalCents": map[string]any{"type": "integer"},
				"items":           map[string]any{"type": "array"},
			},
			"required":             []string{"id", "cardId", "refMonth", "closingAt", "dueAt", "itemsTotalCents", "items"},
			"additionalProperties": false,
		},
	}
	return tool.NewTool("query_card_invoice", "Consulta a fatura de um cartão de crédito para o mês informado.", in, out, buildQueryCardInvoiceExec(ledger))
}

func buildQueryCardInvoiceExec(ledger interfaces.TransactionsLedger) func(context.Context, QueryCardInvoiceInput) (QueryCardInvoiceOutput, error) {
	return func(ctx context.Context, in QueryCardInvoiceInput) (QueryCardInvoiceOutput, error) {
		cardID, err := uuid.Parse(in.CardID)
		if err != nil {
			return QueryCardInvoiceOutput{}, fmt.Errorf("query_card_invoice: cardId inválido: %w", err)
		}
		refMonth := in.RefMonth
		if refMonth == "" {
			loc, locErr := time.LoadLocation("America/Sao_Paulo")
			if locErr != nil {
				loc = time.UTC
			}
			refMonth = time.Now().In(loc).Format("2006-01")
		}
		invoice, err := ledger.GetCardInvoice(ctx, cardID, refMonth)
		if err != nil {
			return QueryCardInvoiceOutput{}, fmt.Errorf("query_card_invoice: %w", err)
		}
		items := make([]QueryCardInvoiceItemOutput, len(invoice.Items))
		for i, item := range invoice.Items {
			items[i] = QueryCardInvoiceItemOutput{
				ID:               item.ID.String(),
				RefMonth:         item.RefMonth,
				InstallmentIndex: item.InstallmentIndex,
				AmountCents:      item.AmountCents,
				InvoiceID:        item.InvoiceID.String(),
			}
		}
		return QueryCardInvoiceOutput{
			ID:              invoice.ID.String(),
			CardID:          invoice.CardID.String(),
			RefMonth:        invoice.RefMonth,
			ClosingAt:       invoice.ClosingAt,
			DueAt:           invoice.DueAt,
			ItemsTotalCents: invoice.ItemsTotalCents,
			Items:           items,
		}, nil
	}
}
