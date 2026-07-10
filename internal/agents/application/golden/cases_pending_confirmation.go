package golden

func pendingCases() []Case {
	return []Case{
		{
			Name:     "confirmacao de escrita apos pergunta pendente",
			Category: CategoryPending,
			Origin:   "synthetic",
			PriorTurns: []Turn{
				{UserMessage: "gastei 40 reais no mercado"},
			},
			Input:            "confirma pagamento débito",
			ToolSubset:       []string{"register_expense"},
			ExpectedTool:     "register_expense",
			ResponseProperty: nonEmptyResponse,
			ResponseDescribe: "completa dado pendente (forma de pagamento) e registra",
		},
	}
}

func confirmationCases() []Case {
	return []Case{
		{
			Name:             "verbatim relay da tool de escrita",
			Category:         CategoryConfirmation,
			Origin:           "synthetic",
			Input:            "recebi 200 reais de um freela hoje",
			ToolSubset:       []string{"register_income_confirm"},
			ExpectedTool:     "register_income",
			ResponseProperty: containsAny(RegisterExpenseConfirmMessage),
			ResponseDescribe: "resposta deve ser exatamente o texto de confirmação retornado pela tool (verbatim)",
		},
	}
}
