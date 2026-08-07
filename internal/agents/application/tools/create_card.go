package tools

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/messages"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/workflows"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/tool"
	wf "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

const (
	createCardOutcomeNeedsSlot                 = "needs_slot"
	createCardOutcomeNeedsClosing              = "needs_closing"
	createCardOutcomeNeedsConfirmation         = "needs_confirmation"
	createCardOutcomePendingConfirmationExists = "pending_confirmation_exists"
)

type CreateCardInput struct {
	Nickname   string `json:"nickname"`
	Bank       string `json:"bank"`
	DueDay     int    `json:"dueDay"`
	ClosingDay *int   `json:"closingDay,omitempty"`
}

type CreateCardOutput struct {
	Outcome            string `json:"outcome"`
	ConfirmationPrompt string `json:"confirmationPrompt"`
	ClarifyPrompt      string `json:"clarifyPrompt"`
}

func BuildCreateCardTool(engine wf.Engine[workflows.CardManageState], def wf.Definition[workflows.CardManageState]) tool.ToolHandle {
	in := llm.Schema{
		Name:   "create_card_input",
		Strict: false,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"nickname":   map[string]any{"type": "string"},
				"bank":       map[string]any{"type": "string"},
				"dueDay":     map[string]any{"type": "integer", "minimum": 1, "maximum": 31},
				"closingDay": map[string]any{"type": "integer", "minimum": 1, "maximum": 31},
			},
			"additionalProperties": false,
		},
	}
	out := llm.Schema{
		Name:   "create_card_output",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"outcome":            map[string]any{"type": "string"},
				"confirmationPrompt": map[string]any{"type": "string"},
				"clarifyPrompt":      map[string]any{"type": "string"},
			},
			"required":             []string{"outcome", "confirmationPrompt", "clarifyPrompt"},
			"additionalProperties": false,
		},
	}
	exec := buildCreateCardExec(engine, def)
	return tool.NewTool[CreateCardInput, CreateCardOutput]("create_card", "Cadastra um novo cartão 💳 de crédito pela conversa. Requer confirmação humana explícita antes de criar.", in, out, exec)
}

func buildCreateCardExec(engine wf.Engine[workflows.CardManageState], def wf.Definition[workflows.CardManageState]) func(context.Context, CreateCardInput) (CreateCardOutput, error) {
	return func(ctx context.Context, in CreateCardInput) (CreateCardOutput, error) {
		rc, ok := wf.RuntimeFrom(ctx)
		if !ok {
			return CreateCardOutput{}, fmt.Errorf("agents.tool.create_card: inbound request ausente no contexto")
		}
		req, ok := rc.(agent.InboundRequest)
		if !ok {
			return CreateCardOutput{}, fmt.Errorf("agents.tool.create_card: tipo de runtime inválido")
		}

		userID, err := uuid.Parse(req.ResourceID)
		if err != nil {
			return CreateCardOutput{}, fmt.Errorf("agents.tool.create_card: parse resource uuid: %w", err)
		}

		var (
			closingDay         int
			closingDayProvided bool
		)
		if in.ClosingDay != nil && *in.ClosingDay >= 1 && *in.ClosingDay <= 31 {
			closingDay = *in.ClosingDay
			closingDayProvided = true
		}

		nickname := strings.TrimSpace(in.Nickname)
		bank := strings.TrimSpace(in.Bank)
		state := workflows.CardManageState{
			Status:             workflows.CardManageActive,
			Operation:          workflows.CardManageOpCreate,
			UserID:             userID,
			Nickname:           nickname,
			NicknameProvided:   nickname != "",
			Bank:               bank,
			BankProvided:       bank != "",
			DueDay:             in.DueDay,
			DueDayProvided:     in.DueDay >= 1 && in.DueDay <= 31,
			ClosingDay:         closingDay,
			ClosingDayProvided: closingDayProvided,
			MessageID:          req.MessageID,
		}

		key := workflows.CardManageKey(req.ResourceID, req.ThreadID)
		result, err := engine.Start(ctx, def, key, state)
		if err != nil && !errors.Is(err, wf.ErrRunAlreadyExists) {
			return CreateCardOutput{}, fmt.Errorf("agents.tool.create_card: iniciar confirmação: %w", err)
		}
		if errors.Is(err, wf.ErrRunAlreadyExists) {
			return CreateCardOutput{
				Outcome:       createCardOutcomePendingConfirmationExists,
				ClarifyPrompt: messages.PendingCardCreationExists(),
			}, nil
		}

		if result.State.Awaiting == workflows.CardManageAwaitingConfirm {
			return CreateCardOutput{
				Outcome:            createCardOutcomeNeedsConfirmation,
				ConfirmationPrompt: result.State.ResponseText,
			}, nil
		}

		return CreateCardOutput{
			Outcome:       createCardOutcomeFor(result.State.Awaiting),
			ClarifyPrompt: result.State.ResponseText,
		}, nil
	}
}

func createCardOutcomeFor(awaiting workflows.CardManageAwaiting) string {
	if awaiting == workflows.CardManageAwaitingClosingDay {
		return createCardOutcomeNeedsClosing
	}
	return createCardOutcomeNeedsSlot
}
