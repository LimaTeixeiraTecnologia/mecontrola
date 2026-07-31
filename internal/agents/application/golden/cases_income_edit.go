package golden

const GoldenEditEntryDefaultImpactNote = "Este lançamento será atualizado com os novos dados. Por favor confirme."
const GoldenEditEntryMultipleCandidatesNote = "Encontrei mais de um lançamento compatível: 1) Salário — R$ 3.200,00 em 05/07; 2) Salário — R$ 3.200,00 em 20/07. Qual deles você quer editar?"
const GoldenEditEntryNotFoundNote = "Não encontrei nenhum lançamento compatível com esses critérios. Nada foi alterado."
const GoldenEditEntryInvalidAmountNote = "O novo valor informado não é válido; preciso de um valor maior que zero para corrigir."

func incomeEditCases() []Case {
	return []Case{
		{
			Name:         "receita edicao localizar por termo e valor era na verdade foi",
			Category:     CategoryExpenseIncome,
			Origin:       "RF-17/RF-18/RF-21: localizar por termo/valor e distinguir valor novo de critério de busca",
			Input:        "corrige aquela receita do salário, era 3200 reais, na verdade foi 3500",
			ToolSubset:   []string{"edit_entry"},
			ExpectedTool: "edit_entry",
			ExpectedArgs: map[string]any{
				"searchAmountCents": 320000.0,
				"amountCents":       350000.0,
			},
			ResponseProperty: containsAny(GoldenEditEntryDefaultImpactNote),
			ResponseDescribe: "distingue o valor atual (critério de busca) do valor novo pretendido e relaya a nota de impacto verbatim",
		},
		{
			Name:         "receita edicao era X na verdade foi Y",
			Category:     CategoryExpenseIncome,
			Origin:       "RF-17: frase 'era X, na verdade foi Y'",
			Input:        "aquela receita do freela era 150 reais, na verdade foi 180 reais",
			ToolSubset:   []string{"edit_entry"},
			ExpectedTool: "edit_entry",
			ExpectedArgs: map[string]any{
				"searchAmountCents": 15000.0,
				"amountCents":       18000.0,
			},
			ResponseProperty: nonEmptyResponse,
			ResponseDescribe: "frase 'era X, na verdade foi Y' edita o lançamento com searchAmountCents=X e amountCents=Y",
		},
		{
			Name:         "receita edicao troca o valor por termo",
			Category:     CategoryExpenseIncome,
			Origin:       "RF-17/RF-21: 'troca o valor' sem valor de busca explícito",
			Input:        "troca o valor da receita do aluguel para 950 reais",
			ToolSubset:   []string{"edit_entry"},
			ExpectedTool: "edit_entry",
			ExpectedArgs: map[string]any{
				"amountCents": 95000.0,
			},
			ResponseProperty: containsAny(GoldenEditEntryDefaultImpactNote),
			ResponseDescribe: "'troca o valor' preenche amountCents com o valor novo e localiza por termo (aluguel)",
		},
		{
			Name:         "receita edicao mudanca de data semana passada",
			Category:     CategoryExpenseIncome,
			Origin:       "RF-17/RF-22: mudança de data reusa a resolução determinística (task 1.0); verbatim contract (task 2.0)",
			Input:        "muda a data daquela receita do salário para semana passada",
			ToolSubset:   []string{"edit_entry"},
			ExpectedTool: "edit_entry",
			ExpectedArgs: map[string]any{
				"occurredAt": "semana passada",
			},
			ResponseProperty: nonEmptyResponse,
			ResponseDescribe: "'muda a data' repassa a expressão relativa verbatim (semana passada), resolvida deterministicamente rio abaixo pela mesma ParseInputDate de RF-05/RF-06 (task 1.0), nunca ISO nem dia corrente",
		},
		{
			Name:         "receita edicao mudanca de data dia especifico",
			Category:     CategoryExpenseIncome,
			Origin:       "RF-17/RF-22: 'foi dia N' com dia-do-mês isolado",
			Input:        "corrige a data daquela receita de comissão, foi dia 10",
			ToolSubset:   []string{"edit_entry"},
			ExpectedTool: "edit_entry",
			ExpectedArgs: map[string]any{
				"occurredAt": "dia 10",
			},
			ResponseProperty: nonEmptyResponse,
			ResponseDescribe: "'foi dia 10' repassa a expressão verbatim (dia 10) sem calcular a data no LLM",
		},
		{
			Name:             "receita edicao mais de um candidato apresenta opcoes",
			Category:         CategoryExpenseIncome,
			Origin:           "RF-19: mais de um candidato compatível apresenta as opções para o usuário escolher",
			Input:            "corrige aquela receita de salário, era 3200 reais",
			ToolSubset:       []string{"edit_entry_multiple_candidates"},
			ExpectedTool:     "edit_entry",
			ResponseProperty: containsAny(GoldenEditEntryMultipleCandidatesNote),
			ResponseDescribe: "havendo mais de um candidato compatível, o agente relaya verbatim as opções para o usuário escolher",
		},
		{
			Name:         "receita edicao candidato unico mostra nota de impacto antes de confirmar",
			Category:     CategoryExpenseIncome,
			Origin:       "RF-19/RF-20: um único candidato apresenta a nota de impacto e pede confirmação antes de gravar",
			Input:        "o valor certo daquela receita de aluguel é 980 reais e não 950 reais",
			ToolSubset:   []string{"edit_entry"},
			ExpectedTool: "edit_entry",
			ExpectedArgs: map[string]any{
				"searchAmountCents": 95000.0,
				"amountCents":       98000.0,
			},
			ResponseProperty: allOf(
				containsAny(GoldenEditEntryDefaultImpactNote),
				notContainsAny("atualizei", "atualizado com sucesso", "pronto, já corrigi"),
			),
			ResponseDescribe: "candidato único mostra a nota de impacto e pede confirmação, sem afirmar que já atualizou antes do sim",
		},
		{
			Name:             "receita edicao nenhum candidato compativel nao altera nada",
			Category:         CategoryExpenseIncome,
			Origin:           "RF-24: nenhuma receita compatível encontrada informa sem alterar nada",
			Input:            "corrige aquela receita de 99999 reais que eu nunca lancei",
			ToolSubset:       []string{"edit_entry_not_found"},
			ExpectedTool:     "edit_entry",
			ResponseProperty: containsAny(GoldenEditEntryNotFoundNote),
			ResponseDescribe: "sem candidato compatível, o agente informa verbatim que nada foi encontrado/alterado",
		},
		{
			Name:             "receita edicao correcao invalida nao persiste e orienta",
			Category:         CategoryExpenseIncome,
			Origin:           "RF-24: correção inválida (novo valor <= 0) não persiste e orienta a correção",
			Input:            "muda aquela receita do freela para 0 reais",
			ToolSubset:       []string{"edit_entry_invalid_amount"},
			ExpectedTool:     "edit_entry",
			ResponseProperty: containsAny(GoldenEditEntryInvalidAmountNote),
			ResponseDescribe: "correção inválida (valor <= 0) não persiste e orienta o usuário a informar um valor correto",
		},
	}
}
