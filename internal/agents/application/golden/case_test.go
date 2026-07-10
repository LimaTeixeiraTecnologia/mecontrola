package golden

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
)

type CaseSchemaSuite struct {
	suite.Suite
}

func TestCaseSchemaSuite(t *testing.T) {
	suite.Run(t, new(CaseSchemaSuite))
}

func (s *CaseSchemaSuite) TestValidate() {
	type args struct {
		c Case
	}

	scenarios := []struct {
		name   string
		args   args
		expect func(err error)
	}{
		{
			name: "deve validar caso completo com expectedTool",
			args: args{c: Case{
				Name:             "caso valido",
				Category:         CategoryQuery,
				Input:            "quanto gastei esse mês?",
				ExpectedTool:     "query_month",
				ExpectedOutcome:  agent.ToolOutcomeRouted,
				ResponseProperty: nonEmptyResponse,
				ResponseDescribe: "resposta não vazia",
			}},
			expect: func(err error) {
				s.NoError(err)
			},
		},
		{
			name: "deve validar caso completo com expectedTools",
			args: args{c: Case{
				Name:             "caso valido multi tool",
				Category:         CategoryQuery,
				Input:            "como estou indo?",
				ExpectedTools:    []string{"query_month", "query_plan"},
				ExpectedOutcome:  agent.ToolOutcomeRouted,
				ResponseProperty: nonEmptyResponse,
				ResponseDescribe: "resposta não vazia",
			}},
			expect: func(err error) {
				s.NoError(err)
			},
		},
		{
			name: "deve validar caso completo com noToolExpected",
			args: args{c: Case{
				Name:             "caso valido sem tool",
				Category:         CategoryOnboarding,
				Input:            "oi",
				NoToolExpected:   true,
				ExpectedOutcome:  agent.ToolOutcomeRouted,
				ResponseProperty: nonEmptyResponse,
				ResponseDescribe: "resposta não vazia",
			}},
			expect: func(err error) {
				s.NoError(err)
			},
		},
		{
			name: "deve rejeitar caso sem name",
			args: args{c: Case{
				Category:         CategoryQuery,
				Input:            "quanto gastei?",
				ExpectedTool:     "query_month",
				ResponseProperty: nonEmptyResponse,
				ResponseDescribe: "resposta não vazia",
			}},
			expect: func(err error) {
				s.Error(err)
				s.Contains(err.Error(), "name")
			},
		},
		{
			name: "deve rejeitar categoria invalida",
			args: args{c: Case{
				Name:             "categoria invalida",
				Category:         Category("invalid"),
				Input:            "quanto gastei?",
				ExpectedTool:     "query_month",
				ResponseProperty: nonEmptyResponse,
				ResponseDescribe: "resposta não vazia",
			}},
			expect: func(err error) {
				s.Error(err)
				s.Contains(err.Error(), "category")
			},
		},
		{
			name: "deve rejeitar caso sem input",
			args: args{c: Case{
				Name:             "sem input",
				Category:         CategoryQuery,
				ExpectedTool:     "query_month",
				ResponseProperty: nonEmptyResponse,
				ResponseDescribe: "resposta não vazia",
			}},
			expect: func(err error) {
				s.Error(err)
				s.Contains(err.Error(), "input")
			},
		},
		{
			name: "deve rejeitar caso sem nenhuma expectativa de tool",
			args: args{c: Case{
				Name:             "sem expectativa de tool",
				Category:         CategoryQuery,
				Input:            "quanto gastei?",
				ResponseProperty: nonEmptyResponse,
				ResponseDescribe: "resposta não vazia",
			}},
			expect: func(err error) {
				s.Error(err)
				s.Contains(err.Error(), "expectedTool")
			},
		},
		{
			name: "deve rejeitar expectedTool e expectedTools coexistindo",
			args: args{c: Case{
				Name:             "conflito de expectativas",
				Category:         CategoryQuery,
				Input:            "quanto gastei?",
				ExpectedTool:     "query_month",
				ExpectedTools:    []string{"query_plan"},
				ResponseProperty: nonEmptyResponse,
				ResponseDescribe: "resposta não vazia",
			}},
			expect: func(err error) {
				s.Error(err)
			},
		},
		{
			name: "deve rejeitar caso sem responseProperty",
			args: args{c: Case{
				Name:             "sem response property",
				Category:         CategoryQuery,
				Input:            "quanto gastei?",
				ExpectedTool:     "query_month",
				ResponseDescribe: "resposta não vazia",
			}},
			expect: func(err error) {
				s.Error(err)
				s.Contains(err.Error(), "responseProperty")
			},
		},
		{
			name: "deve rejeitar caso sem responseDescribe",
			args: args{c: Case{
				Name:             "sem response describe",
				Category:         CategoryQuery,
				Input:            "quanto gastei?",
				ExpectedTool:     "query_month",
				ResponseProperty: nonEmptyResponse,
			}},
			expect: func(err error) {
				s.Error(err)
				s.Contains(err.Error(), "responseDescribe")
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			err := scenario.args.c.Validate()
			scenario.expect(err)
		})
	}
}

func (s *CaseSchemaSuite) TestCategoryIsValid() {
	for _, c := range AllCategories() {
		s.True(c.IsValid(), "categoria %q deveria ser válida", c)
	}
	s.False(Category("nao_existe").IsValid())
}
