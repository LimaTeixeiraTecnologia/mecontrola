package tools

import (
	"context"
	"errors"
	"fmt"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/workflows"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/tool"
	wf "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

const (
	editGoalOutcomeStarted       = "started"
	editGoalOutcomePendingExists = "pending_exists"
)

type EditGoalInput struct{}

type EditGoalOutput struct {
	Outcome            string `json:"outcome"`
	ConfirmationPrompt string `json:"confirmationPrompt"`
}

func BuildEditGoalTool(engine wf.Engine[workflows.GoalEditState], def wf.Definition[workflows.GoalEditState]) tool.ToolHandle {
	in := llm.Schema{
		Name:   "edit_goal_input",
		Strict: true,
		Schema: map[string]any{
			"type":                 "object",
			"properties":           map[string]any{},
			"required":             []string{},
			"additionalProperties": false,
		},
	}
	out := llm.Schema{
		Name:   "edit_goal_output",
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
	exec := buildEditGoalExec(engine, def)
	return tool.NewTool[EditGoalInput, EditGoalOutput]("edit_goal", "Inicia a alteração do objetivo financeiro do usuário, perguntando o novo objetivo antes de gravar na memória de trabalho.", in, out, exec)
}

func buildEditGoalExec(engine wf.Engine[workflows.GoalEditState], def wf.Definition[workflows.GoalEditState]) func(context.Context, EditGoalInput) (EditGoalOutput, error) {
	return func(ctx context.Context, _ EditGoalInput) (EditGoalOutput, error) {
		rc, ok := wf.RuntimeFrom(ctx)
		if !ok {
			return EditGoalOutput{}, fmt.Errorf("agents.tool.edit_goal: inbound request ausente no contexto")
		}
		req, ok := rc.(agent.InboundRequest)
		if !ok {
			return EditGoalOutput{}, fmt.Errorf("agents.tool.edit_goal: tipo de runtime inválido")
		}

		state := workflows.GoalEditState{
			Status:     workflows.GoalEditActive,
			ResourceID: req.ResourceID,
			MessageID:  req.MessageID,
		}

		key := workflows.GoalEditKey(req.ResourceID, req.ThreadID)
		result, startErr := engine.Start(ctx, def, key, state)
		if startErr != nil && !errors.Is(startErr, wf.ErrRunAlreadyExists) {
			return EditGoalOutput{}, fmt.Errorf("agents.tool.edit_goal: iniciar workflow: %w", startErr)
		}
		if errors.Is(startErr, wf.ErrRunAlreadyExists) {
			return EditGoalOutput{
				Outcome:            editGoalOutcomePendingExists,
				ConfirmationPrompt: "Há uma alteração de objetivo em andamento. Por favor, responda a pergunta anterior antes de solicitar outra.",
			}, nil
		}

		return EditGoalOutput{
			Outcome:            editGoalOutcomeStarted,
			ConfirmationPrompt: result.State.ResponseText,
		}, nil
	}
}
