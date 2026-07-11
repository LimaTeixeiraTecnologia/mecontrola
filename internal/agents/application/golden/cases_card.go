package golden

func cardCases() []Case {
	return []Case{
		{
			Name:             "compra no 💳 resolve antes de registrar",
			Category:         CategoryCard,
			Origin:           "synthetic",
			Input:            "comprei uma bota de 250 reais no crédito do nubank",
			ToolSubset:       []string{"register_expense", "resolve_card", "list_cards"},
			ExpectedTools:    []string{"resolve_card", "register_expense"},
			ResponseProperty: nonEmptyResponse,
			ResponseDescribe: "resolve_card precede register_expense; cardId nunca fabricado do texto",
		},
		{
			Name:             "consulta fatura resolve 💳 antes",
			Category:         CategoryCard,
			Origin:           "synthetic",
			Input:            "quanto está minha fatura do 💳 nubank?",
			ToolSubset:       []string{"resolve_card", "list_cards", "query_card_invoice"},
			ExpectedTool:     "resolve_card",
			ResponseProperty: nonEmptyResponse,
			ResponseDescribe: "consulta de fatura por apelido primeiro resolve o 💳",
		},
		{
			Name:         "💳 nao reconhecido pede escolha",
			Category:     CategoryCard,
			Origin:       "synthetic incident-derived (agente inventando 💳 ao não reconhecer apelido)",
			Input:        "comprei uma mochila de 180 reais no 💳 roxinho",
			ToolSubset:   []string{"register_expense", "resolve_card_not_found", "list_cards"},
			ExpectedTool: "resolve_card_not_found",
			ResponseProperty: allOf(
				containsAny("💳"),
				notContainsAny("qual cartão", "cartão você quer", "cartões você tem", "cartões cadastrados"),
			),
			ResponseDescribe: "💳 não reconhecido gera pedido de escolha com emoji, nunca lançamento em 💳 inventado",
		},
		{
			Name:             "lista cartoes explicitamente pedida",
			Category:         CategoryCard,
			Origin:           "synthetic",
			Input:            "quais são meus 💳?",
			ToolSubset:       []string{"list_cards"},
			ExpectedTool:     "list_cards",
			ResponseProperty: allOf(nonEmptyResponse, containsAny("💳")),
			ResponseDescribe: "listagem explícita usa list_cards, não é etapa preparatória, e resposta usa 💳",
		},
		{
			Name:         "cadastro 💳 banco apelido unico",
			Category:     CategoryCard,
			Origin:       "synthetic journey-derived (RF-11/RF-12: banco/apelido único preenche ambos)",
			Input:        "cadastra meu 💳 Nubank, vencimento dia 1",
			ToolSubset:   []string{"create_card"},
			ExpectedTool: "create_card",
			ExpectedArgs: map[string]any{
				"nickname": "Nubank",
				"bank":     "Nubank",
				"dueDay":   1.0,
			},
			ResponseProperty: allOf(nonEmptyResponse, containsAny("💳")),
			ResponseDescribe: "cadastro de 💳 em linguagem natural extrai banco/apelido único e vencimento",
		},
		{
			Name:           "recusa cadastro 💳 nao cria 💳",
			Category:       CategoryCard,
			Origin:         "synthetic journey-derived (RF-14: usuário pode recusar 💳 sem bloqueio)",
			Input:          "não quero cadastrar 💳 agora",
			ToolSubset:     []string{"create_card", "list_cards", "count_cards"},
			NoToolExpected: true,
			ResponseProperty: allOf(
				nonEmptyResponse,
				notContainsAny("cadastrei", "cadastrado com sucesso"),
			),
			ResponseDescribe: "recusa de cadastro de 💳 não chama ferramenta de criação nem afirma sucesso falso",
		},
	}
}
