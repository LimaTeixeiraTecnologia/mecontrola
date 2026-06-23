package tools

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type BudgetCancelSuite struct {
	suite.Suite
}

func TestBudgetCancelSuite(t *testing.T) {
	suite.Run(t, new(BudgetCancelSuite))
}

func (s *BudgetCancelSuite) TestMatchesBudgetCancel() {
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
		s.Run(tc.name, func() {
			s.Equal(tc.want, matchesBudgetCancel(tc.text), "matchesBudgetCancel(%q)", tc.text)
		})
	}
}
