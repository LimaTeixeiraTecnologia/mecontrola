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

type QueryMonthInput struct {
	RefMonth string `json:"refMonth,omitempty"`
	Cursor   string `json:"cursor,omitempty"`
	Limit    int    `json:"limit,omitempty"`
}

type QueryMonthEntryOutput struct {
	Kind        string    `json:"kind"`
	ID          string    `json:"id"`
	RefMonth    string    `json:"refMonth"`
	AmountCents int64     `json:"amountCents"`
	Direction   string    `json:"direction"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"createdAt"`
}

type QueryMonthOutput struct {
	RefMonth     string                  `json:"refMonth"`
	IncomeCents  int64                   `json:"incomeCents"`
	OutcomeCents int64                   `json:"outcomeCents"`
	TotalCents   int64                   `json:"totalCents"`
	Entries      []QueryMonthEntryOutput `json:"entries"`
}

func BuildQueryMonthTool(ledger interfaces.TransactionsLedger) tool.ToolHandle {
	in := llm.Schema{
		Name:   "query_month_input",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"refMonth": map[string]any{"type": "string"},
				"cursor":   map[string]any{"type": "string"},
				"limit":    map[string]any{"type": "integer"},
			},
			"required":             []string{},
			"additionalProperties": false,
		},
	}
	out := llm.Schema{
		Name:   "query_month_output",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"refMonth":     map[string]any{"type": "string"},
				"incomeCents":  map[string]any{"type": "integer"},
				"outcomeCents": map[string]any{"type": "integer"},
				"totalCents":   map[string]any{"type": "integer"},
				"entries":      map[string]any{"type": "array"},
			},
			"required":             []string{"refMonth", "incomeCents", "outcomeCents", "totalCents", "entries"},
			"additionalProperties": false,
		},
	}
	return tool.NewTool[QueryMonthInput, QueryMonthOutput]("query_month", "Consulta o resumo e os lançamentos do mês financeiro do usuário.", in, out, buildQueryMonthExec(ledger))
}

func buildQueryMonthExec(ledger interfaces.TransactionsLedger) func(context.Context, QueryMonthInput) (QueryMonthOutput, error) {
	return func(ctx context.Context, in QueryMonthInput) (QueryMonthOutput, error) {
		resourceID, _, _, ok := agent.InboundIdentityFromContext(ctx)
		if !ok {
			return QueryMonthOutput{}, fmt.Errorf("query_month: identidade não disponível no contexto")
		}
		userID, err := uuid.Parse(resourceID)
		if err != nil {
			return QueryMonthOutput{}, fmt.Errorf("query_month: userId inválido: %w", err)
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
			limit = 50
		}
		summary, err := ledger.GetMonthlySummary(ctx, userID, refMonth)
		if err != nil {
			return QueryMonthOutput{}, fmt.Errorf("query_month: resumo mensal: %w", err)
		}
		entries, err := ledger.ListMonthlyEntries(ctx, userID, refMonth, in.Cursor, limit)
		if err != nil {
			return QueryMonthOutput{}, fmt.Errorf("query_month: lançamentos mensais: %w", err)
		}
		mapped := make([]QueryMonthEntryOutput, len(entries))
		for i, e := range entries {
			mapped[i] = QueryMonthEntryOutput{
				Kind:        e.Kind,
				ID:          e.ID,
				RefMonth:    e.RefMonth,
				AmountCents: e.AmountCents,
				Direction:   e.Direction,
				Description: e.Description,
				CreatedAt:   e.CreatedAt,
			}
		}
		return QueryMonthOutput{
			RefMonth:     summary.RefMonth,
			IncomeCents:  summary.IncomeCents,
			OutcomeCents: summary.OutcomeCents,
			TotalCents:   summary.TotalCents,
			Entries:      mapped,
		}, nil
	}
}
