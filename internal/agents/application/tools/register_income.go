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

type RegisterIncomeInput struct {
	Wamid         string     `json:"wamid"`
	ItemSeq       int        `json:"itemSeq"`
	UserID        string     `json:"userId"`
	AmountCents   int64      `json:"amountCents"`
	Description   string     `json:"description"`
	PaymentMethod string     `json:"paymentMethod"`
	OccurredAt    string     `json:"occurredAt,omitempty"`
	CategoryID    *uuid.UUID `json:"categoryId,omitempty"`
	SubcategoryID *uuid.UUID `json:"subcategoryId,omitempty"`
}

type RegisterIncomeOutput struct {
	ResourceID string `json:"resourceId"`
	Kind       string `json:"kind"`
	IsReplay   bool   `json:"isReplay"`
	Outcome    string `json:"outcome"`
}

func BuildRegisterIncomeTool(ledger interfaces.TransactionsLedger, writer idempotentWriter) tool.ToolHandle {
	in := llm.Schema{
		Name:   "register_income_input",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"wamid":         map[string]any{"type": "string"},
				"itemSeq":       map[string]any{"type": "integer"},
				"userId":        map[string]any{"type": "string"},
				"amountCents":   map[string]any{"type": "integer"},
				"description":   map[string]any{"type": "string"},
				"paymentMethod": map[string]any{"type": "string"},
				"occurredAt":    map[string]any{"type": "string"},
				"categoryId":    map[string]any{"type": "string"},
				"subcategoryId": map[string]any{"type": "string"},
			},
			"required":             []string{"wamid", "itemSeq", "userId", "amountCents", "description", "paymentMethod"},
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
	return tool.NewTool[RegisterIncomeInput, RegisterIncomeOutput]("register_income", "Registra um lançamento de receita no ledger financeiro do usuário.", in, out, buildRegisterIncomeExec(ledger, writer))
}

func buildRegisterIncomeExec(ledger interfaces.TransactionsLedger, writer idempotentWriter) func(context.Context, RegisterIncomeInput) (RegisterIncomeOutput, error) {
	return func(ctx context.Context, in RegisterIncomeInput) (RegisterIncomeOutput, error) {
		userID, err := uuid.Parse(in.UserID)
		if err != nil {
			return RegisterIncomeOutput{}, fmt.Errorf("register_income: userId inválido: %w", err)
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
		result, writeErr := writer.Execute(ctx, userID, in.Wamid, in.ItemSeq, "create_income", "transaction", func(ctx context.Context) (uuid.UUID, bool, error) {
			ref, err := ledger.CreateTransaction(ctx, interfaces.RawTransaction{
				Direction:       "income",
				PaymentMethod:   in.PaymentMethod,
				AmountCents:     in.AmountCents,
				Description:     in.Description,
				OccurredAt:      occurredAt,
				CategoryID:      catID,
				SubcategoryID:   in.SubcategoryID,
				OriginWamid:     in.Wamid,
				OriginItemSeq:   in.ItemSeq,
				OriginOperation: "create_income",
			})
			if err != nil {
				return uuid.Nil, false, err
			}
			return ref.ID, ref.Reconciled, nil
		})
		if writeErr != nil {
			return RegisterIncomeOutput{}, fmt.Errorf("register_income: %w", writeErr)
		}
		return RegisterIncomeOutput{
			ResourceID: result.ResourceID.String(),
			Kind:       "transaction",
			IsReplay:   result.Outcome == agent.ToolOutcomeReplay,
			Outcome:    result.Outcome.String(),
		}, nil
	}
}
