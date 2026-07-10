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
	}
}
