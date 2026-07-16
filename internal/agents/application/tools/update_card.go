package tools

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/workflows"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/tool"
	wf "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

const (
	updateCardOutcomeNeedsConfirmation         = "needs_confirmation"
	updateCardOutcomePendingConfirmationExists = "pending_confirmation_exists"
)

type UpdateCardInput struct {
	CardID   string  `json:"cardId"`
	Version  int64   `json:"version"`
	Nickname *string `json:"nickname,omitempty"`
	Bank     *string `json:"bank,omitempty"`
	DueDay   *int    `json:"dueDay,omitempty"`
}

type UpdateCardOutput struct {
	Outcome            string `json:"outcome"`
	ConfirmationPrompt string `json:"confirmationPrompt"`
}

func BuildUpdateCardTool(engine wf.Engine[workflows.CardManageState], def wf.Definition[workflows.CardManageState]) tool.ToolHandle {
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
				"outcome":            map[string]any{"type": "string"},
				"confirmationPrompt": map[string]any{"type": "string"},
			},
			"required":             []string{"outcome", "confirmationPrompt"},
			"additionalProperties": false,
		},
	}
	exec := buildUpdateCardExec(engine, def)
	return tool.NewTool[UpdateCardInput, UpdateCardOutput]("update_card", "Atualiza apelido, banco e/ou dia de vencimento de um 💳. Toda alteração exige confirmação explícita do usuário.", in, out, exec)
}

func buildUpdateCardExec(engine wf.Engine[workflows.CardManageState], def wf.Definition[workflows.CardManageState]) func(context.Context, UpdateCardInput) (UpdateCardOutput, error) {
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
		if _, err := uuid.Parse(in.CardID); err != nil {
			return UpdateCardOutput{}, fmt.Errorf("agents.tool.update_card: parse card uuid: %w", err)
		}

		state := workflows.CardManageState{
			Status:    workflows.CardManageActive,
			Operation: workflows.CardManageOpEdit,
			UserID:    userID,
			CardID:    in.CardID,
			MessageID: req.MessageID,
		}
		if in.Nickname != nil {
			state.Nickname = *in.Nickname
			state.NicknameProvided = true
		}
		if in.Bank != nil {
			state.Bank = *in.Bank
			state.BankProvided = true
		}
		if in.DueDay != nil {
			state.DueDay = *in.DueDay
			state.DueDayProvided = true
		}

		key := workflows.CardManageKey(req.ResourceID, req.ThreadID)
		result, startErr := engine.Start(ctx, def, key, state)
		if startErr != nil && !errors.Is(startErr, wf.ErrRunAlreadyExists) {
			return UpdateCardOutput{}, fmt.Errorf("agents.tool.update_card: iniciar confirmação: %w", startErr)
		}
		if errors.Is(startErr, wf.ErrRunAlreadyExists) {
			return UpdateCardOutput{
				Outcome:            updateCardOutcomePendingConfirmationExists,
				ConfirmationPrompt: "Há uma confirmação pendente. Por favor, responda sim ou não antes de solicitar outra operação.",
			}, nil
		}

		return UpdateCardOutput{
			Outcome:            updateCardOutcomeNeedsConfirmation,
			ConfirmationPrompt: result.State.ResponseText,
		}, nil
	}
}
