package tools

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/workflows"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/tool"
	wf "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

type DeleteRecurrenceInput struct {
	TemplateID string `json:"templateId"`
	Version    int64  `json:"version"`
}

type DeleteRecurrenceOutput struct {
	NeedsConfirmation bool   `json:"needsConfirmation"`
	ImpactNote        string `json:"impactNote"`
	TargetRef         string `json:"targetRef"`
	TargetKind        string `json:"targetKind"`
}

func BuildDeleteRecurrenceTool(engine wf.Engine[workflows.DestructiveManageState], def wf.Definition[workflows.DestructiveManageState]) tool.ToolHandle {
	in := llm.Schema{
		Name:   "delete_recurrence_input",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"templateId": map[string]any{"type": "string"},
				"version":    map[string]any{"type": "integer"},
			},
			"required":             []string{"templateId", "version"},
			"additionalProperties": false,
		},
	}
	out := llm.Schema{
		Name:   "delete_recurrence_output",
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
	exec := buildDeleteRecurrenceExec(engine, def)
	return tool.NewVerbatimTool("delete_recurrence", "Solicita confirmação do usuário para remover uma recorrência financeira.", in, out, exec, extractDeleteRecurrenceVerbatim)
}

func extractDeleteRecurrenceVerbatim(o DeleteRecurrenceOutput) (string, bool) {
	return o.ImpactNote, o.NeedsConfirmation && o.ImpactNote != ""
}

func buildDeleteRecurrenceExec(engine wf.Engine[workflows.DestructiveManageState], def wf.Definition[workflows.DestructiveManageState]) func(context.Context, DeleteRecurrenceInput) (DeleteRecurrenceOutput, error) {
	return func(ctx context.Context, in DeleteRecurrenceInput) (DeleteRecurrenceOutput, error) {
		rc, ok := wf.RuntimeFrom(ctx)
		if !ok {
			return DeleteRecurrenceOutput{}, fmt.Errorf("agents.tool.delete_recurrence: inbound request ausente no contexto")
		}
		req, ok := rc.(agent.InboundRequest)
		if !ok {
			return DeleteRecurrenceOutput{}, fmt.Errorf("agents.tool.delete_recurrence: tipo de runtime inválido")
		}

		userID, err := uuid.Parse(req.ResourceID)
		if err != nil {
			return DeleteRecurrenceOutput{}, fmt.Errorf("agents.tool.delete_recurrence: parse resource uuid: %w", err)
		}

		impactNote := "Esta recorrência será removida permanentemente."

		state := workflows.DestructiveManageState{
			Status:      workflows.DestructiveManageActive,
			Operation:   workflows.DestructiveOpDeleteRecurrence,
			TargetRef:   in.TemplateID,
			ImpactNote:  impactNote,
			MessageID:   req.MessageID,
			SuspendedAt: time.Now().UTC(),
			UserID:      userID,
			Version:     in.Version,
		}

		key := workflows.DestructiveManageKey(req.ResourceID, req.ThreadID)
		_, err = engine.Start(ctx, def, key, state)
		if err != nil && !errors.Is(err, wf.ErrRunAlreadyExists) {
			return DeleteRecurrenceOutput{}, fmt.Errorf("agents.tool.delete_recurrence: iniciar confirmação: %w", err)
		}
		if errors.Is(err, wf.ErrRunAlreadyExists) {
			return DeleteRecurrenceOutput{
				NeedsConfirmation: true,
				ImpactNote:        "Há uma confirmação pendente. Por favor, responda sim ou não antes de solicitar outra operação.",
				TargetRef:         in.TemplateID,
				TargetKind:        "recurring_template",
			}, nil
		}

		return DeleteRecurrenceOutput{
			NeedsConfirmation: true,
			ImpactNote:        impactNote,
			TargetRef:         in.TemplateID,
			TargetKind:        "recurring_template",
		}, nil
	}
}
