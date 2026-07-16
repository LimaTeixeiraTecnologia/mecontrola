package tools

import (
	"context"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/messages"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/tool"
)

type CancelPlanInfoInput struct{}

type CancelPlanInfoOutput struct {
	Message string `json:"message"`
}

func BuildCancelPlanInfoTool() tool.ToolHandle {
	in := llm.Schema{
		Name:   "cancel_plan_info_input",
		Strict: true,
		Schema: map[string]any{
			"type":                 "object",
			"properties":           map[string]any{},
			"required":             []string{},
			"additionalProperties": false,
		},
	}
	out := llm.Schema{
		Name:   "cancel_plan_info_output",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"message": map[string]any{"type": "string"},
			},
			"required":             []string{"message"},
			"additionalProperties": false,
		},
	}
	return tool.NewVerbatimTool("cancel_plan_info", "Responde com o passo a passo oficial para cancelar a assinatura pela Kiwify; leitura estática, não altera o estado da assinatura nem chama billing.", in, out, buildCancelPlanInfoExec(), extractCancelPlanInfoVerbatim)
}

func extractCancelPlanInfoVerbatim(o CancelPlanInfoOutput) (string, bool) {
	return o.Message, o.Message != ""
}

func buildCancelPlanInfoExec() func(context.Context, CancelPlanInfoInput) (CancelPlanInfoOutput, error) {
	return func(_ context.Context, _ CancelPlanInfoInput) (CancelPlanInfoOutput, error) {
		return CancelPlanInfoOutput{Message: messages.CancelPlanInfo()}, nil
	}
}
