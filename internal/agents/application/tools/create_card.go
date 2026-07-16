package tools

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
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

func BuildCreateCardTool(engine wf.Engine[workflows.CardManageState], def wf.Definition[workflows.CardManageState], cards interfaces.CardManager) tool.ToolHandle {
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
	exec := buildCreateCardExec(engine, def, cards)
	return tool.NewTool[CreateCardInput, CreateCardOutput]("create_card", "Cadastra um novo 💳 de crédito pela conversa. Requer confirmação humana explícita antes de criar.", in, out, exec)
}

func buildCreateCardExec(engine wf.Engine[workflows.CardManageState], def wf.Definition[workflows.CardManageState], cards interfaces.CardManager) func(context.Context, CreateCardInput) (CreateCardOutput, error) {
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

		if clarify, ok := createCardMissingSlot(in); ok {
			return CreateCardOutput{
				Outcome:       createCardOutcomeNeedsSlot,
				ClarifyPrompt: clarify,
			}, nil
		}

		recognized, err := cards.BankRecognized(ctx, in.Bank)
		if err != nil {
			return CreateCardOutput{}, fmt.Errorf("agents.tool.create_card: verificar banco: %w", err)
		}

		var (
			closingDay         int
			closingDayProvided bool
		)
		switch {
		case recognized:
			closingDayProvided = false
		case in.ClosingDay == nil:
			return CreateCardOutput{
				Outcome:       createCardOutcomeNeedsClosing,
				ClarifyPrompt: "Não reconheço esse banco na minha lista. Qual é o dia de fechamento da fatura desse 💳?",
			}, nil
		default:
			closingDay = *in.ClosingDay
			closingDayProvided = true
		}

		state := workflows.CardManageState{
			Status:             workflows.CardManageActive,
			Operation:          workflows.CardManageOpCreate,
			UserID:             userID,
			Nickname:           in.Nickname,
			NicknameProvided:   true,
			Bank:               in.Bank,
			BankProvided:       true,
			DueDay:             in.DueDay,
			DueDayProvided:     true,
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
				ClarifyPrompt: "Há uma confirmação pendente. Por favor, responda sim ou não antes de solicitar outro cadastro.",
			}, nil
		}

		return CreateCardOutput{
			Outcome:            createCardOutcomeNeedsConfirmation,
			ConfirmationPrompt: result.State.ResponseText,
		}, nil
	}
}

func createCardMissingSlot(in CreateCardInput) (string, bool) {
	if strings.TrimSpace(in.Nickname) == "" {
		return "Qual apelido você quer dar para esse 💳?", true
	}
	if strings.TrimSpace(in.Bank) == "" {
		return "Qual é o banco desse 💳?", true
	}
	if in.DueDay <= 0 {
		return "Qual é o dia de vencimento da fatura desse 💳?", true
	}
	return "", false
}
