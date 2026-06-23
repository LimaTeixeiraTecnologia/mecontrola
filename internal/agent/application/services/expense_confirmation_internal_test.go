package services

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type ExpenseConfirmationMatchSuite struct {
	suite.Suite
}

func TestExpenseConfirmationMatchSuite(t *testing.T) {
	suite.Run(t, new(ExpenseConfirmationMatchSuite))
}

func (s *ExpenseConfirmationMatchSuite) TestMatchesExpenseConfirmation() {
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
