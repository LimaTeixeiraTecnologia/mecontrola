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
	toolCreateCard        = "create_card"
	toolCountCards        = "count_cards"
	toolUpdateCard        = "update_card"
	toolDeleteCard        = "delete_card"
	toolEditCategoryPct   = "edit_category_percentage"
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
			Description: "Mostra o orcamento DETALHADO do mes (competencia), com a quebra completa por categoria; use quando o usuario pedir o detalhamento ou fechamento do mes. Para uma visao RESUMIDA da saude financeira, use how_am_i_doing.",
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
		{
			Name:        toolCreateCard,
			Description: "Cadastra um novo cartao de credito do usuario com apelido, dia de fechamento, dia de vencimento e limite.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"nickname": map[string]any{
						"type":        "string",
						"description": "Apelido do cartao (ex: nubank, itau roxinho).",
					},
					"name": map[string]any{
						"type":        "string",
						"description": "Nome completo do cartao; se vazio, usa o apelido.",
					},
					"closing_day": map[string]any{
						"type":        "integer",
						"description": "Dia do mes em que a fatura fecha (1 a 31).",
					},
					"due_day": map[string]any{
						"type":        "integer",
						"description": "Dia do mes em que a fatura vence (1 a 31).",
					},
					"limit_cents": map[string]any{
						"type":        "integer",
						"description": "Limite do cartao em centavos (ex: R$ 5.000,00 = 500000).",
					},
				},
				"required":             []string{"nickname"},
				"additionalProperties": false,
			},
		},
		{
			Name:        toolCountCards,
			Description: "Conta quantos cartoes ativos o usuario tem cadastrados.",
			Parameters: map[string]any{
				"type":                 "object",
				"properties":           map[string]any{},
				"additionalProperties": false,
			},
		},
		{
			Name:        toolUpdateCard,
			Description: "Edita um cartao ja cadastrado; envie apenas os campos que mudam.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"card_name": map[string]any{
						"type":        "string",
						"description": "Apelido ou nome atual do cartao a editar.",
					},
					"new_nickname": map[string]any{
						"type":        "string",
						"description": "Novo apelido do cartao, quando o usuario pedir para mudar.",
					},
					"new_name": map[string]any{
						"type":        "string",
						"description": "Novo nome do cartao, quando o usuario pedir para mudar.",
					},
					"new_closing_day": map[string]any{
						"type":        "integer",
						"description": "Novo dia de fechamento (1 a 31), quando o usuario pedir para mudar.",
					},
					"new_due_day": map[string]any{
						"type":        "integer",
						"description": "Novo dia de vencimento (1 a 31), quando o usuario pedir para mudar.",
					},
				},
				"required":             []string{"card_name"},
				"additionalProperties": false,
			},
		},
		{
			Name:        toolDeleteCard,
			Description: "Apaga um cartao cadastrado do usuario.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"card_name": map[string]any{
						"type":        "string",
						"description": "Apelido ou nome do cartao a apagar.",
					},
				},
				"required":             []string{"card_name"},
				"additionalProperties": false,
			},
		},
		{
			Name:        toolEditCategoryPct,
			Description: "Ajusta o percentual de uma categoria do orcamento mensal; rebalanceia as demais.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"category_name": map[string]any{
						"type":        "string",
						"description": "Categoria: Custo Fixo, Conhecimento, Prazeres, Metas ou Liberdade Financeira.",
					},
					"percentage": map[string]any{
						"type":        "integer",
						"description": "Novo percentual da categoria (0 a 100).",
					},
				},
				"required":             []string{"category_name", "percentage"},
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
	case toolCreateCard:
		return intent.NewCreateCard(intent.CreateCardFields{
			Nickname:   stringArg(call.ArgumentsJSON, "nickname"),
			Name:       stringArg(call.ArgumentsJSON, "name"),
			ClosingDay: int(intArg(call.ArgumentsJSON, "closing_day")),
			DueDay:     int(intArg(call.ArgumentsJSON, "due_day")),
			LimitCents: intArg(call.ArgumentsJSON, "limit_cents"),
		})
	case toolCountCards:
		return intent.NewCountCards(), nil
	case toolUpdateCard:
		return intent.NewUpdateCard(intent.UpdateCardFields{
			CardName:   stringArg(call.ArgumentsJSON, "card_name"),
			Nickname:   stringPtrArg(call.ArgumentsJSON, "new_nickname"),
			Name:       stringPtrArg(call.ArgumentsJSON, "new_name"),
			ClosingDay: intPtrArg(call.ArgumentsJSON, "new_closing_day"),
			DueDay:     intPtrArg(call.ArgumentsJSON, "new_due_day"),
		})
	case toolDeleteCard:
		return intent.NewDeleteCard(stringArg(call.ArgumentsJSON, "card_name"))
	case toolEditCategoryPct:
		return intent.NewEditCategoryPercentage(intent.EditCategoryPercentageFields{
			CategoryName: stringArg(call.ArgumentsJSON, "category_name"),
			Percentage:   int(intArg(call.ArgumentsJSON, "percentage")),
		})
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
		return intent.NewRecordIncome(intent.RecordIncomeFields{
			AmountCents:   dto.AmountCents,
			Source:        dto.Merchant,
			CategoryHint:  dto.CategoryHint,
			PaymentMethod: dto.PaymentMethod,
		})
	}
	return intent.NewRecordExpense(intent.RecordExpenseFields{
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

func stringPtrArg(args map[string]any, key string) *string {
	if args == nil {
		return nil
	}
	v, ok := args[key].(string)
	if !ok {
		return nil
	}
	trimmed := strings.TrimSpace(v)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func intPtrArg(args map[string]any, key string) *int {
	if args == nil {
		return nil
	}
	if _, ok := args[key]; !ok {
		return nil
	}
	value := int(intArg(args, key))
	return &value
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
