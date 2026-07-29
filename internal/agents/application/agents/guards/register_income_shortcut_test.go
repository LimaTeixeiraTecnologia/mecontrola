package guards

import (
	"testing"
)

func TestParseRegisterIncomeShortcut(t *testing.T) {
	scenarios := []struct {
		name    string
		input   string
		wantOK  bool
		wantAmt int64
		wantDsc string
	}{
		{name: "receita avulsa", input: "recebi 500 de freelance", wantOK: true, wantAmt: 50000, wantDsc: "freelance"},
		{name: "milhar com centavos", input: "caiu R$ 1.250,90 de reembolso", wantOK: true, wantAmt: 125090, wantDsc: "reembolso"},
		{name: "producao todo dia 5 eu recebo salario nao dispara", input: "todo dia 5 eu recebo R$ 13.874,40 de salário", wantOK: false},
		{name: "todo mes nao dispara", input: "todo mês eu recebo R$ 13.874,40 de salário", wantOK: false},
		{name: "todo mes sem acento nao dispara", input: "todo mes caiu 1000 de pensão", wantOK: false},
		{name: "mensalmente nao dispara", input: "mensalmente caiu 800 de aluguel", wantOK: false},
		{name: "toda semana nao dispara", input: "toda semana entrou 200 de bico", wantOK: false},
		{name: "anualmente nao dispara", input: "anualmente caiu 5000 de bonificação", wantOK: false},
		{name: "sem gatilho de receita", input: "gastei 30 no mercado", wantOK: false},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			args, ok := parseRegisterIncomeShortcut(scenario.input, nil)
			if ok != scenario.wantOK {
				t.Fatalf("parseRegisterIncomeShortcut(%q) ok = %v; want %v", scenario.input, ok, scenario.wantOK)
			}
			if !scenario.wantOK {
				return
			}
			amt, amtOK := args["amountCents"].(int64)
			if !amtOK || amt != scenario.wantAmt {
				t.Fatalf("amountCents = %v; want %d", args["amountCents"], scenario.wantAmt)
			}
			if dsc, _ := args["description"].(string); dsc != scenario.wantDsc {
				t.Fatalf("description = %q; want %q", args["description"], scenario.wantDsc)
			}
		})
	}
}
