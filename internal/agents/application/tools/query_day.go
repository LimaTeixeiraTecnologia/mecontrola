package tools

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/money"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/tool"
)

const (
	queryDayOutcomeOK = "ok"
	dayRefToday       = "today"
	dayRefYesterday   = "yesterday"
)

type QueryDayInput struct {
	DayRefKind string `json:"dayRefKind,omitempty"`
}

type QueryDayOutput struct {
	Outcome      string                  `json:"outcome"`
	Day          string                  `json:"day"`
	IncomeCents  int64                   `json:"incomeCents"`
	IncomeBRL    string                  `json:"incomeBRL"`
	OutcomeCents int64                   `json:"outcomeCents"`
	OutcomeBRL   string                  `json:"outcomeBRL"`
	TotalCents   int64                   `json:"totalCents"`
	TotalBRL     string                  `json:"totalBRL"`
	Entries      []QueryMonthEntryOutput `json:"entries"`
}

func BuildQueryDayTool(ledger interfaces.TransactionsLedger) tool.ToolHandle {
	in := llm.Schema{
		Name:   "query_day_input",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"dayRefKind": map[string]any{
					"type":        "string",
					"enum":        []string{dayRefToday, dayRefYesterday},
					"description": "Dia de referência da consulta: today para hoje, yesterday para ontem. Omitir equivale a today.",
				},
			},
			"required":             []string{},
			"additionalProperties": false,
		},
	}
	out := llm.Schema{
		Name:   "query_day_output",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"outcome":      map[string]any{"type": "string"},
				"day":          map[string]any{"type": "string"},
				"incomeCents":  map[string]any{"type": "integer"},
				"incomeBRL":    map[string]any{"type": "string"},
				"outcomeCents": map[string]any{"type": "integer"},
				"outcomeBRL":   map[string]any{"type": "string"},
				"totalCents":   map[string]any{"type": "integer"},
				"totalBRL":     map[string]any{"type": "string"},
				"entries":      map[string]any{"type": "array"},
			},
			"required":             []string{"outcome", "day", "incomeCents", "incomeBRL", "outcomeCents", "outcomeBRL", "totalCents", "totalBRL", "entries"},
			"additionalProperties": false,
		},
	}
	return tool.NewTool("query_day", "Consulta o total gasto, o total recebido e os lançamentos de um dia específico (hoje ou ontem) do usuário; é a ÚNICA fonte confiável para perguntas como \"quanto gastei hoje?\" ou \"quanto gastei ontem?\"; nunca responda essas perguntas de memória ou a partir de query_month.", in, out, buildQueryDayExec(ledger))
}

func buildQueryDayExec(ledger interfaces.TransactionsLedger) func(context.Context, QueryDayInput) (QueryDayOutput, error) {
	return func(ctx context.Context, in QueryDayInput) (QueryDayOutput, error) {
		resourceID, _, _, ok := agent.InboundIdentityFromContext(ctx)
		if !ok {
			return QueryDayOutput{}, fmt.Errorf("query_day: identidade não disponível no contexto")
		}
		userID, err := uuid.Parse(resourceID)
		if err != nil {
			return QueryDayOutput{}, fmt.Errorf("query_day: userId inválido: %w", err)
		}

		now := time.Now().UTC()
		day := now.Format("2006-01-02")
		if in.DayRefKind == dayRefYesterday {
			day = now.Add(-24 * time.Hour).Format("2006-01-02")
		}

		summary, err := ledger.GetDaySummary(ctx, userID, day)
		if err != nil {
			return QueryDayOutput{}, fmt.Errorf("query_day: resumo do dia: %w", err)
		}
		entries := make([]QueryMonthEntryOutput, len(summary.Entries))
		for i, e := range summary.Entries {
			entries[i] = QueryMonthEntryOutput{
				Kind:        e.Kind.String(),
				ID:          e.ID,
				RefMonth:    e.RefMonth,
				AmountCents: e.AmountCents,
				AmountBRL:   money.FromCents(e.AmountCents).BRL(),
				Direction:   e.Direction,
				Description: e.Description,
				CreatedAt:   e.CreatedAt,
			}
		}
		return QueryDayOutput{
			Outcome:      queryDayOutcomeOK,
			Day:          summary.Day,
			IncomeCents:  summary.IncomeCents,
			IncomeBRL:    money.FromCents(summary.IncomeCents).BRL(),
			OutcomeCents: summary.OutcomeCents,
			OutcomeBRL:   money.FromCents(summary.OutcomeCents).BRL(),
			TotalCents:   summary.TotalCents,
			TotalBRL:     money.FromCents(summary.TotalCents).BRL(),
			Entries:      entries,
		}, nil
	}
}
