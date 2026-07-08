package tools

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/tool"
)

type CreateRecurrenceInput struct {
	Direction     string `json:"direction"`
	PaymentMethod string `json:"paymentMethod"`
	AmountCents   int64  `json:"amountCents"`
	Description   string `json:"description"`
	CategoryID    string `json:"categoryId"`
	SubcategoryID string `json:"subcategoryId"`
	CardID        string `json:"cardId,omitempty"`
	Frequency     string `json:"frequency"`
	DayOfMonth    int    `json:"dayOfMonth"`
	StartedAt     string `json:"startedAt,omitempty"`
}

type CreateRecurrenceOutput struct {
	ResourceID string `json:"resourceId"`
	Kind       string `json:"kind"`
	IsReplay   bool   `json:"isReplay"`
	Outcome    string `json:"outcome"`
	Message    string `json:"message"`
}

func BuildCreateRecurrenceTool(registrar recurrenceRegistrar) tool.ToolHandle {
	in := llm.Schema{
		Name:   "create_recurrence_input",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"direction":     map[string]any{"type": "string", "enum": []string{"income", "outcome"}},
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
			"required":             []string{"direction", "paymentMethod", "amountCents", "description", "categoryId", "subcategoryId", "frequency", "dayOfMonth"},
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
				"message":    map[string]any{"type": "string"},
			},
			"required":             []string{"resourceId", "kind", "isReplay", "outcome", "message"},
			"additionalProperties": false,
		},
	}
	return tool.NewTool("create_recurrence", "Solicita a criação de um template de lançamento recorrente; a persistência só ocorre após confirmação explícita do usuário.", in, out, buildCreateRecurrenceExec(registrar))
}

func buildCreateRecurrenceExec(registrar recurrenceRegistrar) func(context.Context, CreateRecurrenceInput) (CreateRecurrenceOutput, error) {
	return func(ctx context.Context, in CreateRecurrenceInput) (CreateRecurrenceOutput, error) {
		resourceID, threadID, wamid, itemSeq, ok := agent.InboundExecutionFromContext(ctx)
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
		subCatID, err := uuid.Parse(in.SubcategoryID)
		if err != nil {
			return CreateRecurrenceOutput{}, fmt.Errorf("create_recurrence: subcategoryId inválido: %w", err)
		}
		var cardID *uuid.UUID
		if in.CardID != "" {
			parsed, parseErr := uuid.Parse(in.CardID)
			if parseErr != nil {
				return CreateRecurrenceOutput{}, fmt.Errorf("create_recurrence: cardId inválido: %w", parseErr)
			}
			cardID = &parsed
		}
		result, err := registrar.CreateRecurrence(ctx, usecases.CreateRecurrenceCommand{
			UserID:        userID,
			ThreadID:      threadID,
			WAMID:         wamid,
			ItemSeq:       itemSeq,
			Direction:     in.Direction,
			PaymentMethod: in.PaymentMethod,
			CardID:        cardID,
			AmountCents:   in.AmountCents,
			Description:   in.Description,
			CategoryID:    catID,
			SubcategoryID: subCatID,
			Frequency:     in.Frequency,
			DayOfMonth:    in.DayOfMonth,
			StartedAt:     in.StartedAt,
		})
		if err != nil {
			return CreateRecurrenceOutput{}, fmt.Errorf("create_recurrence: %w", err)
		}
		resource := ""
		if result.Outcome != agent.ToolOutcomeClarify {
			resource = result.ResourceID.String()
		}
		return CreateRecurrenceOutput{
			ResourceID: resource,
			Kind:       "recurring_template",
			IsReplay:   result.Outcome == agent.ToolOutcomeReplay,
			Outcome:    result.Outcome.String(),
			Message:    result.Message,
		}, nil
	}
}
