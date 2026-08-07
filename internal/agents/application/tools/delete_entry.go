package tools

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/messages"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/workflows"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/tool"
	wf "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

type DeleteEntryInput struct {
	EntryID           string `json:"entryId,omitempty"`
	EntryKind         string `json:"entryKind"`
	Version           int64  `json:"version"`
	SearchAmountCents int64  `json:"searchAmountCents,omitempty"`
	SearchTerm        string `json:"searchTerm,omitempty"`
}

type DeleteEntryOutput struct {
	NeedsConfirmation bool   `json:"needsConfirmation"`
	ImpactNote        string `json:"impactNote"`
	TargetRef         string `json:"targetRef"`
	TargetKind        string `json:"targetKind"`
}

func BuildDeleteEntryTool(engine wf.Engine[workflows.DestructiveManageState], def wf.Definition[workflows.DestructiveManageState], cards interfaces.CardManager) tool.ToolHandle {
	in := llm.Schema{
		Name:   "delete_entry_input",
		Strict: false,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"entryId":           map[string]any{"type": "string", "description": "Id do lançamento ou cartão 💳 a excluir, quando já conhecido (ex.: cardId retornado por resolve_card). Se ausente, para lançamentos use searchAmountCents/searchTerm para localizar o item; NUNCA invente um valor."},
				"entryKind":         map[string]any{"type": "string"},
				"version":           map[string]any{"type": "integer"},
				"searchAmountCents": map[string]any{"type": "integer", "description": "Valor do lançamento a excluir, usado só como critério de busca quando entryId é desconhecido (ignorado para entryKind=card)."},
				"searchTerm":        map[string]any{"type": "string", "description": "Termo (descrição atual ou apelido do item) usado só como critério de busca quando entryId é desconhecido (ignorado para entryKind=card)."},
			},
			"required":             []string{"entryKind", "version"},
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
	return tool.NewVerbatimTool("delete_entry", "Solicita confirmação do usuário para excluir um lançamento financeiro ou um cartão 💳.", in, out, exec, extractDeleteEntryVerbatim)
}

func extractDeleteEntryVerbatim(o DeleteEntryOutput) (string, bool) {
	return o.ImpactNote, o.NeedsConfirmation && o.ImpactNote != ""
}

func buildDeleteEntryExec(engine wf.Engine[workflows.DestructiveManageState], def wf.Definition[workflows.DestructiveManageState], cards interfaces.CardManager) func(context.Context, DeleteEntryInput) (DeleteEntryOutput, error) {
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

		opKind := workflows.DestructiveOpDeleteEntry
		if in.EntryKind == "card" {
			opKind = workflows.DestructiveOpDeleteCard
		}

		targetRef := ""
		impactNote := ""
		if _, parseErr := uuid.Parse(in.EntryID); parseErr == nil {
			targetRef = in.EntryID
			impactNote = workflows.BuildDestructiveManageImpactNote(ctx, in.EntryID, in.EntryKind, userID, cards)
		}

		state := workflows.DestructiveManageState{
			Status:            workflows.DestructiveManageActive,
			Operation:         opKind,
			UserID:            userID,
			TargetRef:         targetRef,
			ImpactNote:        impactNote,
			MessageID:         req.MessageID,
			SuspendedAt:       time.Now().UTC(),
			Version:           in.Version,
			SearchAmountCents: in.SearchAmountCents,
			SearchTerm:        in.SearchTerm,
		}

		key := workflows.DestructiveManageKey(req.ResourceID, req.ThreadID)
		result, err := engine.Start(ctx, def, key, state)
		if err != nil && !errors.Is(err, wf.ErrRunAlreadyExists) {
			return DeleteEntryOutput{}, fmt.Errorf("agents.tool.delete_entry: iniciar confirmação: %w", err)
		}
		if errors.Is(err, wf.ErrRunAlreadyExists) {
			return DeleteEntryOutput{
				NeedsConfirmation: true,
				ImpactNote:        messages.PendingConfirmationExists(),
				TargetRef:         in.EntryID,
				TargetKind:        in.EntryKind,
			}, nil
		}

		if targetRef != "" {
			return DeleteEntryOutput{
				NeedsConfirmation: true,
				ImpactNote:        workflows.DestructiveManageConfirmQuestionText(impactNote),
				TargetRef:         targetRef,
				TargetKind:        in.EntryKind,
			}, nil
		}

		return DeleteEntryOutput{
			NeedsConfirmation: true,
			ImpactNote:        result.State.ResponseText,
			TargetRef:         result.State.TargetRef,
			TargetKind:        in.EntryKind,
		}, nil
	}
}
