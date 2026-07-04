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

type RegisterCardPurchaseInput struct {
	CardID            string `json:"cardId"`
	TotalAmountCents  int64  `json:"totalAmountCents"`
	InstallmentsTotal int    `json:"installmentsTotal"`
	Description       string `json:"description"`
	PurchasedAt       string `json:"purchasedAt,omitempty"`
}

type RegisterCardPurchaseOutput struct {
	ResourceID string `json:"resourceId"`
	Kind       string `json:"kind"`
	IsReplay   bool   `json:"isReplay"`
	Outcome    string `json:"outcome"`
}

func BuildRegisterCardPurchaseTool(registrar entryRegistrar) tool.ToolHandle {
	in := llm.Schema{
		Name:   "register_card_purchase_input",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"cardId":            map[string]any{"type": "string"},
				"totalAmountCents":  map[string]any{"type": "integer"},
				"installmentsTotal": map[string]any{"type": "integer", "minimum": 1, "maximum": 24},
				"description":       map[string]any{"type": "string"},
				"purchasedAt":       map[string]any{"type": "string"},
			},
			"required":             []string{"cardId", "totalAmountCents", "installmentsTotal", "description"},
			"additionalProperties": false,
		},
	}
	out := llm.Schema{
		Name:   "register_card_purchase_output",
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
	return tool.NewTool[RegisterCardPurchaseInput, RegisterCardPurchaseOutput]("register_card_purchase", "Registra uma compra no cartão de crédito no ledger financeiro do usuário; a categoria é resolvida automaticamente.", in, out, buildRegisterCardPurchaseExec(registrar))
}

func buildRegisterCardPurchaseExec(registrar entryRegistrar) func(context.Context, RegisterCardPurchaseInput) (RegisterCardPurchaseOutput, error) {
	return func(ctx context.Context, in RegisterCardPurchaseInput) (RegisterCardPurchaseOutput, error) {
		resourceID, wamid, itemSeq, ok := agent.InboundIdentityFromContext(ctx)
		if !ok {
			return RegisterCardPurchaseOutput{}, fmt.Errorf("register_card_purchase: identidade não disponível no contexto")
		}
		userID, err := uuid.Parse(resourceID)
		if err != nil {
			return RegisterCardPurchaseOutput{}, fmt.Errorf("register_card_purchase: userId inválido: %w", err)
		}
		cardID, err := uuid.Parse(in.CardID)
		if err != nil {
			return RegisterCardPurchaseOutput{}, fmt.Errorf("register_card_purchase: cardId inválido: %w", err)
		}
		result, err := registrar.RegisterCardPurchase(ctx, usecases.RegisterCardPurchaseCommand{
			UserID:            userID,
			WAMID:             wamid,
			ItemSeq:           itemSeq,
			CardID:            cardID,
			TotalAmountCents:  in.TotalAmountCents,
			InstallmentsTotal: in.InstallmentsTotal,
			Description:       in.Description,
			PurchasedAt:       in.PurchasedAt,
		})
		if err != nil {
			return RegisterCardPurchaseOutput{}, fmt.Errorf("register_card_purchase: %w", err)
		}
		resource := ""
		if result.Outcome != agent.ToolOutcomeClarify {
			resource = result.ResourceID.String()
		}
		return RegisterCardPurchaseOutput{
			ResourceID: resource,
			Kind:       result.Kind,
			IsReplay:   result.Outcome == agent.ToolOutcomeReplay,
			Outcome:    result.Outcome.String(),
		}, nil
	}
}
