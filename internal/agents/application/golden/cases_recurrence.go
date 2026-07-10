package golden

func recurrenceCases() []Case {
	return []Case{
		{
			Name:         "criar recorrencia mensal",
			Category:     CategoryRecurrence,
			Origin:       "synthetic",
			Input:        "quero criar um lançamento recorrente de academia 150 reais todo dia 10, débito",
			ToolSubset:   []string{"create_recurrence"},
			ExpectedTool: "create_recurrence",
			ExpectedArgs: map[string]any{
				"amountCents": 15000.0,
				"dayOfMonth":  10.0,
			},
			ResponseProperty: nonEmptyResponse,
			ResponseDescribe: "cria template recorrente com valor e dia corretos",
		},
		{
			Name:             "listar recorrencias",
			Category:         CategoryRecurrence,
			Origin:           "synthetic",
			Input:            "lista minhas recorrências",
			ToolSubset:       []string{"list_recurrences"},
			ExpectedTool:     "list_recurrences",
			ResponseProperty: nonEmptyResponse,
			ResponseDescribe: "listagem explícita de recorrências",
		},
		{
			Name:             "excluir recorrencia pede confirmacao",
			Category:         CategoryRecurrence,
			Origin:           "synthetic",
			Input:            "quero excluir a recorrência de id rec-001",
			ToolSubset:       []string{"delete_recurrence", "list_recurrences"},
			ExpectedTool:     "delete_recurrence",
			ResponseProperty: nonEmptyResponse,
			ResponseDescribe: "exclusão de recorrência identificada passa por confirmação antes de efetivar",
		},
	}
}
