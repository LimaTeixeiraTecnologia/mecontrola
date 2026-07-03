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

type CreateRecurrenceInput struct {
	Direction     string  `json:"direction"`
	PaymentMethod string  `json:"paymentMethod"`
	AmountCents   int64   `json:"amountCents"`
	Description   string  `json:"description"`
	CategoryID    string  `json:"categoryId"`
	SubcategoryID *string `json:"subcategoryId,omitempty"`
	CardID        *string `json:"cardId,omitempty"`
	Frequency     string  `json:"frequency"`
	DayOfMonth    int     `json:"dayOfMonth"`
	StartedAt     string  `json:"startedAt,omitempty"`
}

type CreateRecurrenceOutput struct {
	ResourceID string `json:"resourceId"`
	Kind       string `json:"kind"`
	IsReplay   bool   `json:"isReplay"`
	Outcome    string `json:"outcome"`
}

func BuildCreateRecurrenceTool(recurrences interfaces.RecurrenceManager, writer idempotentWriter) tool.ToolHandle {
	in := llm.Schema{
		Name:   "create_recurrence_input",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"direction":     map[string]any{"type": "string"},
				"paymentMethod": map[string]any{"type": "string"},
				"amountCents":   map[string]any{"type": "integer"},
				"description":   map[string]any{"type": "string"},
				"categoryId":    map[string]any{"type": "string"},
				"subcategoryId": map[string]any{"type": "string"},
				"cardId":        map[string]any{"type": "string"},
				"frequency":     map[string]any{"type": "string"},
				"dayOfMonth":    map[string]any{"type": "integer"},
				"startedAt":     map[string]any{"type": "string"},
			},
			"required":             []string{"direction", "paymentMethod", "amountCents", "description", "categoryId", "frequency", "dayOfMonth"},
			"additionalProperties": false,
		},
	}
	out := llm.Schema{
		Name:   "create_recurrence_output",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"resourceId": map[string]any{"type": "string"},
				"kind":       map[string]any{"type": "string"},
				"isReplay":   map[string]any{"type": "boolean"},
				"outcome":    map[string]any{"type": "string"},
			},
			"required":             []string{"resourceId", "kind", "isReplay", "outcome"},
			"additionalProperties": false,
		},
	}
	return tool.NewTool[CreateRecurrenceInput, CreateRecurrenceOutput]("create_recurrence", "Cria um template de recorrência financeira para o usuário.", in, out, buildCreateRecurrenceExec(recurrences, writer))
}

func buildCreateRecurrenceExec(recurrences interfaces.RecurrenceManager, writer idempotentWriter) func(context.Context, CreateRecurrenceInput) (CreateRecurrenceOutput, error) {
	return func(ctx context.Context, in CreateRecurrenceInput) (CreateRecurrenceOutput, error) {
		resourceID, wamid, itemSeq, ok := agent.InboundIdentityFromContext(ctx)
		if !ok {
			return CreateRecurrenceOutput{}, fmt.Errorf("create_recurrence: identidade não disponível no contexto")
		}
		userID, err := uuid.Parse(resourceID)
		if err != nil {
			return CreateRecurrenceOutput{}, fmt.Errorf("create_recurrence: userId inválido: %w", err)
		}
		catID, err := uuid.Parse(in.CategoryID)
		if err != nil {
			return CreateRecurrenceOutput{}, fmt.Errorf("create_recurrence: categoryId inválido: %w", err)
		}
		startedAt := in.StartedAt
		if startedAt == "" {
			loc, locErr := time.LoadLocation("America/Sao_Paulo")
			if locErr != nil {
				loc = time.UTC
			}
			startedAt = time.Now().In(loc).Format("2006-01-02")
		}
		var subCatID *uuid.UUID
		if in.SubcategoryID != nil {
			parsed, parseErr := uuid.Parse(*in.SubcategoryID)
			if parseErr != nil {
				return CreateRecurrenceOutput{}, fmt.Errorf("create_recurrence: subcategoryId inválido: %w", parseErr)
			}
			subCatID = &parsed
		}
		var cardID *uuid.UUID
		if in.CardID != nil {
			parsed, parseErr := uuid.Parse(*in.CardID)
			if parseErr != nil {
				return CreateRecurrenceOutput{}, fmt.Errorf("create_recurrence: cardId inválido: %w", parseErr)
			}
			cardID = &parsed
		}
		result, writeErr := writer.Execute(ctx, userID, wamid, itemSeq, "create_recurrence", "recurring_template", func(ctx context.Context) (uuid.UUID, bool, error) {
			ref, refErr := recurrences.CreateRecurrence(ctx, interfaces.RawRecurrence{
				UserID:        userID,
				Direction:     in.Direction,
				PaymentMethod: in.PaymentMethod,
				CardID:        cardID,
				AmountCents:   in.AmountCents,
				Description:   in.Description,
				CategoryID:    catID,
				SubcategoryID: subCatID,
				Frequency:     in.Frequency,
				DayOfMonth:    in.DayOfMonth,
				StartedAt:     startedAt,
			})
			if refErr != nil {
				return uuid.Nil, false, refErr
			}
			return ref.ID, ref.Reconciled, nil
		})
		if writeErr != nil {
			return CreateRecurrenceOutput{}, fmt.Errorf("create_recurrence: %w", writeErr)
		}
		return CreateRecurrenceOutput{
			ResourceID: result.ResourceID.String(),
			Kind:       "recurring_template",
			IsReplay:   result.Outcome == agent.ToolOutcomeReplay,
			Outcome:    result.Outcome.String(),
		}, nil
	}
}
