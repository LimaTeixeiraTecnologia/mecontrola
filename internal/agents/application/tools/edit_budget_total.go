package tools

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/workflows"
	budgetsvo "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/tool"
	wf "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

const (
	editBudgetTotalOutcomeStarted       = "started"
	editBudgetTotalOutcomeClarify       = "clarify"
	editBudgetTotalOutcomePendingExists = "pending_exists"
)

type EditBudgetTotalInput struct {
	MonthRefKind string `json:"monthRefKind,omitempty"`
	Year         int    `json:"year,omitempty"`
	Month        int    `json:"month,omitempty"`
}

type EditBudgetTotalOutput struct {
	Outcome            string `json:"outcome"`
	Competence         string `json:"competence"`
	ConfirmationPrompt string `json:"confirmationPrompt"`
	ClarifyPrompt      string `json:"clarifyPrompt"`
}

func BuildEditBudgetTotalTool(engine wf.Engine[workflows.BudgetManageState], def wf.Definition[workflows.BudgetManageState]) tool.ToolHandle {
	in := llm.Schema{
		Name:   "edit_budget_total_input",
		Strict: false,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"monthRefKind": map[string]any{
					"type":        "string",
					"enum":        []string{"current", "previous", "next", "explicit", "named_without_year", "unknown"},
					"description": "Classificação da referência de mês citada pelo usuário. Vazio assume o mês corrente.",
				},
				"year":  map[string]any{"type": "integer"},
				"month": map[string]any{"type": "integer", "minimum": 1, "maximum": 12},
			},
			"required":             []string{},
			"additionalProperties": false,
		},
	}
	out := llm.Schema{
		Name:   "edit_budget_total_output",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"outcome":            map[string]any{"type": "string"},
				"competence":         map[string]any{"type": "string"},
				"confirmationPrompt": map[string]any{"type": "string"},
				"clarifyPrompt":      map[string]any{"type": "string"},
			},
			"required":             []string{"outcome", "competence", "confirmationPrompt", "clarifyPrompt"},
			"additionalProperties": false,
		},
	}
	exec := buildEditBudgetTotalExec(engine, def)
	return tool.NewTool[EditBudgetTotalInput, EditBudgetTotalOutput]("edit_budget_total", "Inicia a alteração do valor total mensal do orçamento ativo do usuário; as categorias serão reescaladas proporcionalmente após confirmação.", in, out, exec)
}

func buildEditBudgetTotalExec(engine wf.Engine[workflows.BudgetManageState], def wf.Definition[workflows.BudgetManageState]) func(context.Context, EditBudgetTotalInput) (EditBudgetTotalOutput, error) {
	return func(ctx context.Context, in EditBudgetTotalInput) (EditBudgetTotalOutput, error) {
		rc, ok := wf.RuntimeFrom(ctx)
		if !ok {
			return EditBudgetTotalOutput{}, fmt.Errorf("agents.tool.edit_budget_total: inbound request ausente no contexto")
		}
		req, ok := rc.(agent.InboundRequest)
		if !ok {
			return EditBudgetTotalOutput{}, fmt.Errorf("agents.tool.edit_budget_total: tipo de runtime inválido")
		}

		userID, err := uuid.Parse(req.ResourceID)
		if err != nil {
			return EditBudgetTotalOutput{}, fmt.Errorf("agents.tool.edit_budget_total: parse resource uuid: %w", err)
		}

		competence, clarifyReason, err := resolveCompetenceReference(in.MonthRefKind, in.Year, in.Month)
		if err != nil {
			return EditBudgetTotalOutput{}, fmt.Errorf("agents.tool.edit_budget_total: resolver competência: %w", err)
		}
		if clarifyReason != budgetsvo.ClarifyNone {
			return EditBudgetTotalOutput{
				Outcome:       editBudgetTotalOutcomeClarify,
				ClarifyPrompt: competenceReferenceClarifyPrompt(clarifyReason),
			}, nil
		}
		competenceStr := competence.String()
		if competenceStr == "" {
			competenceStr = currentCompetenceFallback()
		}

		state := workflows.BudgetManageState{
			Status:     workflows.BudgetManageActive,
			Operation:  workflows.BudgetManageOpEditTotal,
			UserID:     userID,
			Competence: competenceStr,
			MessageID:  req.MessageID,
		}

		key := workflows.BudgetManageKey(req.ResourceID, req.ThreadID)
		result, startErr := engine.Start(ctx, def, key, state)
		if startErr != nil && !errors.Is(startErr, wf.ErrRunAlreadyExists) {
			return EditBudgetTotalOutput{}, fmt.Errorf("agents.tool.edit_budget_total: iniciar workflow: %w", startErr)
		}
		if errors.Is(startErr, wf.ErrRunAlreadyExists) {
			return EditBudgetTotalOutput{
				Outcome:       editBudgetTotalOutcomePendingExists,
				Competence:    competenceStr,
				ClarifyPrompt: "Há uma operação de orçamento em andamento. Por favor, responda a pergunta anterior antes de solicitar outra.",
			}, nil
		}

		return EditBudgetTotalOutput{
			Outcome:            editBudgetTotalOutcomeStarted,
			Competence:         competenceStr,
			ConfirmationPrompt: result.State.ResponseText,
		}, nil
	}
}
