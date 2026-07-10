package golden

func journeyCases() []Case {
	return []Case{
		{
			Name:             "jornada padaria dinheiro nao vira multiplo lancamento",
			Category:         CategoryExpenseIncome,
			Origin:           "synthetic journey-derived (falso multiplo lancamento em despesa simples)",
			Input:            "Gastei 10 na padaria no dinheiro",
			ToolSubset:       []string{"register_expense"},
			ExpectedTool:     "register_expense",
			ExpectedArgs:     map[string]any{"amountCents": 1000.0},
			ResponseProperty: notContainsAny("um lançamento por vez", "um de cada vez", "separadamente"),
			ResponseDescribe: "despesa simples em dinheiro registra sem receber orientação de múltiplos lançamentos",
		},
		{
			Name:             "jornada padaria pix registra valor unico",
			Category:         CategoryExpenseIncome,
			Origin:           "synthetic journey-derived (segunda despesa simples no pix)",
			Input:            "Gastei 19 na padaria no Pix",
			ToolSubset:       []string{"register_expense"},
			ExpectedTool:     "register_expense",
			ExpectedArgs:     map[string]any{"amountCents": 1900.0},
			ResponseProperty: notContainsAny("um lançamento por vez", "um de cada vez", "separadamente"),
			ResponseDescribe: "segunda despesa simples no pix registra um único valor sem orientação de múltiplos lançamentos",
		},
		{
			Name:     "jornada hoje completa data da pendencia",
			Category: CategoryPending,
			Origin:   "synthetic journey-derived (completa dado de data da pendência ativa)",
			PriorTurns: []Turn{
				{UserMessage: "Gastei 19 na padaria no Pix"},
			},
			Input:            "Hoje",
			ToolSubset:       []string{"register_expense"},
			ExpectedTool:     "register_expense",
			ResponseProperty: nonEmptyResponse,
			ResponseDescribe: "resposta 'Hoje' completa a data da pendência e prossegue para o registro/confirmação",
		},
		{
			Name:               "jornada personalizacao valida nao aplica distribuicao padrao",
			Category:           CategoryBudget,
			Origin:             "synthetic journey-derived (personalização perdida para distribuição padrão)",
			Input:              "quero personalizar meu orçamento deste mês: 2500 de custo fixo, 0 de conhecimento, 500 de prazeres, 0 de metas e 2000 de liberdade",
			ToolSubset:         []string{"create_budget", "adjust_allocation", "suggest_allocation"},
			ExpectedAnyOfTools: []string{"create_budget", "adjust_allocation"},
			ResponseProperty:   notContainsAny("4000", "40%", "distribuição padrão", "distribuicao padrao"),
			ResponseDescribe:   "personalização válida é roteada para a tool de orçamento sem propor a distribuição padrão de basis points",
		},
		{
			Name:               "jornada personalizacao invalida nao fecha cem por cento",
			Category:           CategoryBudget,
			Origin:             "synthetic journey-derived (distribuição que não fecha 100% deve pedir ajuste)",
			Input:              "meu orçamento fica assim: 3000 de custo fixo, 500 de conhecimento, 500 de prazeres, 0 de metas e 500 de liberdade",
			ToolSubset:         []string{"create_budget", "adjust_allocation", "suggest_allocation"},
			ExpectedAnyOfTools: []string{"create_budget", "adjust_allocation"},
			ResponseProperty:   nonEmptyResponse,
			ResponseDescribe:   "distribuição que não fecha 100% é roteada para a tool de orçamento, que pede ajuste em vez de ativar parcial",
		},
	}
}
