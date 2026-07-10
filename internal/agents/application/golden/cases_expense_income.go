package golden

import (
	"strings"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
)

func expenseIncomeCases() []Case {
	return []Case{
		{
			Name:         "despesa simples debito",
			Category:     CategoryExpenseIncome,
			Origin:       "synthetic",
			Input:        "gastei 50 reais no almoço hoje, pagamento débito",
			ToolSubset:   []string{"register_expense"},
			ExpectedTool: "register_expense",
			ExpectedArgs: map[string]any{
				"amountCents": 5000.0,
			},
			ResponseProperty: nonEmptyResponse,
			ResponseDescribe: "confirma o registro da despesa em débito",
		},
		{
			Name:         "receita salario",
			Category:     CategoryExpenseIncome,
			Origin:       "synthetic",
			Input:        "recebi meu salário de 5000 reais hoje",
			ToolSubset:   []string{"register_income"},
			ExpectedTool: "register_income",
			ExpectedArgs: map[string]any{
				"amountCents": 500000.0,
			},
			ResponseProperty: nonEmptyResponse,
			ResponseDescribe: "resposta não vazia confirmando o registro",
		},
		{
			Name:             "multi item bloqueia antes do llm",
			Category:         CategoryExpenseIncome,
			Origin:           "synthetic",
			Input:            "gastei 50 reais no mercado e 30 reais no farmácia",
			ToolSubset:       []string{"register_expense"},
			NoToolExpected:   true,
			ExpectedOutcome:  agent.ToolOutcomeClarify,
			ResponseProperty: containsAny("um lançamento por vez", "um de cada vez", "separadamente"),
			ResponseDescribe: "orientação verbatim de lançamento único, sem chamar tool de escrita",
		},
		{
			Name:             "valor brasileiro nao conta como multi item",
			Category:         CategoryExpenseIncome,
			Origin:           "synthetic",
			Input:            "gastei R$ 1.234,56 no mercado hoje, pagamento débito",
			ToolSubset:       []string{"register_expense"},
			ExpectedTool:     "register_expense",
			ResponseProperty: nonEmptyResponse,
			ResponseDescribe: "valor com separador de milhar brasileiro tratado como um único valor",
		},
		{
			Name:             "despesa cartao usa cardId resolvido",
			Category:         CategoryExpenseIncome,
			Origin:           "synthetic",
			Input:            "comprei um tênis de 300 reais no cartão nubank",
			ToolSubset:       []string{"register_expense", "resolve_card", "list_cards"},
			ExpectedTools:    []string{"resolve_card", "register_expense"},
			ResponseProperty: nonEmptyResponse,
			ResponseDescribe: "resolve o cartão antes de registrar a compra",
		},
	}
}

func nonEmptyResponse(response string) bool {
	return strings.TrimSpace(response) != ""
}

func containsAny(terms ...string) ResponsePropertyFunc {
	return func(response string) bool {
		lower := strings.ToLower(response)
		for _, term := range terms {
			if strings.Contains(lower, strings.ToLower(term)) {
				return true
			}
		}
		return false
	}
}
