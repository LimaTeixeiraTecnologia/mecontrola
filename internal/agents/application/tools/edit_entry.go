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

type EditEntryInput struct {
	EntryID     string `json:"entryId"`
	AmountCents int64  `json:"amountCents,omitempty"`
	Description string `json:"description,omitempty"`
	OccurredAt  string `json:"occurredAt,omitempty"`
}

type EditEntryOutput struct {
	NeedsConfirmation bool   `json:"needsConfirmation"`
	ImpactNote        string `json:"impactNote"`
	TargetRef         string `json:"targetRef"`
	Outcome           string `json:"outcome"`
}

func BuildEditEntryTool(editor entryEditor) tool.ToolHandle {
	in := llm.Schema{
		Name:   "edit_entry_input",
		Strict: false,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"entryId":     map[string]any{"type": "string"},
				"amountCents": map[string]any{"type": "integer"},
				"description": map[string]any{"type": "string"},
				"occurredAt":  map[string]any{"type": "string"},
			},
			"required":             []string{"entryId"},
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
				"outcome":           map[string]any{"type": "string"},
			},
			"required":             []string{"needsConfirmation", "impactNote", "targetRef", "outcome"},
			"additionalProperties": false,
		},
	}
	return tool.NewTool[EditEntryInput, EditEntryOutput]("edit_entry", "Solicita a edição de um lançamento financeiro; a persistência só ocorre após confirmação explícita do usuário.", in, out, buildEditEntryExec(editor))
}

func buildEditEntryExec(editor entryEditor) func(context.Context, EditEntryInput) (EditEntryOutput, error) {
	return func(ctx context.Context, in EditEntryInput) (EditEntryOutput, error) {
		resourceID, threadID, wamid, itemSeq, ok := agent.InboundExecutionFromContext(ctx)
		if !ok {
			return EditEntryOutput{}, fmt.Errorf("agents.tool.edit_entry: identidade não disponível no contexto")
		}

		userID, err := uuid.Parse(resourceID)
		if err != nil {
			return EditEntryOutput{}, fmt.Errorf("agents.tool.edit_entry: parse resource uuid: %w", err)
		}

		targetID, err := uuid.Parse(in.EntryID)
		if err != nil {
			return EditEntryOutput{}, fmt.Errorf("agents.tool.edit_entry: parse entry uuid: %w", err)
		}

		result, err := editor.EditEntry(ctx, usecases.EditEntryCommand{
			UserID:              userID,
			ThreadID:            threadID,
			WAMID:               wamid,
			ItemSeq:             itemSeq,
			TargetTransactionID: targetID,
			AmountCents:         in.AmountCents,
			Description:         in.Description,
			OccurredAt:          in.OccurredAt,
		})
		if err != nil {
			return EditEntryOutput{}, fmt.Errorf("agents.tool.edit_entry: %w", err)
		}

		impact := result.Message
		if impact == "" {
			impact = "Este lançamento será atualizado com os novos dados. Por favor confirme."
		}
		return EditEntryOutput{
			NeedsConfirmation: true,
			ImpactNote:        impact,
			TargetRef:         in.EntryID,
			Outcome:           result.Outcome.String(),
		}, nil
	}
}
