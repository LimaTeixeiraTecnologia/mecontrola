package usecases

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type FunnelTokenSuite struct {
	suite.Suite
}

func TestFunnelTokenSuite(t *testing.T) {
	suite.Run(t, new(FunnelTokenSuite))
}

func (s *FunnelTokenSuite) SetupTest() {}

func (s *FunnelTokenSuite) TestExtractFunnelToken() {
	type args struct {
		sck string
		s1  string
		src string
	}

	scenarios := []struct {
		name   string
		args   args
		expect func(result string)
	}{
		{
			name: "deve priorizar sck sobre s1 e src",
			args: args{sck: "token-sck", s1: "token-s1", src: "token-src"},
			expect: func(result string) {
				s.Equal("token-sck", result)
			},
		},
		{
			name: "deve priorizar sck sobre src quando s1 estiver vazio",
			args: args{sck: "token-sck", src: "token-src"},
			expect: func(result string) {
				s.Equal("token-sck", result)
			},
		},
		{
			name: "deve usar s1 quando sck estiver vazio",
			args: args{s1: "token-s1", src: "token-src"},
			expect: func(result string) {
				s.Equal("token-s1", result)
			},
		},
		{
			name: "deve usar src quando sck e s1 estiverem vazios",
			args: args{src: "token-src"},
			expect: func(result string) {
				s.Equal("token-src", result)
			},
		},
		{
			name: "deve retornar vazio quando todos os campos estiverem vazios",
			args: args{},
			expect: func(result string) {
				s.Empty(result)
			},
		},
		{
			name: "deve retornar apenas sck quando for o unico presente",
			args: args{sck: "only-sck"},
			expect: func(result string) {
				s.Equal("only-sck", result)
			},
		},
		{
			name: "deve retornar apenas s1 quando for o unico presente",
			args: args{s1: "only-s1"},
			expect: func(result string) {
				s.Equal("only-s1", result)
			},
		},
		{
			name: "deve retornar apenas src quando for o unico presente",
			args: args{src: "only-src"},
			expect: func(result string) {
				s.Equal("only-src", result)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			result := ExtractFunnelTokenForTest(
				scenario.args.sck,
				scenario.args.s1,
				scenario.args.src,
			)
			scenario.expect(result)
		})
	}
}
