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

type RegisterIncomeInput struct {
	AmountCents int64  `json:"amountCents"`
	Description string `json:"description"`
	OccurredAt  string `json:"occurredAt,omitempty"`
}

type RegisterIncomeOutput struct {
	ResourceID string `json:"resourceId"`
	Kind       string `json:"kind"`
	IsReplay   bool   `json:"isReplay"`
	Outcome    string `json:"outcome"`
}

func BuildRegisterIncomeTool(registrar entryRegistrar) tool.ToolHandle {
	in := llm.Schema{
		Name:   "register_income_input",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"amountCents": map[string]any{"type": "integer"},
				"description": map[string]any{"type": "string"},
				"occurredAt":  map[string]any{"type": "string"},
			},
			"required":             []string{"amountCents", "description"},
			"additionalProperties": false,
		},
	}
	out := llm.Schema{
		Name:   "register_income_output",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"resourceId": map[string]any{"type": "string"},
				"kind":       map[string]any{"type": "string"},
				"isReplay":   map[string]any{"type": "boolean"},
				"outcome":    map[string]any{"type": "string"},
			},
			"required":             []string{"resourceId", "kind", "isReplay", "outcome"},
			"additionalProperties": false,
		},
	}
	return tool.NewTool[RegisterIncomeInput, RegisterIncomeOutput]("register_income", "Registra um lançamento de receita no ledger financeiro do usuário; a categoria é resolvida automaticamente.", in, out, buildRegisterIncomeExec(registrar))
}

func buildRegisterIncomeExec(registrar entryRegistrar) func(context.Context, RegisterIncomeInput) (RegisterIncomeOutput, error) {
	return func(ctx context.Context, in RegisterIncomeInput) (RegisterIncomeOutput, error) {
		resourceID, wamid, itemSeq, ok := agent.InboundIdentityFromContext(ctx)
		if !ok {
			return RegisterIncomeOutput{}, fmt.Errorf("register_income: identidade não disponível no contexto")
		}
		userID, err := uuid.Parse(resourceID)
		if err != nil {
			return RegisterIncomeOutput{}, fmt.Errorf("register_income: userId inválido: %w", err)
		}
		result, err := registrar.RegisterIncome(ctx, usecases.RegisterIncomeCommand{
			UserID:      userID,
			WAMID:       wamid,
			ItemSeq:     itemSeq,
			AmountCents: in.AmountCents,
			Description: in.Description,
			OccurredAt:  in.OccurredAt,
		})
		if err != nil {
			return RegisterIncomeOutput{}, fmt.Errorf("register_income: %w", err)
		}
		resource := ""
		if result.Outcome != agent.ToolOutcomeClarify {
			resource = result.ResourceID.String()
		}
		return RegisterIncomeOutput{
			ResourceID: resource,
			Kind:       result.Kind,
			IsReplay:   result.Outcome == agent.ToolOutcomeReplay,
			Outcome:    result.Outcome.String(),
		}, nil
	}
}
