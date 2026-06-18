package services

import "testing"

func TestMatchesBudgetCancel(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		text string
		want bool
	}{
		{name: "exact_cancelar", text: "cancelar", want: true},
		{name: "cancela", text: "cancela isso", want: true},
		{name: "deixa_pra_la_accent", text: "deixa pra lá", want: true},
		{name: "deixa_pra_la_no_accent", text: "deixa pra la por favor", want: true},
		{name: "esquece", text: "esquece", want: true},
		{name: "parar", text: "quero parar", want: true},
		{name: "uppercase", text: "CANCELAR", want: true},
		{name: "padded", text: "  cancelar  ", want: true},
		{name: "empty", text: "   ", want: false},
		{name: "category_value", text: "custos fixos 35%", want: false},
		{name: "income", text: "ganho 5 mil por mês", want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := matchesBudgetCancel(tc.text); got != tc.want {
				t.Fatalf("matchesBudgetCancel(%q) = %v, want %v", tc.text, got, tc.want)
			}
		})
	}
}
