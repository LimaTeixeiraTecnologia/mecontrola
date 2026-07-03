package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/workflows"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/tool"
	wf "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

type UpdateRecurrenceInput struct {
	TemplateID    string  `json:"templateId"`
	Version       int64   `json:"version"`
	Direction     *string `json:"direction,omitempty"`
	PaymentMethod *string `json:"paymentMethod,omitempty"`
	AmountCents   *int64  `json:"amountCents,omitempty"`
	Description   *string `json:"description,omitempty"`
	CategoryID    *string `json:"categoryId,omitempty"`
	Frequency     *string `json:"frequency,omitempty"`
	DayOfMonth    *int    `json:"dayOfMonth,omitempty"`
	EndedAt       *string `json:"endedAt,omitempty"`
}

type UpdateRecurrenceOutput struct {
	NeedsConfirmation bool   `json:"needsConfirmation"`
	ImpactNote        string `json:"impactNote"`
	TargetRef         string `json:"targetRef"`
	TargetKind        string `json:"targetKind"`
}

func BuildUpdateRecurrenceTool(engine wf.Engine[workflows.ConfirmState], def wf.Definition[workflows.ConfirmState]) tool.ToolHandle {
	in := llm.Schema{
		Name:   "update_recurrence_input",
		Strict: false,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"templateId":    map[string]any{"type": "string"},
				"version":       map[string]any{"type": "integer"},
				"direction":     map[string]any{"type": "string"},
				"paymentMethod": map[string]any{"type": "string"},
				"amountCents":   map[string]any{"type": "integer"},
				"description":   map[string]any{"type": "string"},
				"categoryId":    map[string]any{"type": "string"},
				"frequency":     map[string]any{"type": "string"},
				"dayOfMonth":    map[string]any{"type": "integer"},
				"endedAt":       map[string]any{"type": "string"},
			},
			"required":             []string{"templateId", "version"},
			"additionalProperties": false,
		},
	}
	out := llm.Schema{
		Name:   "update_recurrence_output",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"needsConfirmation": map[string]any{"type": "boolean"},
				"impactNote":        map[string]any{"type": "string"},
				"targetRef":         map[string]any{"type": "string"},
				"targetKind":        map[string]any{"type": "string"},
			},
			"required":             []string{"needsConfirmation", "impactNote", "targetRef", "targetKind"},
			"additionalProperties": false,
		},
	}
	exec := buildUpdateRecurrenceExec(engine, def)
	return tool.NewTool[UpdateRecurrenceInput, UpdateRecurrenceOutput]("update_recurrence", "Solicita confirmação do usuário para atualizar uma recorrência financeira.", in, out, exec)
}

func buildUpdateRecurrenceExec(engine wf.Engine[workflows.ConfirmState], def wf.Definition[workflows.ConfirmState]) func(context.Context, UpdateRecurrenceInput) (UpdateRecurrenceOutput, error) {
	return func(ctx context.Context, in UpdateRecurrenceInput) (UpdateRecurrenceOutput, error) {
		rc, ok := wf.RuntimeFrom(ctx)
		if !ok {
			return UpdateRecurrenceOutput{}, fmt.Errorf("agents.tool.update_recurrence: inbound request ausente no contexto")
		}
		req, ok := rc.(agent.InboundRequest)
		if !ok {
			return UpdateRecurrenceOutput{}, fmt.Errorf("agents.tool.update_recurrence: tipo de runtime inválido")
		}

		userID, err := uuid.Parse(req.ResourceID)
		if err != nil {
			return UpdateRecurrenceOutput{}, fmt.Errorf("agents.tool.update_recurrence: parse resource uuid: %w", err)
		}

		upd := interfaces.RawUpdateRecurrence{
			Direction:     in.Direction,
			PaymentMethod: in.PaymentMethod,
			AmountCents:   in.AmountCents,
			Description:   in.Description,
			Frequency:     in.Frequency,
			DayOfMonth:    in.DayOfMonth,
			EndedAt:       in.EndedAt,
			Version:       in.Version,
		}
		if in.CategoryID != nil {
			catID, err := uuid.Parse(*in.CategoryID)
			if err != nil {
				return UpdateRecurrenceOutput{}, fmt.Errorf("agents.tool.update_recurrence: parse category uuid: %w", err)
			}
			upd.CategoryID = &catID
		}

		updatePayloadBytes, err := json.Marshal(upd)
		if err != nil {
			return UpdateRecurrenceOutput{}, fmt.Errorf("agents.tool.update_recurrence: marshal payload: %w", err)
		}

		impactNote := "Esta recorrência será atualizada."

		state := workflows.ConfirmState{
			Awaiting:      workflows.AwaitingConfirm,
			Operation:     workflows.OpUpdateRecurrence,
			TargetRef:     in.TemplateID,
			TargetKind:    "recurring_template",
			ImpactNote:    impactNote,
			MessageID:     req.MessageID,
			SuspendedAt:   time.Now().UTC(),
			UpdatePayload: string(updatePayloadBytes),
			UserID:        userID,
			Version:       in.Version,
		}

		key := workflows.DestructiveConfirmKey(req.ResourceID)
		_, err = engine.Start(ctx, def, key, state)
		if err != nil && !errors.Is(err, wf.ErrRunAlreadyExists) {
			return UpdateRecurrenceOutput{}, fmt.Errorf("agents.tool.update_recurrence: iniciar confirmação: %w", err)
		}
		if errors.Is(err, wf.ErrRunAlreadyExists) {
			return UpdateRecurrenceOutput{
				NeedsConfirmation: true,
				ImpactNote:        "Há uma confirmação pendente. Por favor, responda sim ou não antes de solicitar outra operação.",
				TargetRef:         in.TemplateID,
				TargetKind:        "recurring_template",
			}, nil
		}

		return UpdateRecurrenceOutput{
			NeedsConfirmation: true,
			ImpactNote:        impactNote,
			TargetRef:         in.TemplateID,
			TargetKind:        "recurring_template",
		}, nil
	}
}
