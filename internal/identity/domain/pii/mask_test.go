package pii_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/pii"
)

type MaskSuite struct {
	suite.Suite
}

func TestMaskSuite(t *testing.T) {
	suite.Run(t, new(MaskSuite))
}

func (s *MaskSuite) SetupTest() {}

func (s *MaskSuite) TestMaskDisplayName() {
	type args struct {
		input string
	}

	scenarios := []struct {
		name   string
		args   args
		expect func(string)
	}{
		{
			name: "deve retornar vazio para nome vazio",
			args: args{input: ""},
			expect: func(masked string) {
				s.Equal("", masked)
			},
		},
		{
			name: "deve mascarar nome ascii curto",
			args: args{input: "Jo"},
			expect: func(masked string) {
				s.Equal("J****", masked)
			},
		},
		{
			name: "deve mascarar nome com acento",
			args: args{input: "Ângela"},
			expect: func(masked string) {
				s.Equal("Â****", masked)
			},
		},
		{
			name: "deve mascarar utf8 invalido",
			args: args{input: "\xff"},
			expect: func(masked string) {
				s.Equal("****", masked)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			scenario.expect(pii.MaskDisplayName(scenario.args.input))
		})
	}
}
