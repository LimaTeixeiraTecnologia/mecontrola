package workflow_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/workflow"
)

type MoneySuite struct {
	suite.Suite
}

func TestMoneySuite(t *testing.T) {
	suite.Run(t, new(MoneySuite))
}

func (s *MoneySuite) TestParseMoneyCents() {
	scenarios := []struct {
		name   string
		input  string
		expect int64
		ok     bool
	}{
		{name: "integer", input: "1234", expect: 123400, ok: true},
		{name: "decimal br", input: "12,34", expect: 1234, ok: true},
		{name: "decimal us", input: "12.34", expect: 1234, ok: true},
		{name: "currency br", input: "R$ 12,34", expect: 1234, ok: true},
		{name: "currency lower", input: "r$12,34", expect: 1234, ok: true},
		{name: "thousands br", input: "1.234,56", expect: 123456, ok: true},
		{name: "thousands us", input: "1,234.56", expect: 123456, ok: true},
		{name: "single decimal", input: "12,3", expect: 1230, ok: true},
		{name: "zero", input: "0", expect: 0, ok: true},
		{name: "empty", input: "", expect: 0, ok: false},
		{name: "only currency", input: "R$", expect: 0, ok: false},
		{name: "negative", input: "-10", expect: 0, ok: false},
		{name: "three decimals", input: "12,345", expect: 0, ok: false},
		{name: "invalid", input: "abc", expect: 0, ok: false},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			cents, ok := workflow.ParseMoneyCents(scenario.input)
			s.Equal(scenario.ok, ok)
			if scenario.ok {
				s.Equal(scenario.expect, cents)
			}
		})
	}
}

func (s *MoneySuite) TestParseMoneyCents_NoFloatPrecisionLoss() {
	cents, ok := workflow.ParseMoneyCents("12,34")
	s.True(ok)
	s.Equal(int64(1234), cents)
}
