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

type RegisterExpenseInput struct {
	AmountCents     int64  `json:"amountCents"`
	Description     string `json:"description"`
	PaymentMethod   string `json:"paymentMethod"`
	CardID          string `json:"cardId,omitempty"`
	Installments    int    `json:"installments,omitempty"`
	OccurredAt      string `json:"occurredAt,omitempty"`
	CategoryID      string `json:"categoryId,omitempty"`
	SubcategoryID   string `json:"subcategoryId,omitempty"`
	CategoryVersion int64  `json:"categoryVersion,omitempty"`
}

type RegisterExpenseOutput struct {
	ResourceID string `json:"resourceId"`
	Kind       string `json:"kind"`
	IsReplay   bool   `json:"isReplay"`
	Outcome    string `json:"outcome"`
	Message    string `json:"message"`
}

func BuildRegisterExpenseTool(registrar entryRegistrar) tool.ToolHandle {
	in := llm.Schema{
		Name:   "register_expense_input",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"amountCents":     map[string]any{"type": "integer"},
				"description":     map[string]any{"type": "string"},
				"paymentMethod":   map[string]any{"type": "string", "enum": []string{"pix", "debit_card", "debit_in_account", "cash", "boleto", "ted", "credit_card", "vale_refeicao", "vale_alimentacao"}},
				"cardId":          map[string]any{"type": "string"},
				"installments":    map[string]any{"type": "integer", "minimum": 1, "maximum": 24},
				"occurredAt":      map[string]any{"type": "string"},
				"categoryId":      map[string]any{"type": "string"},
				"subcategoryId":   map[string]any{"type": "string"},
				"categoryVersion": map[string]any{"type": "integer"},
			},
			"required":             []string{"amountCents", "description"},
			"additionalProperties": false,
		},
	}
	out := llm.Schema{
		Name:   "register_expense_output",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"resourceId": map[string]any{"type": "string"},
				"kind":       map[string]any{"type": "string"},
				"isReplay":   map[string]any{"type": "boolean"},
				"outcome":    map[string]any{"type": "string"},
				"message":    map[string]any{"type": "string"},
			},
			"required":             []string{"resourceId", "kind", "isReplay", "outcome", "message"},
			"additionalProperties": false,
		},
	}
	return tool.NewVerbatimTool("register_expense", "Registra um lançamento de despesa no ledger financeiro do usuário; a categoria é resolvida automaticamente por busca textual do campo description (nunca parafraseie o termo do usuário). Para compra no cartão de crédito, use paymentMethod=credit_card com cardId (obtido via resolve_card) e installments (1 para à vista, 2..24 para parcelada).", in, out, buildRegisterExpenseExec(registrar), extractRegisterExpenseVerbatim)
}

func extractRegisterExpenseVerbatim(o RegisterExpenseOutput) (string, bool) {
	return o.Message, o.Outcome == agent.ToolOutcomeClarify.String() && o.Message != ""
}

func buildRegisterExpenseExec(registrar entryRegistrar) func(context.Context, RegisterExpenseInput) (RegisterExpenseOutput, error) {
	return func(ctx context.Context, in RegisterExpenseInput) (RegisterExpenseOutput, error) {
		resourceID, threadID, wamid, itemSeq, ok := agent.InboundExecutionFromContext(ctx)
		if !ok {
			return RegisterExpenseOutput{}, fmt.Errorf("register_expense: identidade não disponível no contexto")
		}
		userID, err := uuid.Parse(resourceID)
		if err != nil {
			return RegisterExpenseOutput{}, fmt.Errorf("register_expense: userId inválido: %w", err)
		}
		var cardID *uuid.UUID
		if in.CardID != "" {
			parsed, parseErr := uuid.Parse(in.CardID)
			if parseErr != nil {
				return RegisterExpenseOutput{}, fmt.Errorf("register_expense: cardId inválido: %w", parseErr)
			}
			cardID = &parsed
		}
		var categoryID uuid.UUID
		if in.CategoryID != "" {
			parsed, parseErr := uuid.Parse(in.CategoryID)
			if parseErr != nil {
				return RegisterExpenseOutput{}, fmt.Errorf("register_expense: categoryId inválido: %w", parseErr)
			}
			categoryID = parsed
		}
		var subcategoryID uuid.UUID
		if in.SubcategoryID != "" {
			parsed, parseErr := uuid.Parse(in.SubcategoryID)
			if parseErr != nil {
				return RegisterExpenseOutput{}, fmt.Errorf("register_expense: subcategoryId inválido: %w", parseErr)
			}
			subcategoryID = parsed
		}
		if err := validateEntryAmount(in.AmountCents); err != nil {
			return RegisterExpenseOutput{
				Outcome: "clarify",
				Message: "❌ Valor inválido. O valor deve ser positivo e não ultrapassar R$ 10.000.000,00.",
			}, nil
		}
		installments := in.Installments
		if installments <= 0 {
			installments = 1
		}
		result, err := registrar.RegisterExpense(ctx, usecases.RegisterExpenseCommand{
			UserID:          userID,
			ThreadID:        threadID,
			WAMID:           wamid,
			ItemSeq:         itemSeq,
			AmountCents:     in.AmountCents,
			Description:     in.Description,
			PaymentMethod:   in.PaymentMethod,
			CardID:          cardID,
			Installments:    installments,
			OccurredAt:      in.OccurredAt,
			CategoryID:      categoryID,
			SubcategoryID:   subcategoryID,
			CategoryVersion: in.CategoryVersion,
		})
		if err != nil {
			return RegisterExpenseOutput{}, fmt.Errorf("register_expense: %w", err)
		}
		resource := ""
		if result.Outcome != agent.ToolOutcomeClarify {
			resource = result.ResourceID.String()
		}
		return RegisterExpenseOutput{
			ResourceID: resource,
			Kind:       result.Kind,
			IsReplay:   result.Outcome == agent.ToolOutcomeReplay,
			Outcome:    result.Outcome.String(),
			Message:    result.Message,
		}, nil
	}
}
