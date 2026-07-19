package golden

func categoryConversationCases() []Case {
	return []Case{
		{
			Name:         "categoria explicita raiz plural vira categoryText verbatim",
			Category:     CategoryExpenseIncome,
			Origin:       "prod 2026-07-19 usuário 7df14c0d (D6)",
			Input:        "Comprei ovos no Hortifruti e gastei 22 reais, coloque na categoria custos fixos e utilizei cartão de crédito XP",
			ToolSubset:   []string{"register_expense", "resolve_card", "list_cards", "classify_category", "list_categories"},
			ExpectedTool: "register_expense",
			ExpectedArgs: map[string]any{
				"categoryText": "custos fixos",
			},
			AbsentArgs:       []string{"categoryId", "subcategoryId"},
			ResponseProperty: nonEmptyResponse,
			ResponseDescribe: "registra com categoryText verbatim, sem inventar ids de categoria",
		},
		{
			Name:         "categoria explicita raiz e folha vira categoryText verbatim",
			Category:     CategoryExpenseIncome,
			Origin:       "prod 2026-07-19 usuário 7df14c0d (D2)",
			Input:        "Paguei 100 reais no abastecimento do veículo no cartão xp, coloque na categoria custo fixo e veículos",
			ToolSubset:   []string{"register_expense", "resolve_card", "list_cards", "classify_category", "list_categories"},
			ExpectedTool: "register_expense",
			ExpectedArgs: map[string]any{
				"categoryText": "custo fixo e veículos",
			},
			AbsentArgs:       []string{"categoryId", "subcategoryId"},
			ResponseProperty: nonEmptyResponse,
			ResponseDescribe: "registra com categoryText verbatim da categoria citada",
		},
		{
			Name:             "sem categoria citada omite categoryText",
			Category:         CategoryExpenseIncome,
			Origin:           "prod 2026-07-19 usuário 7df14c0d (D5)",
			Input:            "Gastei 21,57 no supermercado, utilizei o cartão XP no crédito",
			ToolSubset:       []string{"register_expense", "resolve_card", "list_cards"},
			ExpectedTool:     "register_expense",
			AbsentArgs:       []string{"categoryText", "categoryId", "subcategoryId"},
			ResponseProperty: nonEmptyResponse,
			ResponseDescribe: "resolve o 💳 e registra sem categoryText quando o usuário não cita categoria",
		},
	}
}
