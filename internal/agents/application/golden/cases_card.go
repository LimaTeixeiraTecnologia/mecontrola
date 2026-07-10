package golden

func cardCases() []Case {
	return []Case{
		{
			Name:             "compra no cartao resolve antes de registrar",
			Category:         CategoryCard,
			Origin:           "synthetic",
			Input:            "comprei uma bota de 250 reais no crédito do nubank",
			ToolSubset:       []string{"register_expense", "resolve_card", "list_cards"},
			ExpectedTools:    []string{"resolve_card", "register_expense"},
			ResponseProperty: nonEmptyResponse,
			ResponseDescribe: "resolve_card precede register_expense; cardId nunca fabricado do texto",
		},
		{
			Name:             "consulta fatura resolve cartao antes",
			Category:         CategoryCard,
			Origin:           "synthetic",
			Input:            "quanto está minha fatura do cartão nubank?",
			ToolSubset:       []string{"resolve_card", "list_cards", "query_card_invoice"},
			ExpectedTool:     "resolve_card",
			ResponseProperty: nonEmptyResponse,
			ResponseDescribe: "consulta de fatura por apelido primeiro resolve o cartão",
		},
		{
			Name:             "cartao nao reconhecido pede escolha",
			Category:         CategoryCard,
			Origin:           "synthetic incident-derived (agente inventando cartão ao não reconhecer apelido)",
			Input:            "comprei uma mochila de 180 reais no cartão roxinho",
			ToolSubset:       []string{"register_expense", "resolve_card_not_found", "list_cards"},
			ExpectedTool:     "resolve_card_not_found",
			ResponseProperty: containsAny("qual cartão", "cartão você quer", "escolher", "não consegui encontrar", "não encontrei", "cartões você tem", "cartões cadastrados"),
			ResponseDescribe: "cartão não reconhecido gera pedido de escolha, nunca lançamento em cartão inventado",
		},
		{
			Name:             "lista cartoes explicitamente pedida",
			Category:         CategoryCard,
			Origin:           "synthetic",
			Input:            "quais são meus cartões?",
			ToolSubset:       []string{"list_cards"},
			ExpectedTool:     "list_cards",
			ResponseProperty: nonEmptyResponse,
			ResponseDescribe: "listagem explícita usa list_cards, não é etapa preparatória",
		},
	}
}
