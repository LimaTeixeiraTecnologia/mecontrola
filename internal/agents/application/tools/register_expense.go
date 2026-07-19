package tools

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/tool"
)

const cardNotFoundClarifyMessage = "❌ Não encontrei esse 💳. Pode me dizer o apelido do 💳 (ex.: nubank) para eu localizar o certo?"

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
	CategoryText    string `json:"categoryText,omitempty"`
}

type RegisterExpenseOutput struct {
	ResourceID string `json:"resourceId"`
	Kind       string `json:"kind"`
	IsReplay   bool   `json:"isReplay"`
	Outcome    string `json:"outcome"`
	Message    string `json:"message"`
}

func BuildRegisterExpenseTool(registrar entryRegistrar, cards interfaces.CardManager) tool.ToolHandle {
	in := llm.Schema{
		Name:   "register_expense_input",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"amountCents":     map[string]any{"type": "integer"},
				"description":     map[string]any{"type": "string"},
				"paymentMethod":   map[string]any{"type": "string", "enum": []string{"pix", "debit_card", "debit_in_account", "cash", "boleto", "ted", "credit_card", "doc", "vale_refeicao", "vale_alimentacao", "transferencia", "apple_pay", "google_pay", "picpay", "mercado_pago", "cheque"}},
				"cardId":          map[string]any{"type": "string"},
				"installments":    map[string]any{"type": "integer", "minimum": 1, "maximum": 24},
				"occurredAt":      map[string]any{"type": "string"},
				"categoryId":      map[string]any{"type": "string"},
				"subcategoryId":   map[string]any{"type": "string"},
				"categoryVersion": map[string]any{"type": "integer"},
				"categoryText":    map[string]any{"type": "string"},
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
	return tool.NewVerbatimTool("register_expense", "Registra um lançamento de despesa no ledger financeiro do usuário; a categoria é resolvida automaticamente por busca textual do campo description (nunca parafraseie o termo do usuário). Se o usuário citar a categoria desejada, copie o trecho literal no campo categoryText. Para compra no 💳 de crédito, use paymentMethod=credit_card com cardId (obtido via resolve_card) e installments (1 para à vista, 2..24 para parcelada).", in, out, buildRegisterExpenseExec(registrar, cards), extractRegisterExpenseVerbatim)
}

func extractRegisterExpenseVerbatim(o RegisterExpenseOutput) (string, bool) {
	return o.Message, o.Outcome == agent.ToolOutcomeClarify.String() && o.Message != ""
}

func buildRegisterExpenseExec(registrar entryRegistrar, cards interfaces.CardManager) func(context.Context, RegisterExpenseInput) (RegisterExpenseOutput, error) {
	return func(ctx context.Context, in RegisterExpenseInput) (RegisterExpenseOutput, error) {
		resourceID, threadID, wamid, itemSeq, ok := agent.InboundExecutionFromContext(ctx)
		if !ok {
			return RegisterExpenseOutput{}, fmt.Errorf("register_expense: identidade não disponível no contexto")
		}
		userID, err := uuid.Parse(resourceID)
		if err != nil {
			return RegisterExpenseOutput{}, fmt.Errorf("register_expense: userId inválido: %w", err)
		}
		cardID, err := resolveRegisterExpenseCard(ctx, cards, in.CardID, userID)
		if err != nil {
			var clarifyErr *clarifyError
			if errors.As(err, &clarifyErr) {
				return RegisterExpenseOutput{
					Outcome: agent.ToolOutcomeClarify.String(),
					Message: clarifyErr.message,
				}, nil
			}
			return RegisterExpenseOutput{}, err
		}
		categoryID, subcategoryID, err := resolveRegisterExpenseCategoryIDs(in.CategoryID, in.SubcategoryID)
		if err != nil {
			return RegisterExpenseOutput{}, err
		}
		if err := validateEntryAmount(in.AmountCents); err != nil {
			return RegisterExpenseOutput{
				Outcome: "clarify",
				Message: "❌ Valor inválido. O valor deve ser positivo e não ultrapassar R$ 10.000.000,00.",
			}, nil
		}
		if err := validateEntryDescription(in.Description); err != nil {
			return RegisterExpenseOutput{
				Outcome: "clarify",
				Message: "❌ Não entendi o que foi esse gasto. Me diz em uma palavra (ex.: mercado, farmácia)? 🙂",
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
			CategoryText:    in.CategoryText,
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

func resolveRegisterExpenseCard(ctx context.Context, cards interfaces.CardManager, cardIDStr string, userID uuid.UUID) (*uuid.UUID, error) {
	if cardIDStr == "" {
		return nil, nil
	}
	parsed, err := uuid.Parse(cardIDStr)
	if err != nil {
		return nil, fmt.Errorf("register_expense: cardId inválido: %w", err)
	}
	if _, getErr := cards.GetCard(ctx, parsed, userID); getErr != nil {
		if errors.Is(getErr, interfaces.ErrCardNotFound) {
			return nil, errCardNotFoundClarify
		}
		return nil, fmt.Errorf("register_expense: %w", getErr)
	}
	return &parsed, nil
}

func resolveRegisterExpenseCategoryIDs(categoryIDStr, subcategoryIDStr string) (uuid.UUID, uuid.UUID, error) {
	categoryID := parseOptionalUUID(categoryIDStr)
	subcategoryID := parseOptionalUUID(subcategoryIDStr)
	if categoryID == uuid.Nil {
		return uuid.Nil, uuid.Nil, nil
	}
	return categoryID, subcategoryID, nil
}

func parseOptionalUUID(value string) uuid.UUID {
	parsed, err := uuid.Parse(value)
	if err != nil {
		return uuid.Nil
	}
	return parsed
}

var errCardNotFoundClarify = &clarifyError{message: cardNotFoundClarifyMessage}

type clarifyError struct {
	message string
}

func (e *clarifyError) Error() string {
	return e.message
}
