package golden

type AudioGroup string

const (
	AudioGroupExpense      AudioGroup = "expense"
	AudioGroupIncome       AudioGroup = "income"
	AudioGroupQuery        AudioGroup = "query"
	AudioGroupEdit         AudioGroup = "edit"
	AudioGroupConfirmation AudioGroup = "confirmation"
)

func (g AudioGroup) String() string {
	return string(g)
}

func (g AudioGroup) IsValid() bool {
	switch g {
	case AudioGroupExpense, AudioGroupIncome, AudioGroupQuery, AudioGroupEdit, AudioGroupConfirmation:
		return true
	default:
		return false
	}
}

func AllAudioGroups() []AudioGroup {
	return []AudioGroup{
		AudioGroupExpense,
		AudioGroupIncome,
		AudioGroupQuery,
		AudioGroupEdit,
		AudioGroupConfirmation,
	}
}

type AudioPairedCase struct {
	Name         string
	Group        AudioGroup
	SpokenText   string
	TextCaseName string
}

func (c AudioPairedCase) ResolveTextCase() (Case, bool) {
	for _, textCase := range AllCases() {
		if textCase.Name == c.TextCaseName {
			return textCase, true
		}
	}
	return Case{}, false
}

func AudioPairedCases() []AudioPairedCase {
	var out []AudioPairedCase
	out = append(out, audioExpenseCases()...)
	out = append(out, audioIncomeCases()...)
	out = append(out, audioQueryCases()...)
	out = append(out, audioEditCases()...)
	out = append(out, audioConfirmationCases()...)
	return out
}

func audioExpenseCases() []AudioPairedCase {
	return []AudioPairedCase{
		{
			Name:         "audio despesa almoco debito",
			Group:        AudioGroupExpense,
			SpokenText:   "gastei 50 reais no almoço hoje  pagamento débito",
			TextCaseName: "despesa simples debito",
		},
		{
			Name:         "audio despesa supermercado pix",
			Group:        AudioGroupExpense,
			SpokenText:   "gastei 50 reais no supermercado no pix",
			TextCaseName: "despesa pix nao pergunta 💳",
		},
		{
			Name:         "audio despesa padaria valor cru",
			Group:        AudioGroupExpense,
			SpokenText:   "gastei 30 na padaria",
			TextCaseName: "gastei 30 na padaria roteia sem falso multiplo",
		},
	}
}

func audioIncomeCases() []AudioPairedCase {
	return []AudioPairedCase{
		{
			Name:         "audio receita salario",
			Group:        AudioGroupIncome,
			SpokenText:   "recebi meu salário de 5000 reais hoje",
			TextCaseName: "receita salario",
		},
		{
			Name:         "audio receita salario separador milhar",
			Group:        AudioGroupIncome,
			SpokenText:   "recebi 13.874,40 de salário",
			TextCaseName: "receita salario separador milhar nao vira multiplo",
		},
		{
			Name:         "audio receita freela relay verbatim",
			Group:        AudioGroupIncome,
			SpokenText:   "recebi 200 reais de um freela hoje",
			TextCaseName: "verbatim relay da tool de escrita",
		},
	}
}

func audioQueryCases() []AudioPairedCase {
	return []AudioPairedCase{
		{
			Name:         "audio consulta gasto do mes",
			Group:        AudioGroupQuery,
			SpokenText:   "quanto gastei esse mês",
			TextCaseName: "c1 quanto gastei no mes",
		},
		{
			Name:         "audio consulta panorama",
			Group:        AudioGroupQuery,
			SpokenText:   "como estou indo",
			TextCaseName: "c2 como estou indo panorama",
		},
		{
			Name:         "audio consulta orcamento do mes",
			Group:        AudioGroupQuery,
			SpokenText:   "como está meu orçamento deste mês",
			TextCaseName: "c3 orcamento do mes",
		},
	}
}

func audioEditCases() []AudioPairedCase {
	return []AudioPairedCase{
		{
			Name:         "audio edicao freela era na verdade foi",
			Group:        AudioGroupEdit,
			SpokenText:   "aquela receita do freela era 150 reais  na verdade foi 180 reais",
			TextCaseName: "receita edicao era X na verdade foi Y",
		},
		{
			Name:         "audio edicao troca valor por termo",
			Group:        AudioGroupEdit,
			SpokenText:   "troca o valor da receita do aluguel para 950 reais",
			TextCaseName: "receita edicao troca o valor por termo",
		},
		{
			Name:         "audio edicao localizar por termo e valor",
			Group:        AudioGroupEdit,
			SpokenText:   "corrige aquela receita do salário  era 3200 reais  na verdade foi 3500",
			TextCaseName: "receita edicao localizar por termo e valor era na verdade foi",
		},
	}
}

func audioConfirmationCases() []AudioPairedCase {
	return []AudioPairedCase{
		{
			Name:         "audio confirmacao de escrita apos pergunta pendente",
			Group:        AudioGroupConfirmation,
			SpokenText:   "confirma pagamento débito",
			TextCaseName: "confirmacao de escrita apos pergunta pendente",
		},
		{
			Name:         "audio confirmacao completa data da pendencia",
			Group:        AudioGroupConfirmation,
			SpokenText:   "hoje",
			TextCaseName: "jornada hoje completa data da pendencia",
		},
		{
			Name:         "audio follow up reinvoca tool de fatura",
			Group:        AudioGroupConfirmation,
			SpokenText:   "e a fatura do meu cartão nubank",
			TextCaseName: "follow up reinvoca tool de fatura",
		},
	}
}
