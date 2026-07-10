package golden

func toolErrorCases() []Case {
	return []Case{
		{
			Name:             "erro de tool nao inventa sucesso",
			Category:         CategoryToolError,
			Origin:           "synthetic incident-derived (agente confirmando sucesso apesar de falha de persistência)",
			Input:            "gastei 30 reais no café, débito",
			ToolSubset:       []string{"register_expense"},
			ExpectedTool:     "register_expense",
			ResponseProperty: notContainsAny("registrei com sucesso", "foi registrado", "lançamento confirmado", "registrado com sucesso"),
			ResponseDescribe: "quando a tool falha, o agente não afirma sucesso inventado",
		},
	}
}

func notContainsAny(terms ...string) ResponsePropertyFunc {
	return func(response string) bool {
		return !containsAny(terms...)(response)
	}
}
