package tools

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	budgetsvo "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/tool"
)

const (
	queryMonthOutcomeOK      = "ok"
	queryMonthOutcomeClarify = "clarify"
)

type QueryMonthInput struct {
	MonthRefKind string `json:"monthRefKind,omitempty"`
	Year         int    `json:"year,omitempty"`
	Month        int    `json:"month,omitempty"`
	Cursor       string `json:"cursor,omitempty"`
	Limit        int    `json:"limit,omitempty"`
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
	Outcome       string                  `json:"outcome"`
	RefMonth      string                  `json:"refMonth"`
	IncomeCents   int64                   `json:"incomeCents"`
	OutcomeCents  int64                   `json:"outcomeCents"`
	TotalCents    int64                   `json:"totalCents"`
	Entries       []QueryMonthEntryOutput `json:"entries"`
	ClarifyPrompt string                  `json:"clarifyPrompt,omitempty"`
}

func BuildQueryMonthTool(ledger interfaces.TransactionsLedger) tool.ToolHandle {
	in := llm.Schema{
		Name:   "query_month_input",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"monthRefKind": map[string]any{
					"type":        "string",
					"enum":        []string{"current", "previous", "next", "explicit", "named_without_year", "unknown"},
					"description": "Classificação da referência de mês citada pelo usuário. Use named_without_year sempre que um nome de mês (ex.: junho, março) for citado SEM um ano junto — mesmo que esse mês já tenha passado no ano corrente. Use explicit sempre que o usuário citar mês E ano juntos. NUNCA use current quando um nome de mês foi citado, mesmo sem ano.",
				},
				"year":   map[string]any{"type": "integer", "description": "Ano numérico, apenas quando o usuário citou explicitamente o ano junto ao mês (monthRefKind=explicit). Omitir para named_without_year."},
				"month":  map[string]any{"type": "integer", "minimum": 1, "maximum": 12, "description": "Mês numérico (1-12), quando monthRefKind=explicit ou named_without_year."},
				"cursor": map[string]any{"type": "string"},
				"limit":  map[string]any{"type": "integer"},
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
				"outcome":       map[string]any{"type": "string"},
				"refMonth":      map[string]any{"type": "string"},
				"incomeCents":   map[string]any{"type": "integer"},
				"outcomeCents":  map[string]any{"type": "integer"},
				"totalCents":    map[string]any{"type": "integer"},
				"entries":       map[string]any{"type": "array"},
				"clarifyPrompt": map[string]any{"type": "string"},
			},
			"required":             []string{"outcome", "refMonth", "incomeCents", "outcomeCents", "totalCents", "entries"},
			"additionalProperties": false,
		},
	}
	return tool.NewTool("query_month", "Consulta o resumo e os lançamentos do mês financeiro do usuário.", in, out, buildQueryMonthExec(ledger))
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

		competence, clarifyReason, err := resolveCompetenceReference(in.MonthRefKind, in.Year, in.Month)
		if err != nil {
			return QueryMonthOutput{}, fmt.Errorf("query_month: resolver competência: %w", err)
		}
		if clarifyReason != budgetsvo.ClarifyNone {
			return QueryMonthOutput{
				Outcome:       queryMonthOutcomeClarify,
				ClarifyPrompt: competenceReferenceClarifyPrompt(clarifyReason),
			}, nil
		}
		refMonth := competence.String()
		if refMonth == "" {
			refMonth = currentCompetenceFallback()
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
				Kind:        e.Kind.String(),
				ID:          e.ID,
				RefMonth:    e.RefMonth,
				AmountCents: e.AmountCents,
				Direction:   e.Direction,
				Description: e.Description,
				CreatedAt:   e.CreatedAt,
			}
		}
		return QueryMonthOutput{
			Outcome:      queryMonthOutcomeOK,
			RefMonth:     summary.RefMonth,
			IncomeCents:  summary.IncomeCents,
			OutcomeCents: summary.OutcomeCents,
			TotalCents:   summary.TotalCents,
			Entries:      mapped,
		}, nil
	}
}
