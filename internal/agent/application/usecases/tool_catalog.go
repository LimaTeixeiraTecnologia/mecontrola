package usecases

import (
	"fmt"
	"strings"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
)

const (
	toolRecordTransaction = "record_transaction"
	toolMonthlySummary    = "monthly_summary"
	toolListCards         = "list_cards"
	toolConfigureBudget   = "configure_budget"
)

var ErrToolUnsupported = fmt.Errorf("agent.usecase.tool_catalog: tool nao suportada")

func AgentToolCatalog() []interfaces.ToolSpec {
	return []interfaces.ToolSpec{
		{
			Name:        toolRecordTransaction,
			Description: "Registra uma transacao financeira de entrada (income) ou saida (outcome), ex: salario, supermercado, iFood.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"direction": map[string]any{
						"type":        "string",
						"enum":        []string{"income", "outcome"},
						"description": "income para entradas (salario, recebimentos); outcome para gastos.",
					},
					"amount_cents": map[string]any{
						"type":        "integer",
						"description": "Valor em centavos (ex: R$ 58,00 = 5800).",
					},
					"merchant": map[string]any{
						"type":        "string",
						"description": "Estabelecimento ou origem (ex: iFood, supermercado, salario).",
					},
					"category_hint": map[string]any{
						"type":        "string",
						"description": "Categoria sugerida em linguagem natural; sera resolvida internamente.",
					},
					"payment_method": map[string]any{
						"type":        "string",
						"description": "Forma de pagamento, quando informada (ex: pix, debito, credito, dinheiro).",
					},
				},
				"required":             []string{"direction", "amount_cents"},
				"additionalProperties": false,
			},
		},
		{
			Name:        toolMonthlySummary,
			Description: "Mostra o resumo mensal do orcamento do usuario para uma competencia (mes).",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"ref_month": map[string]any{
						"type":        "string",
						"description": "Competencia no formato YYYY-MM; vazio assume o mes atual.",
					},
				},
				"additionalProperties": false,
			},
		},
		{
			Name:        toolListCards,
			Description: "Lista os cartoes cadastrados do usuario.",
			Parameters: map[string]any{
				"type":                 "object",
				"properties":           map[string]any{},
				"additionalProperties": false,
			},
		},
		{
			Name:        toolConfigureBudget,
			Description: "Inicia a configuracao do orcamento mensal do usuario.",
			Parameters: map[string]any{
				"type":                 "object",
				"properties":           map[string]any{},
				"additionalProperties": false,
			},
		},
	}
}

func ToolCallToIntent(call interfaces.ToolCall, fallbackText string) (intent.Intent, error) {
	switch call.FunctionName {
	case toolRecordTransaction:
		return recordTransactionIntent(call.ArgumentsJSON)
	case toolMonthlySummary:
		return intent.NewMonthlySummary(stringArg(call.ArgumentsJSON, "ref_month"))
	case toolListCards:
		return intent.NewListCards(), nil
	case toolConfigureBudget:
		return intent.NewConfigureBudget(), nil
	default:
		return intent.Intent{}, fmt.Errorf("%w: %s", ErrToolUnsupported, call.FunctionName)
	}
}

func recordTransactionIntent(args map[string]any) (intent.Intent, error) {
	direction := strings.ToLower(strings.TrimSpace(stringArg(args, "direction")))
	dto := rawIntentDTO{
		AmountCents:   intArg(args, "amount_cents"),
		Merchant:      stringArg(args, "merchant"),
		CategoryHint:  stringArg(args, "category_hint"),
		PaymentMethod: stringArg(args, "payment_method"),
	}
	if direction == directionIncome {
		return intent.NewLogIncome(intent.LogIncomeFields{
			AmountCents:   dto.AmountCents,
			Source:        dto.Merchant,
			CategoryHint:  dto.CategoryHint,
			PaymentMethod: dto.PaymentMethod,
		})
	}
	return intent.NewLogExpense(intent.LogExpenseFields{
		AmountCents:   dto.AmountCents,
		Merchant:      dto.Merchant,
		CategoryHint:  dto.CategoryHint,
		PaymentMethod: dto.PaymentMethod,
	})
}

func stringArg(args map[string]any, key string) string {
	if args == nil {
		return ""
	}
	if v, ok := args[key].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

func intArg(args map[string]any, key string) int64 {
	if args == nil {
		return 0
	}
	switch v := args[key].(type) {
	case float64:
		return int64(v)
	case int64:
		return v
	case int:
		return int64(v)
	default:
		return 0
	}
}
