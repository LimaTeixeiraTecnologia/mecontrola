package golden

func onboardingCases() []Case {
	return []Case{
		{
			Name:             "usuario novo cumprimenta",
			Category:         CategoryOnboarding,
			Origin:           "synthetic",
			Input:            "oi",
			ToolSubset:       []string{"register_expense"},
			NoToolExpected:   true,
			ResponseProperty: nonEmptyResponse,
			ResponseDescribe: "responde de forma acolhedora sem exigir tool para uma saudação simples",
		},
		{
			Name:           "onboarding inicial sem oi combina boas vindas e objetivo",
			Category:       CategoryOnboarding,
			Origin:         "synthetic journey-derived (RF-01/RF-02/RF-03: primeira resposta não exige 'Oi')",
			Input:          "Quero começar a usar o MeControla",
			ToolSubset:     []string{"register_expense"},
			NoToolExpected: true,
			ResponseProperty: allOf(
				containsAny("Bem-vindo", "bem-vindo"),
				containsAny("objetivo financeiro", "principal objetivo"),
			),
			ResponseDescribe: "primeira interação de onboarding responde com boas-vindas e pergunta o objetivo financeiro, sem exigir 'Oi'",
		},
		{
			Name:           "guard de gatilho inicial nao regressa com valor por extenso na mensagem",
			Category:       CategoryOnboarding,
			Origin:         "synthetic (PRD onboarding-valor-extenso-confirmacao-meta, subtarefa 3.3: guarda de não regressão do gatilho determinístico onboarding_initial — internal/agents/application/agents/guards/onboarding_initial.go — quando a primeira mensagem já combina objetivo e valor por extenso; NÃO exercita BuildGoalStep/BuildMonthlyBudgetStep do workflow de onboarding, cobertos por real-LLM em onboarding_workflow_integration_test.go)",
			Input:          "Quero começar a usar o MeControla, meu objetivo é juntar uma reserva de dez mil reais",
			ToolSubset:     []string{"register_expense"},
			NoToolExpected: true,
			ResponseProperty: allOf(
				containsAny("Bem-vindo", "bem-vindo"),
				containsAny("objetivo financeiro", "principal objetivo"),
			),
			ResponseDescribe: "guard determinístico de gatilho inicial de onboarding continua classificando a mensagem como início de onboarding e devolvendo a mensagem estática de boas-vindas/objetivo (sem chamar ferramenta) quando o objetivo e um valor por extenso chegam juntos na primeira mensagem",
		},
		{
			Name:           "guard de gatilho inicial nao regressa com meta e valor mascarado juntos",
			Category:       CategoryOnboarding,
			Origin:         "synthetic (PRD onboarding-valor-extenso-confirmacao-meta, subtarefa 3.3: guarda de não regressão do gatilho determinístico onboarding_initial quando meta+valor em R$ chegam juntos na primeira mensagem; NÃO exercita BuildGoalStep/BuildMonthlyBudgetStep do workflow de onboarding, cobertos por real-LLM em onboarding_workflow_integration_test.go)",
			Input:          "Quero começar a usar o MeControla, comprar um celular, meta de R$ 5.000",
			ToolSubset:     []string{"register_expense"},
			NoToolExpected: true,
			ResponseProperty: allOf(
				containsAny("Bem-vindo", "bem-vindo"),
				containsAny("objetivo financeiro", "principal objetivo"),
			),
			ResponseDescribe: "guard determinístico de gatilho inicial de onboarding continua classificando a mensagem como início de onboarding e devolvendo a mensagem estática de boas-vindas/objetivo (sem chamar ferramenta nem o LLM) quando meta e valor mascarado em R$ chegam juntos na primeira mensagem",
		},
	}
}
