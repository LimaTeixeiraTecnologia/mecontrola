package tools

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
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
	Outcome         string                       `json:"outcome,omitempty"`
	Message         string                       `json:"message,omitempty"`
}

func extractQueryCardInvoiceVerbatim(o QueryCardInvoiceOutput) (string, bool) {
	return o.Message, o.Outcome == agent.ToolOutcomeClarify.String() && o.Message != ""
}

func BuildQueryCardInvoiceTool(ledger interfaces.TransactionsLedger, cards interfaces.CardManager) tool.ToolHandle {
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
				"outcome":         map[string]any{"type": "string"},
				"message":         map[string]any{"type": "string"},
			},
			"required":             []string{"id", "cardId", "refMonth", "closingAt", "dueAt", "itemsTotalCents", "items", "outcome", "message"},
			"additionalProperties": false,
		},
	}
	return tool.NewVerbatimTool("query_card_invoice", "Consulta a fatura de um 💳 de crédito para o mês informado.", in, out, buildQueryCardInvoiceExec(ledger, cards), extractQueryCardInvoiceVerbatim)
}

func buildQueryCardInvoiceExec(ledger interfaces.TransactionsLedger, cards interfaces.CardManager) func(context.Context, QueryCardInvoiceInput) (QueryCardInvoiceOutput, error) {
	return func(ctx context.Context, in QueryCardInvoiceInput) (QueryCardInvoiceOutput, error) {
		resourceID, _, _, ok := agent.InboundIdentityFromContext(ctx)
		if !ok {
			return QueryCardInvoiceOutput{}, fmt.Errorf("query_card_invoice: identidade não disponível no contexto")
		}
		userID, err := uuid.Parse(resourceID)
		if err != nil {
			return QueryCardInvoiceOutput{}, fmt.Errorf("query_card_invoice: userId inválido: %w", err)
		}
		cardID, err := uuid.Parse(in.CardID)
		if err != nil {
			return QueryCardInvoiceOutput{}, fmt.Errorf("query_card_invoice: cardId inválido: %w", err)
		}
		if _, getErr := cards.GetCard(ctx, cardID, userID); getErr != nil {
			if errors.Is(getErr, interfaces.ErrCardNotFound) {
				return QueryCardInvoiceOutput{
					Outcome: agent.ToolOutcomeClarify.String(),
					Message: cardNotFoundClarifyMessage,
				}, nil
			}
			return QueryCardInvoiceOutput{}, fmt.Errorf("query_card_invoice: %w", getErr)
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
