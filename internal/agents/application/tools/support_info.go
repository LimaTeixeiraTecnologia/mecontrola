package tools

import (
	"context"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/messages"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/tool"
)

type SupportInfoInput struct{}

type SupportInfoOutput struct {
	Message string `json:"message"`
}

func BuildSupportInfoTool() tool.ToolHandle {
	in := llm.Schema{
		Name:   "support_info_input",
		Strict: true,
		Schema: map[string]any{
			"type":                 "object",
			"properties":           map[string]any{},
			"required":             []string{},
			"additionalProperties": false,
		},
	}
	out := llm.Schema{
		Name:   "support_info_output",
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
	return tool.NewVerbatimTool("support_info", "Responde com o canal de suporte oficial (e-mail e prazo de resposta); leitura estática, não altera dados do usuário.", in, out, buildSupportInfoExec(), extractSupportInfoVerbatim)
}

func extractSupportInfoVerbatim(o SupportInfoOutput) (string, bool) {
	return o.Message, o.Message != ""
}

func buildSupportInfoExec() func(context.Context, SupportInfoInput) (SupportInfoOutput, error) {
	return func(_ context.Context, _ SupportInfoInput) (SupportInfoOutput, error) {
		return SupportInfoOutput{Message: messages.SupportInfo()}, nil
	}
}
