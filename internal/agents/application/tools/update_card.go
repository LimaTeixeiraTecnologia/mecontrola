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

type UpdateCardInput struct {
	CardID   string  `json:"cardId"`
	Version  int64   `json:"version"`
	Nickname *string `json:"nickname,omitempty"`
	Bank     *string `json:"bank,omitempty"`
	DueDay   *int    `json:"dueDay,omitempty"`
}

type UpdateCardOutput struct {
	NeedsConfirmation bool   `json:"needsConfirmation"`
	Executed          bool   `json:"executed"`
	ImpactNote        string `json:"impactNote"`
	TargetRef         string `json:"targetRef"`
	TargetKind        string `json:"targetKind"`
}

func BuildUpdateCardTool(engine wf.Engine[workflows.ConfirmState], def wf.Definition[workflows.ConfirmState], cards interfaces.CardManager) tool.ToolHandle {
	in := llm.Schema{
		Name:   "update_card_input",
		Strict: false,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"cardId":   map[string]any{"type": "string"},
				"version":  map[string]any{"type": "integer"},
				"nickname": map[string]any{"type": "string"},
				"bank":     map[string]any{"type": "string"},
				"dueDay":   map[string]any{"type": "integer"},
			},
			"required":             []string{"cardId", "version"},
			"additionalProperties": false,
		},
	}
	out := llm.Schema{
		Name:   "update_card_output",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"needsConfirmation": map[string]any{"type": "boolean"},
				"executed":          map[string]any{"type": "boolean"},
				"impactNote":        map[string]any{"type": "string"},
				"targetRef":         map[string]any{"type": "string"},
				"targetKind":        map[string]any{"type": "string"},
			},
			"required":             []string{"needsConfirmation", "executed", "impactNote", "targetRef", "targetKind"},
			"additionalProperties": false,
		},
	}
	exec := buildUpdateCardExec(engine, def, cards)
	return tool.NewVerbatimTool("update_card", "Atualiza os dados de um cartão. Requer confirmação quando o dia de vencimento é alterado.", in, out, exec, extractUpdateCardVerbatim)
}

func extractUpdateCardVerbatim(o UpdateCardOutput) (string, bool) {
	return o.ImpactNote, o.NeedsConfirmation && o.ImpactNote != ""
}

func buildUpdateCardExec(engine wf.Engine[workflows.ConfirmState], def wf.Definition[workflows.ConfirmState], cards interfaces.CardManager) func(context.Context, UpdateCardInput) (UpdateCardOutput, error) {
	return func(ctx context.Context, in UpdateCardInput) (UpdateCardOutput, error) {
		rc, ok := wf.RuntimeFrom(ctx)
		if !ok {
			return UpdateCardOutput{}, fmt.Errorf("agents.tool.update_card: inbound request ausente no contexto")
		}
		req, ok := rc.(agent.InboundRequest)
		if !ok {
			return UpdateCardOutput{}, fmt.Errorf("agents.tool.update_card: tipo de runtime inválido")
		}

		userID, err := uuid.Parse(req.ResourceID)
		if err != nil {
			return UpdateCardOutput{}, fmt.Errorf("agents.tool.update_card: parse resource uuid: %w", err)
		}

		cardID, err := uuid.Parse(in.CardID)
		if err != nil {
			return UpdateCardOutput{}, fmt.Errorf("agents.tool.update_card: parse card uuid: %w", err)
		}

		if !cardUpdateNeedsConfirmation(in) {
			upd := interfaces.CardUpdate{
				ID:       cardID,
				UserID:   userID,
				Nickname: in.Nickname,
				Bank:     in.Bank,
			}
			_, err = cards.UpdateCard(ctx, upd)
			if err != nil {
				return UpdateCardOutput{}, fmt.Errorf("agents.tool.update_card: update direto: %w", err)
			}
			return UpdateCardOutput{
				NeedsConfirmation: false,
				Executed:          true,
				ImpactNote:        "Cartão atualizado.",
				TargetRef:         in.CardID,
				TargetKind:        "card",
			}, nil
		}

		upd := interfaces.CardUpdate{
			Nickname: in.Nickname,
			Bank:     in.Bank,
			DueDay:   in.DueDay,
		}
		updatePayloadBytes, err := json.Marshal(upd)
		if err != nil {
			return UpdateCardOutput{}, fmt.Errorf("agents.tool.update_card: marshal payload: %w", err)
		}

		impactNote := "A alteração do dia de vencimento pode impactar parcelas em aberto."

		state := workflows.ConfirmState{
			Awaiting:      workflows.AwaitingConfirm,
			Operation:     workflows.OpUpdateCard,
			TargetRef:     in.CardID,
			TargetKind:    "card",
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
			return UpdateCardOutput{}, fmt.Errorf("agents.tool.update_card: iniciar confirmação: %w", err)
		}
		if errors.Is(err, wf.ErrRunAlreadyExists) {
			return UpdateCardOutput{
				NeedsConfirmation: true,
				Executed:          false,
				ImpactNote:        "Há uma confirmação pendente. Por favor, responda sim ou não antes de solicitar outra operação.",
				TargetRef:         in.CardID,
				TargetKind:        "card",
			}, nil
		}

		return UpdateCardOutput{
			NeedsConfirmation: true,
			Executed:          false,
			ImpactNote:        impactNote,
			TargetRef:         in.CardID,
			TargetKind:        "card",
		}, nil
	}
}

func cardUpdateNeedsConfirmation(in UpdateCardInput) bool {
	return in.DueDay != nil
}
