package tools

import (
	"context"
	"fmt"
	"time"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/tool"
)

type ListRecurrencesInput struct {
	ActiveOnly bool   `json:"activeOnly,omitempty"`
	Cursor     string `json:"cursor,omitempty"`
	Limit      int    `json:"limit,omitempty"`
}

type ListRecurrencesItemOutput struct {
	ID                   string     `json:"id"`
	Direction            string     `json:"direction"`
	PaymentMethod        string     `json:"paymentMethod"`
	AmountCents          int64      `json:"amountCents"`
	Description          string     `json:"description"`
	CategoryID           string     `json:"categoryId"`
	CategoryNameSnapshot string     `json:"categoryNameSnapshot"`
	Frequency            string     `json:"frequency"`
	DayOfMonth           int        `json:"dayOfMonth"`
	InstallmentsTotal    int        `json:"installmentsTotal"`
	StartedAt            time.Time  `json:"startedAt"`
	EndedAt              *time.Time `json:"endedAt,omitempty"`
}

type ListRecurrencesOutput struct {
	Recurrences []ListRecurrencesItemOutput `json:"recurrences"`
}

func BuildListRecurrencesTool(recurrences interfaces.RecurrenceManager) tool.ToolHandle {
	in := llm.Schema{
		Name:   "list_recurrences_input",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"activeOnly": map[string]any{"type": "boolean"},
				"cursor":     map[string]any{"type": "string"},
				"limit":      map[string]any{"type": "integer"},
			},
			"required":             []string{},
			"additionalProperties": false,
		},
	}
	out := llm.Schema{
		Name:   "list_recurrences_output",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"recurrences": map[string]any{"type": "array"},
			},
			"required":             []string{"recurrences"},
			"additionalProperties": false,
		},
	}
	return tool.NewTool("list_recurrences", "Lista as recorrências financeiras do usuário.", in, out, buildListRecurrencesExec(recurrences))
}

func buildListRecurrencesExec(recurrences interfaces.RecurrenceManager) func(context.Context, ListRecurrencesInput) (ListRecurrencesOutput, error) {
	return func(ctx context.Context, in ListRecurrencesInput) (ListRecurrencesOutput, error) {
		limit := in.Limit
		if limit <= 0 {
			limit = 50
		}
		result, err := recurrences.ListRecurrences(ctx, in.ActiveOnly, in.Cursor, limit)
		if err != nil {
			return ListRecurrencesOutput{}, fmt.Errorf("list_recurrences: %w", err)
		}
		out := make([]ListRecurrencesItemOutput, len(result))
		for i, r := range result {
			out[i] = ListRecurrencesItemOutput{
				ID:                   r.ID.String(),
				Direction:            r.Direction,
				PaymentMethod:        r.PaymentMethod,
				AmountCents:          r.AmountCents,
				Description:          r.Description,
				CategoryID:           r.CategoryID.String(),
				CategoryNameSnapshot: r.CategoryNameSnapshot,
				Frequency:            r.Frequency,
				DayOfMonth:           r.DayOfMonth,
				InstallmentsTotal:    r.InstallmentsTotal,
				StartedAt:            r.StartedAt,
				EndedAt:              r.EndedAt,
			}
		}
		return ListRecurrencesOutput{Recurrences: out}, nil
	}
}
