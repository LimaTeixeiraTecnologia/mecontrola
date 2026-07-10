package golden

func budgetCases() []Case {
	return []Case{
		{
			Name:         "criar orcamento mes atual",
			Category:     CategoryBudget,
			Origin:       "synthetic",
			Input:        "quero criar um orçamento para esse mês com total de R$ 3.500,00",
			ToolSubset:   []string{"create_budget"},
			ExpectedTool: "create_budget",
			ExpectedArgs: map[string]any{
				"monthRefKind": "current",
			},
			ResponseProperty: nonEmptyResponse,
			ResponseDescribe: "cria orçamento do mês atual com monthRefKind=current",
		},
		{
			Name:         "criar orcamento mes explicito com ano",
			Category:     CategoryBudget,
			Origin:       "synthetic",
			Input:        "cria um orçamento pra junho de 2026",
			ToolSubset:   []string{"create_budget"},
			ExpectedTool: "create_budget",
			ExpectedArgs: map[string]any{
				"monthRefKind": "explicit",
			},
			ResponseProperty: nonEmptyResponse,
			ResponseDescribe: "mês explícito com ano usa monthRefKind=explicit",
		},
		{
			Name:         "mes sem ano pede clarificacao",
			Category:     CategoryBudget,
			Origin:       "synthetic incident-derived (agente assumindo ano indevidamente)",
			Input:        "quero criar o orçamento de junho",
			ToolSubset:   []string{"create_budget"},
			ExpectedTool: "create_budget",
			ExpectedArgs: map[string]any{
				"monthRefKind": "named_without_year",
			},
			ResponseProperty: containsAny("ano"),
			ResponseDescribe: "mês nomeado sem ano gera monthRefKind=named_without_year e pergunta o ano, sem assumir",
		},
		{
			Name:             "orcamento nao encontrado oferece criacao",
			Category:         CategoryBudget,
			Origin:           "synthetic",
			Input:            "como foi meu mês de maio de 2026?",
			ToolSubset:       []string{"query_plan_not_found"},
			ExpectedTool:     "query_plan",
			ResponseProperty: containsAny("ajudar a criar", "criar um orçamento", "criar seu orçamento", "posso te ajudar"),
			ResponseDescribe: "quando não há orçamento, oferece criar (offerCreatePrompt verbatim)",
		},
	}
}
