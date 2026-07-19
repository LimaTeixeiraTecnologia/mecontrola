package tools

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
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
	CategoryID    string `json:"categoryId,omitempty"`
	SubcategoryID string `json:"subcategoryId,omitempty"`
	CategoryText  string `json:"categoryText,omitempty"`
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

func BuildCreateRecurrenceTool(registrar recurrenceRegistrar, cards interfaces.CardManager) tool.ToolHandle {
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
				"categoryText":  map[string]any{"type": "string"},
				"cardId":        map[string]any{"type": "string"},
				"frequency":     map[string]any{"type": "string"},
				"dayOfMonth":    map[string]any{"type": "integer"},
				"startedAt":     map[string]any{"type": "string"},
			},
			"required":             []string{"direction", "paymentMethod", "amountCents", "description", "frequency", "dayOfMonth"},
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
	return tool.NewVerbatimTool("create_recurrence", "Solicita a criação de um template de lançamento recorrente; a persistência só ocorre após confirmação explícita do usuário.", in, out, buildCreateRecurrenceExec(registrar, cards), extractCreateRecurrenceVerbatim)
}

func extractCreateRecurrenceVerbatim(o CreateRecurrenceOutput) (string, bool) {
	return o.Message, o.Outcome == agent.ToolOutcomeClarify.String() && o.Message != ""
}

func buildCreateRecurrenceExec(registrar recurrenceRegistrar, cards interfaces.CardManager) func(context.Context, CreateRecurrenceInput) (CreateRecurrenceOutput, error) {
	return func(ctx context.Context, in CreateRecurrenceInput) (CreateRecurrenceOutput, error) {
		resourceID, threadID, wamid, itemSeq, ok := agent.InboundExecutionFromContext(ctx)
		if !ok {
			return CreateRecurrenceOutput{}, fmt.Errorf("create_recurrence: identidade não disponível no contexto")
		}
		userID, err := uuid.Parse(resourceID)
		if err != nil {
			return CreateRecurrenceOutput{}, fmt.Errorf("create_recurrence: userId inválido: %w", err)
		}
		catID := parseOptionalUUID(in.CategoryID)
		subCatID := parseOptionalUUID(in.SubcategoryID)
		if catID == uuid.Nil {
			subCatID = uuid.Nil
		}
		var cardID *uuid.UUID
		if in.CardID != "" {
			parsed, parseErr := uuid.Parse(in.CardID)
			if parseErr != nil {
				return CreateRecurrenceOutput{}, fmt.Errorf("create_recurrence: cardId inválido: %w", parseErr)
			}
			if _, getErr := cards.GetCard(ctx, parsed, userID); getErr != nil {
				if errors.Is(getErr, interfaces.ErrCardNotFound) {
					return CreateRecurrenceOutput{
						Outcome: agent.ToolOutcomeClarify.String(),
						Message: cardNotFoundClarifyMessage,
					}, nil
				}
				return CreateRecurrenceOutput{}, fmt.Errorf("create_recurrence: %w", getErr)
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
			CategoryText:  in.CategoryText,
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
