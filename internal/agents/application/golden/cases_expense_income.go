package golden

import (
	"strings"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/messages"
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
			Name:             "despesa 💳 usa cardId resolvido",
			Category:         CategoryExpenseIncome,
			Origin:           "synthetic",
			Input:            "comprei um tênis de 300 reais no 💳 nubank",
			ToolSubset:       []string{"register_expense", "resolve_card", "list_cards"},
			ExpectedTools:    []string{"resolve_card", "register_expense"},
			ResponseProperty: nonEmptyResponse,
			ResponseDescribe: "resolve o 💳 antes de registrar a compra",
		},
		{
			Name:         "despesa pix nao pergunta 💳",
			Category:     CategoryExpenseIncome,
			Origin:       "synthetic journey-derived (RF-16/RF-17: pix não depende de 💳)",
			Input:        "gastei R$ 50,00 no supermercado no pix",
			ToolSubset:   []string{"register_expense", "resolve_card", "list_cards"},
			ExpectedTool: "register_expense",
			ExpectedArgs: map[string]any{
				"amountCents":   5000.0,
				"paymentMethod": "pix",
			},
			ResponseProperty: allOf(
				nonEmptyResponse,
				notContainsAny("qual 💳", "qual 💳", "💳 você quer", "💳 você quer", "escolher", "💳 cadastrados"),
			),
			ResponseDescribe: "despesa pix chega a register_expense sem pergunta de 💳",
		},
		{
			Name:         "receita salario separador milhar nao vira multiplo",
			Category:     CategoryExpenseIncome,
			Origin:       "synthetic journey-derived (RF-20/RF-21: separador de milhar não vira múltiplos lançamentos)",
			Input:        "Recebi R$ 13.874,40 de salário",
			ToolSubset:   []string{"register_income"},
			ExpectedTool: "register_income",
			ExpectedArgs: map[string]any{
				"amountCents": 1387440.0,
				"description": "salário",
			},
			ResponseProperty: allOf(
				nonEmptyResponse,
				notContainsAny("um lançamento por vez", "um de cada vez", "separadamente", "mais de um lançamento"),
			),
			ResponseDescribe: "receita com separador de milhar registra valor único e preserva descrição literal",
		},
		{
			Name:         "valor cru sem reais roteia para register_expense sem falso multiplo",
			Category:     CategoryExpenseIncome,
			Origin:       "producao (+5511930111763, 2026-07-17): \"Gastei 500 no mercado\" disparava falso aviso de múltiplos lançamentos e bloqueava o registro",
			Input:        "Gastei 500 no mercado",
			ToolSubset:   []string{"register_expense", "register_income"},
			ExpectedTool: "register_expense",
			ExpectedArgs: map[string]any{
				"amountCents": 50000.0,
			},
			ResponseProperty: allOf(
				nonEmptyResponse,
				notContainsAny("mais de um lançamento", "um de cada vez", "um lançamento por vez", "separadamente"),
			),
			ResponseDescribe: "valor cru único roteia para register_expense (o workflow pede a forma de pagamento), nunca aviso de múltiplos lançamentos",
		},
		{
			Name:         "gastei 20 no cinema roteia sem falso multiplo",
			Category:     CategoryExpenseIncome,
			Origin:       "producao (+5511930111763, 2026-07-17): \"Gastei 20 no cinema\" recebia falso aviso de múltiplos lançamentos",
			Input:        "Gastei 20 no cinema",
			ToolSubset:   []string{"register_expense", "register_income"},
			ExpectedTool: "register_expense",
			ExpectedArgs: map[string]any{
				"amountCents": 2000.0,
			},
			ResponseProperty: allOf(
				nonEmptyResponse,
				notContainsAny("mais de um lançamento", "um de cada vez", "um lançamento por vez", "separadamente"),
			),
			ResponseDescribe: "valor cru único de lazer roteia para register_expense, nunca aviso de múltiplos lançamentos",
		},
		{
			Name:         "despesa sem forma de pagamento delega pergunta ao workflow",
			Category:     CategoryExpenseIncome,
			Origin:       "producao (+5511930111763, 2026-07-17): clarificação de pagamento é determinística do workflow; LLM não inventa paymentMethod nem formula a pergunta",
			Input:        "gastei 50 no mercado hoje",
			ToolSubset:   []string{"register_expense_payment_clarify"},
			ExpectedTool: "register_expense",
			AbsentArgs:   []string{"paymentMethod"},
			ExpectedArgs: map[string]any{
				"amountCents": 5000.0,
			},
			ResponseProperty: containsAny(messages.ClarificationQuestion(messages.MissingFieldPaymentMethod)),
			ResponseDescribe: "chama register_expense sem paymentMethod e repassa verbatim a pergunta determinística de forma de pagamento",
		},
		{
			Name:         "despesa debito preserva paymentMethod dito",
			Category:     CategoryExpenseIncome,
			Origin:       "synthetic (anti-regressão: remoção do required não pode derrubar o preenchimento quando o usuário informa)",
			Input:        "gastei 80 na farmácia no débito",
			ToolSubset:   []string{"register_expense"},
			ExpectedTool: "register_expense",
			ExpectedArgs: map[string]any{
				"amountCents":   8000.0,
				"paymentMethod": "debit_card",
			},
			ResponseProperty: nonEmptyResponse,
			ResponseDescribe: "usuário informou débito; a tool recebe paymentMethod=debit_card",
		},
		{
			Name:         "gastei 30 na padaria roteia sem falso multiplo",
			Category:     CategoryExpenseIncome,
			Origin:       "producao (+5511930111763, 2026-07-17): \"Gastei 30 na padaria\" recebia falso aviso de múltiplos lançamentos mesmo após o commit 825372d",
			Input:        "Gastei 30 na padaria",
			ToolSubset:   []string{"register_expense", "register_income"},
			ExpectedTool: "register_expense",
			ExpectedArgs: map[string]any{
				"amountCents": 3000.0,
			},
			ResponseProperty: allOf(
				nonEmptyResponse,
				notContainsAny("mais de um lançamento", "um de cada vez", "um lançamento por vez", "separadamente"),
			),
			ResponseDescribe: "valor cru único de padaria roteia para register_expense, nunca aviso de múltiplos lançamentos",
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

func allOf(funcs ...ResponsePropertyFunc) ResponsePropertyFunc {
	return func(response string) bool {
		for _, f := range funcs {
			if !f(response) {
				return false
			}
		}
		return true
	}
}
