package steps

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type MatchHelpersSuite struct {
	suite.Suite
}

func TestMatchHelpersSuite(t *testing.T) {
	suite.Run(t, new(MatchHelpersSuite))
}

func (s *MatchHelpersSuite) TestMatchesExpenseConfirmation() {
	type tc struct {
		text string
		want bool
	}
	cases := []tc{
		{text: "sim", want: true},
		{text: "Sim", want: true},
		{text: "SIM", want: true},
		{text: "s", want: true},
		{text: "confirma", want: true},
		{text: "confirmado", want: true},
		{text: "pode", want: true},
		{text: "ok", want: true},
		{text: "yes", want: true},
		{text: "sim, registrar com essa categoria", want: true},
		{text: "Sim, pode registrar", want: true},
		{text: "ok, pode ser", want: true},
		{text: "confirma, essa categoria mesmo", want: true},
		{text: "sim pode", want: true},
		{text: "não", want: false},
		{text: "nao", want: false},
		{text: "simples demais", want: false},
		{text: "similarity", want: false},
		{text: "okayyyy", want: false},
		{text: "", want: false},
		{text: "cancela", want: false},
	}
	for _, tc := range cases {
		s.Run(tc.text, func() {
			s.Equal(tc.want, matchesExpenseConfirmation(tc.text))
		})
	}
}

func (s *MatchHelpersSuite) TestMatchesExpenseCancellation() {
	type tc struct {
		text string
		want bool
	}
	cases := []tc{
		{text: "não", want: true},
		{text: "nao", want: true},
		{text: "n", want: true},
		{text: "no", want: true},
		{text: "cancela", want: true},
		{text: "cancelar", want: true},
		{text: "sim", want: false},
		{text: "", want: false},
	}
	for _, tc := range cases {
		s.Run(tc.text, func() {
			s.Equal(tc.want, matchesExpenseCancellation(tc.text))
		})
	}
}

func (s *MatchHelpersSuite) TestMatchCandidateByText() {
	candidates := []string{"Alimentação > Supermercado", "Transporte > Combustível", "Prazeres > Academia"}
	type tc struct {
		text string
		want string
	}
	cases := []tc{
		{text: "1", want: "Alimentação > Supermercado"},
		{text: "2", want: "Transporte > Combustível"},
		{text: "3", want: "Prazeres > Academia"},
		{text: "alimen", want: "Alimentação > Supermercado"},
		{text: "transporte", want: "Transporte > Combustível"},
		{text: "prazeres", want: "Prazeres > Academia"},
		{text: "", want: ""},
		{text: "inexistente", want: ""},
		{text: "4", want: ""},
	}
	for _, tc := range cases {
		s.Run(tc.text, func() {
			s.Equal(tc.want, matchCandidateByText(tc.text, candidates))
		})
	}
}
