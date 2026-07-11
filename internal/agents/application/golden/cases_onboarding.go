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
	}
}
