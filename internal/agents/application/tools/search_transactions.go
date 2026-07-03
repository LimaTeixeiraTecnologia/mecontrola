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

type SearchTransactionsInput struct {
	Query    string `json:"query"`
	RefMonth string `json:"refMonth,omitempty"`
	Limit    int    `json:"limit,omitempty"`
}

type SearchTransactionsEntryOutput struct {
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

type SearchTransactionsOutput struct {
	Entries []SearchTransactionsEntryOutput `json:"entries"`
}

func BuildSearchTransactionsTool(ledger interfaces.TransactionsLedger) tool.ToolHandle {
	in := llm.Schema{
		Name:   "search_transactions_input",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"query":    map[string]any{"type": "string"},
				"refMonth": map[string]any{"type": "string"},
				"limit":    map[string]any{"type": "integer"},
			},
			"required":             []string{"query"},
			"additionalProperties": false,
		},
	}
	out := llm.Schema{
		Name:   "search_transactions_output",
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
	return tool.NewTool[SearchTransactionsInput, SearchTransactionsOutput]("search_transactions", "Pesquisa lançamentos do usuário por termo no mês informado.", in, out, buildSearchTransactionsExec(ledger))
}

func buildSearchTransactionsExec(ledger interfaces.TransactionsLedger) func(context.Context, SearchTransactionsInput) (SearchTransactionsOutput, error) {
	return func(ctx context.Context, in SearchTransactionsInput) (SearchTransactionsOutput, error) {
		resourceID, _, _, ok := agent.InboundIdentityFromContext(ctx)
		if !ok {
			return SearchTransactionsOutput{}, fmt.Errorf("search_transactions: identidade não disponível no contexto")
		}
		userID, err := uuid.Parse(resourceID)
		if err != nil {
			return SearchTransactionsOutput{}, fmt.Errorf("search_transactions: userId inválido: %w", err)
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
		entries, err := ledger.SearchTransactions(ctx, userID, in.Query, refMonth, limit)
		if err != nil {
			return SearchTransactionsOutput{}, fmt.Errorf("search_transactions: %w", err)
		}
		out := make([]SearchTransactionsEntryOutput, len(entries))
		for i, e := range entries {
			out[i] = SearchTransactionsEntryOutput{
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
		return SearchTransactionsOutput{Entries: out}, nil
	}
}
