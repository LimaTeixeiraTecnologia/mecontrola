package tools

import (
	"context"
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

type DeleteEntryInput struct {
	EntryID   string `json:"entryId"`
	EntryKind string `json:"entryKind"`
	Version   int64  `json:"version"`
}

type DeleteEntryOutput struct {
	NeedsConfirmation bool   `json:"needsConfirmation"`
	ImpactNote        string `json:"impactNote"`
	TargetRef         string `json:"targetRef"`
	TargetKind        string `json:"targetKind"`
}

func BuildDeleteEntryTool(engine wf.Engine[workflows.ConfirmState], def wf.Definition[workflows.ConfirmState], cards interfaces.CardManager) tool.ToolHandle {
	in := llm.Schema{
		Name:   "delete_entry_input",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"entryId":   map[string]any{"type": "string"},
				"entryKind": map[string]any{"type": "string"},
				"version":   map[string]any{"type": "integer"},
			},
			"required":             []string{"entryId", "entryKind", "version"},
			"additionalProperties": false,
		},
	}
	out := llm.Schema{
		Name:   "delete_entry_output",
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
	exec := buildDeleteEntryExec(engine, def, cards)
	return tool.NewVerbatimTool("delete_entry", "Solicita confirmação do usuário para excluir um lançamento financeiro.", in, out, exec, extractDeleteEntryVerbatim)
}

func extractDeleteEntryVerbatim(o DeleteEntryOutput) (string, bool) {
	return o.ImpactNote, o.NeedsConfirmation && o.ImpactNote != ""
}

func buildDeleteEntryExec(engine wf.Engine[workflows.ConfirmState], def wf.Definition[workflows.ConfirmState], cards interfaces.CardManager) func(context.Context, DeleteEntryInput) (DeleteEntryOutput, error) {
	return func(ctx context.Context, in DeleteEntryInput) (DeleteEntryOutput, error) {
		rc, ok := wf.RuntimeFrom(ctx)
		if !ok {
			return DeleteEntryOutput{}, fmt.Errorf("agents.tool.delete_entry: inbound request ausente no contexto")
		}
		req, ok := rc.(agent.InboundRequest)
		if !ok {
			return DeleteEntryOutput{}, fmt.Errorf("agents.tool.delete_entry: tipo de runtime inválido")
		}

		userID, err := uuid.Parse(req.ResourceID)
		if err != nil {
			return DeleteEntryOutput{}, fmt.Errorf("agents.tool.delete_entry: parse resource uuid: %w", err)
		}

		opKind := workflows.OpDeleteEntry
		if in.EntryKind == "card" {
			opKind = workflows.OpDeleteCard
		}

		impactNote := workflows.BuildImpactNote(ctx, in.EntryID, in.EntryKind, userID, cards)

		state := workflows.ConfirmState{
			Awaiting:    workflows.AwaitingConfirm,
			Operation:   opKind,
			TargetRef:   in.EntryID,
			TargetKind:  in.EntryKind,
			ImpactNote:  impactNote,
			MessageID:   req.MessageID,
			SuspendedAt: time.Now().UTC(),
			UserID:      userID,
			Version:     in.Version,
		}

		key := workflows.DestructiveConfirmKey(req.ResourceID)
		_, err = engine.Start(ctx, def, key, state)
		if err != nil && !errors.Is(err, wf.ErrRunAlreadyExists) {
			return DeleteEntryOutput{}, fmt.Errorf("agents.tool.delete_entry: iniciar confirmação: %w", err)
		}
		if errors.Is(err, wf.ErrRunAlreadyExists) {
			return DeleteEntryOutput{
				NeedsConfirmation: true,
				ImpactNote:        "Há uma confirmação pendente. Por favor, responda sim ou não antes de solicitar outra operação.",
				TargetRef:         in.EntryID,
				TargetKind:        in.EntryKind,
			}, nil
		}

		return DeleteEntryOutput{
			NeedsConfirmation: true,
			ImpactNote:        impactNote,
			TargetRef:         in.EntryID,
			TargetKind:        in.EntryKind,
		}, nil
	}
}
