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
	editTreatmentNameStatusStarted       = "started"
	editTreatmentNameStatusPendingExists = "pending_exists"
)

type EditTreatmentNameInput struct {
	Name string `json:"name,omitempty"`
}

func (i *EditTreatmentNameInput) Validate() error {
	return nil
}

type EditTreatmentNameOutput struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

func BuildEditTreatmentNameTool(engine wf.Engine[workflows.TreatmentNameEditState], def wf.Definition[workflows.TreatmentNameEditState]) tool.ToolHandle {
	in := llm.Schema{
		Name:   "edit_treatment_name_input",
		Strict: false,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type":        "string",
					"description": "Novo nome de tratamento informado pelo usuário na própria mensagem que pediu a troca, se houver. Vazio quando o usuário ainda não disse o nome.",
				},
			},
			"required":             []string{},
			"additionalProperties": false,
		},
	}
	out := llm.Schema{
		Name:   "edit_treatment_name_output",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"status":  map[string]any{"type": "string"},
				"message": map[string]any{"type": "string"},
			},
			"required":             []string{"status", "message"},
			"additionalProperties": false,
		},
	}
	exec := buildEditTreatmentNameExec(engine, def)
	return tool.NewTool[EditTreatmentNameInput, EditTreatmentNameOutput]("edit_treatment_name", "Inicia a alteração do nome de tratamento do usuário. SEMPRE chame esta ferramenta quando o usuário pedir para trocar como é chamado, mesmo que o campo name venha vazio — NUNCA responda essa pergunta diretamente sem chamar a ferramenta, pois é ela quem persiste o estado necessário para aplicar o nome na resposta seguinte. Aplica imediatamente quando o nome já vier na mensagem (name preenchido) ou pergunta o novo nome antes de gravar na memória de trabalho (name vazio).", in, out, exec)
}

func buildEditTreatmentNameExec(engine wf.Engine[workflows.TreatmentNameEditState], def wf.Definition[workflows.TreatmentNameEditState]) func(context.Context, EditTreatmentNameInput) (EditTreatmentNameOutput, error) {
	return func(ctx context.Context, in EditTreatmentNameInput) (EditTreatmentNameOutput, error) {
		if err := in.Validate(); err != nil {
			return EditTreatmentNameOutput{}, fmt.Errorf("agents.tool.edit_treatment_name: input inválido: %w", err)
		}

		rc, ok := wf.RuntimeFrom(ctx)
		if !ok {
			return EditTreatmentNameOutput{}, fmt.Errorf("agents.tool.edit_treatment_name: inbound request ausente no contexto")
		}
		req, ok := rc.(agent.InboundRequest)
		if !ok {
			return EditTreatmentNameOutput{}, fmt.Errorf("agents.tool.edit_treatment_name: tipo de runtime inválido")
		}

		state := workflows.TreatmentNameEditState{
			Status:       workflows.TreatmentNameEditActive,
			ResourceID:   req.ResourceID,
			MessageID:    req.MessageID,
			ProvidedName: in.Name,
		}

		key := workflows.TreatmentNameEditKey(req.ResourceID, req.ThreadID)
		result, startErr := engine.Start(ctx, def, key, state)
		if startErr != nil && !errors.Is(startErr, wf.ErrRunAlreadyExists) {
			return EditTreatmentNameOutput{}, fmt.Errorf("agents.tool.edit_treatment_name: iniciar workflow: %w", startErr)
		}
		if errors.Is(startErr, wf.ErrRunAlreadyExists) {
			return EditTreatmentNameOutput{
				Status:  editTreatmentNameStatusPendingExists,
				Message: "Há uma alteração de nome de tratamento em andamento. Por favor, responda a pergunta anterior antes de solicitar outra.",
			}, nil
		}

		return EditTreatmentNameOutput{
			Status:  editTreatmentNameStatusStarted,
			Message: result.State.ResponseText,
		}, nil
	}
}
