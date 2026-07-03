package tools

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/tool"
)

type RegisterExpenseInput struct {
	AmountCents   int64      `json:"amountCents"`
	Description   string     `json:"description"`
	PaymentMethod string     `json:"paymentMethod"`
	OccurredAt    string     `json:"occurredAt,omitempty"`
	CategoryID    *uuid.UUID `json:"categoryId,omitempty"`
	SubcategoryID *uuid.UUID `json:"subcategoryId,omitempty"`
}

type RegisterExpenseOutput struct {
	ResourceID string `json:"resourceId"`
	Kind       string `json:"kind"`
	IsReplay   bool   `json:"isReplay"`
	Outcome    string `json:"outcome"`
}

func BuildRegisterExpenseTool(ledger interfaces.TransactionsLedger, writer idempotentWriter) tool.ToolHandle {
	in := llm.Schema{
		Name:   "register_expense_input",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"amountCents":   map[string]any{"type": "integer"},
				"description":   map[string]any{"type": "string"},
				"paymentMethod": map[string]any{"type": "string"},
				"occurredAt":    map[string]any{"type": "string"},
				"categoryId":    map[string]any{"type": "string"},
				"subcategoryId": map[string]any{"type": "string"},
			},
			"required":             []string{"amountCents", "description", "paymentMethod"},
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
			},
			"required":             []string{"resourceId", "kind", "isReplay", "outcome"},
			"additionalProperties": false,
		},
	}
	return tool.NewTool[RegisterExpenseInput, RegisterExpenseOutput]("register_expense", "Registra um lançamento de despesa no ledger financeiro do usuário.", in, out, buildRegisterExpenseExec(ledger, writer))
}

func buildRegisterExpenseExec(ledger interfaces.TransactionsLedger, writer idempotentWriter) func(context.Context, RegisterExpenseInput) (RegisterExpenseOutput, error) {
	return func(ctx context.Context, in RegisterExpenseInput) (RegisterExpenseOutput, error) {
		resourceID, wamid, itemSeq, ok := agent.InboundIdentityFromContext(ctx)
		if !ok {
			return RegisterExpenseOutput{}, fmt.Errorf("register_expense: identidade não disponível no contexto")
		}
		userID, err := uuid.Parse(resourceID)
		if err != nil {
			return RegisterExpenseOutput{}, fmt.Errorf("register_expense: userId inválido: %w", err)
		}
		occurredAt := in.OccurredAt
		if occurredAt == "" {
			loc, locErr := time.LoadLocation("America/Sao_Paulo")
			if locErr != nil {
				loc = time.UTC
			}
			occurredAt = time.Now().In(loc).Format("2006-01-02")
		}
		catID := uuid.Nil
		if in.CategoryID != nil {
			catID = *in.CategoryID
		}
		result, writeErr := writer.Execute(ctx, userID, wamid, itemSeq, "create_expense", "transaction", func(ctx context.Context) (uuid.UUID, bool, error) {
			ref, err := ledger.CreateTransaction(ctx, interfaces.RawTransaction{
				Direction:       "outcome",
				PaymentMethod:   in.PaymentMethod,
				AmountCents:     in.AmountCents,
				Description:     in.Description,
				OccurredAt:      occurredAt,
				CategoryID:      catID,
				SubcategoryID:   in.SubcategoryID,
				OriginWamid:     wamid,
				OriginItemSeq:   itemSeq,
				OriginOperation: "create_expense",
			})
			if err != nil {
				return uuid.Nil, false, err
			}
			return ref.ID, ref.Reconciled, nil
		})
		if writeErr != nil {
			return RegisterExpenseOutput{}, fmt.Errorf("register_expense: %w", writeErr)
		}
		return RegisterExpenseOutput{
			ResourceID: result.ResourceID.String(),
			Kind:       "transaction",
			IsReplay:   result.Outcome == agent.ToolOutcomeReplay,
			Outcome:    result.Outcome.String(),
		}, nil
	}
}
