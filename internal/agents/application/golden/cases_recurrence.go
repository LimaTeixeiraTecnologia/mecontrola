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
		{
			Name:         "editar valor de recorrencia por nome (incidente 2026-07-30)",
			Category:     CategoryRecurrence,
			Origin:       "incident-2026-07-30",
			Input:        "muda o aluguel para 1600",
			ToolSubset:   []string{"update_recurrence", "delete_recurrence", "list_recurrences"},
			ExpectedTool: "update_recurrence",
			ExpectedArgs: map[string]any{
				"templateId": "2dc23397-5551-494a-8085-854fa96858a3",
			},
			ResponseProperty: nonEmptyResponse,
			ResponseDescribe: "resolve 'aluguel' à recorrência correspondente e pede confirmação antes de atualizar o valor",
		},
		{
			Name:         "cancelar recorrencia por nome sem ambiguidade (incidente 2026-07-30)",
			Category:     CategoryRecurrence,
			Origin:       "incident-2026-07-30",
			Input:        "cancela a recorrência da pensão",
			ToolSubset:   []string{"update_recurrence", "delete_recurrence", "list_recurrences"},
			ExpectedTool: "delete_recurrence",
			ExpectedArgs: map[string]any{
				"templateId": "f7e70454-3662-472a-bacb-a2932682b5bb",
			},
			ResponseProperty: nonEmptyResponse,
			ResponseDescribe: "resolve 'pensão' à recorrência correspondente e pede confirmação antes de excluir",
		},
		{
			Name:         "cancelar recorrencia ambigua desambiguada pelo dia (incidente 2026-07-30)",
			Category:     CategoryRecurrence,
			Origin:       "incident-2026-07-30",
			Input:        "cancela a recorrência do salário do dia 5",
			ToolSubset:   []string{"update_recurrence", "delete_recurrence", "list_recurrences"},
			ExpectedTool: "delete_recurrence",
			ExpectedArgs: map[string]any{
				"templateId": "a83be4e3-cb07-435e-96fc-a53ded2ef65b",
			},
			ResponseProperty: nonEmptyResponse,
			ResponseDescribe: "duas recorrências de salário (dia 5 e dia 31); resolve pela desambiguação de dia citada na mensagem",
		},
	}
}
