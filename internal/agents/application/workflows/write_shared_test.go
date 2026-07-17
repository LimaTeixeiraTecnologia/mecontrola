package workflows

import "testing"

func TestNormalizeEntryDescription(t *testing.T) {
	scenarios := []struct {
		name  string
		input string
		want  string
	}{
		{name: "termo literal permanece intacto", input: "mercado", want: "mercado"},
		{name: "descricao com digito permanece intacta", input: "13º salário", want: "13º salário"},
		{name: "salario literal permanece", input: "salário", want: "salário"},
		{name: "prefixo compra no removido", input: "Compra no mercado", want: "mercado"},
		{name: "prefixo compra na removido", input: "Compra na farmácia", want: "farmácia"},
		{name: "prefixo recebimento do removido", input: "Recebimento do 13º salário", want: "13º salário"},
		{name: "prefixo pagamento de removido", input: "Pagamento de aluguel", want: "aluguel"},
		{name: "prefixo gasto com removido", input: "gasto com farmácia", want: "farmácia"},
		{name: "verbo comprei no removido", input: "comprei no supermercado", want: "supermercado"},
		{name: "supermercado nao e prefixo", input: "supermercado", want: "supermercado"},
		{name: "conta de luz preservada", input: "conta de luz", want: "conta de luz"},
		{name: "prefixo sem complemento nao remove", input: "compra", want: "compra"},
		{name: "espacos aparados", input: "  mercado  ", want: "mercado"},
		{name: "vazio permanece vazio", input: "", want: ""},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			if got := NormalizeEntryDescription(scenario.input); got != scenario.want {
				t.Fatalf("NormalizeEntryDescription(%q) = %q; want %q", scenario.input, got, scenario.want)
			}
		})
	}
}
