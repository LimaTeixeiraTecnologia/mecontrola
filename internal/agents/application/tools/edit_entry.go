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

type EditEntryInput struct {
	EntryID           string `json:"entryId"`
	EntryKind         string `json:"entryKind"`
	Version           int64  `json:"version"`
	AmountCents       int64  `json:"amountCents,omitempty"`
	Description       string `json:"description,omitempty"`
	OccurredAt        string `json:"occurredAt,omitempty"`
	InstallmentsTotal int    `json:"installmentsTotal,omitempty"`
}

type EditEntryOutput struct {
	NeedsConfirmation bool   `json:"needsConfirmation"`
	ImpactNote        string `json:"impactNote"`
	TargetRef         string `json:"targetRef"`
	TargetKind        string `json:"targetKind"`
}

func BuildEditEntryTool(engine wf.Engine[workflows.ConfirmState], def wf.Definition[workflows.ConfirmState]) tool.ToolHandle {
	in := llm.Schema{
		Name:   "edit_entry_input",
		Strict: false,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"entryId":           map[string]any{"type": "string"},
				"entryKind":         map[string]any{"type": "string"},
				"version":           map[string]any{"type": "integer"},
				"amountCents":       map[string]any{"type": "integer"},
				"description":       map[string]any{"type": "string"},
				"occurredAt":        map[string]any{"type": "string"},
				"installmentsTotal": map[string]any{"type": "integer"},
			},
			"required":             []string{"entryId", "entryKind", "version"},
			"additionalProperties": false,
		},
	}
	out := llm.Schema{
		Name:   "edit_entry_output",
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
	exec := buildEditEntryExec(engine, def)
	return tool.NewTool[EditEntryInput, EditEntryOutput]("edit_entry", "Solicita confirmação do usuário para editar um lançamento financeiro.", in, out, exec)
}

func buildEditEntryExec(engine wf.Engine[workflows.ConfirmState], def wf.Definition[workflows.ConfirmState]) func(context.Context, EditEntryInput) (EditEntryOutput, error) {
	return func(ctx context.Context, in EditEntryInput) (EditEntryOutput, error) {
		rc, ok := wf.RuntimeFrom(ctx)
		if !ok {
			return EditEntryOutput{}, fmt.Errorf("agents.tool.edit_entry: inbound request ausente no contexto")
		}
		req, ok := rc.(agent.InboundRequest)
		if !ok {
			return EditEntryOutput{}, fmt.Errorf("agents.tool.edit_entry: tipo de runtime inválido")
		}

		userID, err := uuid.Parse(req.ResourceID)
		if err != nil {
			return EditEntryOutput{}, fmt.Errorf("agents.tool.edit_entry: parse resource uuid: %w", err)
		}

		updatePayload, err := buildUpdatePayload(in)
		if err != nil {
			return EditEntryOutput{}, fmt.Errorf("agents.tool.edit_entry: build payload: %w", err)
		}

		impactNote := "Este lançamento será atualizado com os novos dados."

		state := workflows.ConfirmState{
			Awaiting:      workflows.AwaitingConfirm,
			Operation:     workflows.OpEditEntry,
			TargetRef:     in.EntryID,
			TargetKind:    in.EntryKind,
			ImpactNote:    impactNote,
			MessageID:     req.MessageID,
			SuspendedAt:   time.Now().UTC(),
			UpdatePayload: updatePayload,
			UserID:        userID,
			Version:       in.Version,
		}

		key := workflows.DestructiveConfirmKey(req.ResourceID)
		_, err = engine.Start(ctx, def, key, state)
		if err != nil && !errors.Is(err, wf.ErrRunAlreadyExists) {
			return EditEntryOutput{}, fmt.Errorf("agents.tool.edit_entry: iniciar confirmação: %w", err)
		}
		if errors.Is(err, wf.ErrRunAlreadyExists) {
			return EditEntryOutput{
				NeedsConfirmation: true,
				ImpactNote:        "Há uma confirmação pendente. Por favor, responda sim ou não antes de solicitar outra operação.",
				TargetRef:         in.EntryID,
				TargetKind:        in.EntryKind,
			}, nil
		}

		return EditEntryOutput{
			NeedsConfirmation: true,
			ImpactNote:        impactNote,
			TargetRef:         in.EntryID,
			TargetKind:        in.EntryKind,
		}, nil
	}
}

func buildUpdatePayload(in EditEntryInput) (string, error) {
	entryID, err := uuid.Parse(in.EntryID)
	if err != nil {
		return "", fmt.Errorf("parse entry uuid: %w", err)
	}
	payload := interfaces.RawUpdateTransaction{
		ID:          entryID,
		AmountCents: in.AmountCents,
		Description: in.Description,
		OccurredAt:  in.OccurredAt,
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal payload: %w", err)
	}
	return string(b), nil
}
