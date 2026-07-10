package golden

func ambiguityCases() []Case {
	return []Case{
		{
			Name:             "mes por nome sem ano pede clarificacao",
			Category:         CategoryAmbiguity,
			Origin:           "synthetic",
			Input:            "quanto gastei em junho?",
			ToolSubset:       []string{"query_month_ask_year"},
			ExpectedTool:     "query_month",
			ResponseProperty: containsAny("ano"),
			ResponseDescribe: "mês nomeado sem ano pergunta o ano em vez de assumir (verbatim do clarifyPrompt)",
		},
	}
}

func whatsAppFormatCases() []Case {
	return []Case{
		{
			Name:             "resposta sem markdown duplo asterisco",
			Category:         CategoryWhatsAppFormat,
			Origin:           "synthetic",
			Input:            "como está meu orçamento deste mês?",
			ToolSubset:       []string{"query_plan"},
			ExpectedTool:     "query_plan",
			ResponseProperty: notContainsAny("**"),
			ResponseDescribe: "resposta usa asterisco simples do WhatsApp, nunca ** duplo",
		},
	}
}

func noInternalTermsCases() []Case {
	return []Case{
		{
			Name:             "resposta sem termos internos vazados",
			Category:         CategoryNoInternalTerms,
			Origin:           "synthetic incident-derived (termos internos vazando para o usuário)",
			Input:            "gastei 30 reais no café, débito",
			ToolSubset:       []string{"register_expense"},
			ExpectedTool:     "register_expense",
			ResponseProperty: notContainsAny("workflow", "thread", "run", "correlation", "infraestrutura", "sistema interno", "usecase"),
			ResponseDescribe: "resposta não expõe termos técnicos internos ao usuário final",
		},
	}
}
