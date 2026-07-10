package golden

func followUpCases() []Case {
	return []Case{
		{
			Name:     "follow up reinvoca tool de fatura",
			Category: CategoryFollowUp,
			Origin:   "synthetic incident-derived (agente respondendo follow-up de memória)",
			PriorTurns: []Turn{
				{UserMessage: "como está meu orçamento deste mês?"},
			},
			Input:            "e a fatura do meu cartão nubank?",
			ToolSubset:       []string{"query_plan", "resolve_card", "query_card_invoice"},
			ExpectedTool:     "resolve_card",
			ResponseProperty: nonEmptyResponse,
			ResponseDescribe: "follow-up reinvoca a tool correspondente em vez de responder de memória",
		},
	}
}
