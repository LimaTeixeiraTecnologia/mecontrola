package scorers

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
	llmmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/scorer"
)

type MecontrolaScorersSuite struct {
	suite.Suite
	ctx          context.Context
	providerMock *llmmocks.Provider
}

func TestMecontrolaScorersSuite(t *testing.T) {
	suite.Run(t, new(MecontrolaScorersSuite))
}

func (s *MecontrolaScorersSuite) SetupTest() {
	s.ctx = context.Background()
	s.providerMock = llmmocks.NewProvider(s.T())
}

func (s *MecontrolaScorersSuite) TestFinancialToolCallAccuracyScorer() {
	type args struct {
		sample scorer.RunSample
	}

	scenarios := []struct {
		name   string
		args   args
		expect func(result scorer.ScoreResult, err error)
	}{
		{
			name: "deve retornar score 1.0 quando ferramenta financeira e invocada",
			args: args{sample: scorer.RunSample{
				ToolCalls: []scorer.ToolCallRecord{{ID: "1", Name: "register_expense"}},
			}},
			expect: func(result scorer.ScoreResult, err error) {
				s.NoError(err)
				s.InDelta(1.0, result.Score, 0.001)
			},
		},
		{
			name: "deve retornar score 1.0 com qualquer ferramenta financeira valida",
			args: args{sample: scorer.RunSample{
				ToolCalls: []scorer.ToolCallRecord{{ID: "1", Name: "query_month"}},
			}},
			expect: func(result scorer.ScoreResult, err error) {
				s.NoError(err)
				s.InDelta(1.0, result.Score, 0.001)
			},
		},
		{
			name: "deve retornar score 0.0 quando nenhuma ferramenta e invocada",
			args: args{sample: scorer.RunSample{ToolCalls: nil}},
			expect: func(result scorer.ScoreResult, err error) {
				s.NoError(err)
				s.InDelta(0.0, result.Score, 0.001)
			},
		},
		{
			name: "deve retornar score 0.0 quando ferramenta nao e financeira",
			args: args{sample: scorer.RunSample{
				ToolCalls: []scorer.ToolCallRecord{{ID: "1", Name: "send_email"}},
			}},
			expect: func(result scorer.ScoreResult, err error) {
				s.NoError(err)
				s.InDelta(0.0, result.Score, 0.001)
			},
		},
		{
			name: "deve retornar score 1.0 com multiplas ferramentas financeiras",
			args: args{sample: scorer.RunSample{
				ToolCalls: []scorer.ToolCallRecord{
					{ID: "1", Name: "register_expense"},
					{ID: "2", Name: "register_income"},
				},
			}},
			expect: func(result scorer.ScoreResult, err error) {
				s.NoError(err)
				s.InDelta(1.0, result.Score, 0.001)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			sc := NewFinancialToolCallAccuracyScorer()
			result, err := sc.Score(s.ctx, scenario.args.sample)
			scenario.expect(result, err)
		})
	}
}

func (s *MecontrolaScorersSuite) TestFinancialCompletenessScorer() {
	type args struct {
		sample scorer.RunSample
	}

	scenarios := []struct {
		name   string
		args   args
		expect func(result scorer.ScoreResult, err error)
	}{
		{
			name: "deve retornar score alto quando output contem palavras-chave financeiras",
			args: args{sample: scorer.RunSample{
				Output: "✅ Registrei sua despesa de R$ 50,00 na categoria Alimentação. Lançamento confirmado para o mês de junho.",
			}},
			expect: func(result scorer.ScoreResult, err error) {
				s.NoError(err)
				s.Greater(result.Score, 0.3)
			},
		},
		{
			name: "deve retornar score 0.0 quando output esta vazio",
			args: args{sample: scorer.RunSample{Output: ""}},
			expect: func(result scorer.ScoreResult, err error) {
				s.NoError(err)
				s.InDelta(0.0, result.Score, 0.001)
			},
		},
		{
			name: "deve retornar score baixo quando output e texto sem contexto financeiro",
			args: args{sample: scorer.RunSample{Output: "Olá! Como posso te ajudar hoje?"}},
			expect: func(result scorer.ScoreResult, err error) {
				s.NoError(err)
				s.Less(result.Score, 0.3)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			sc := NewFinancialCompletenessScorer()
			result, err := sc.Score(s.ctx, scenario.args.sample)
			scenario.expect(result, err)
		})
	}
}

func (s *MecontrolaScorersSuite) TestCategorizationScorer() {
	type args struct {
		sample scorer.RunSample
	}
	type dependencies struct {
		providerMock *llmmocks.Provider
	}

	validJudgeResponse := `{"score":0.9,"reason":"categoria plausivel para o contexto"}`

	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(result scorer.ScoreResult, err error)
	}{
		{
			name: "deve retornar score quando judge responde com contrato valido",
			args: args{sample: scorer.RunSample{
				Input:  "gastei no mercado",
				Output: "✅ Registrei R$ 100,00 na categoria Alimentação.",
			}},
			dependencies: dependencies{
				providerMock: func() *llmmocks.Provider {
					s.providerMock.EXPECT().
						Complete(mock.Anything, mock.AnythingOfType("llm.Request")).
						Return(llm.Response{RawJSON: []byte(validJudgeResponse)}, nil).
						Once()
					return s.providerMock
				}(),
			},
			expect: func(result scorer.ScoreResult, err error) {
				s.NoError(err)
				s.InDelta(0.9, result.Score, 0.001)
			},
		},
		{
			name: "deve retornar erro quando provider falha",
			args: args{sample: scorer.RunSample{Input: "mercado", Output: "Alimentação"}},
			dependencies: dependencies{
				providerMock: func() *llmmocks.Provider {
					s.providerMock.EXPECT().
						Complete(mock.Anything, mock.AnythingOfType("llm.Request")).
						Return(llm.Response{}, errors.New("provider error")).
						Once()
					return s.providerMock
				}(),
			},
			expect: func(result scorer.ScoreResult, err error) {
				s.Error(err)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			sc := NewCategorizationScorer(scenario.dependencies.providerMock)
			result, err := sc.Score(s.ctx, scenario.args.sample)
			scenario.expect(result, err)
		})
	}
}

func (s *MecontrolaScorersSuite) TestExpectedToolScorer() {
	type args struct {
		sample       scorer.RunSample
		expectedTool string
	}

	scenarios := []struct {
		name   string
		args   args
		expect func(result scorer.ScoreResult, err error)
	}{
		{
			name: "deve retornar score 1.0 quando tool esperada e invocada",
			args: args{
				expectedTool: "register_expense",
				sample: scorer.RunSample{
					ToolCalls: []scorer.ToolCallRecord{{ID: "1", Name: "register_expense"}},
				},
			},
			expect: func(result scorer.ScoreResult, err error) {
				s.NoError(err)
				s.InDelta(1.0, result.Score, 0.001)
			},
		},
		{
			name: "deve retornar score 0.0 quando tool diferente e invocada",
			args: args{
				expectedTool: "register_expense",
				sample: scorer.RunSample{
					ToolCalls: []scorer.ToolCallRecord{{ID: "1", Name: "query_month"}},
				},
			},
			expect: func(result scorer.ScoreResult, err error) {
				s.NoError(err)
				s.InDelta(0.0, result.Score, 0.001)
			},
		},
		{
			name: "deve retornar score 0.0 quando nenhuma tool e invocada",
			args: args{
				expectedTool: "list_cards",
				sample:       scorer.RunSample{ToolCalls: nil},
			},
			expect: func(result scorer.ScoreResult, err error) {
				s.NoError(err)
				s.InDelta(0.0, result.Score, 0.001)
			},
		},
		{
			name: "deve retornar score 1.0 quando tool esperada e uma dentre multiplas",
			args: args{
				expectedTool: "query_plan",
				sample: scorer.RunSample{
					ToolCalls: []scorer.ToolCallRecord{
						{ID: "1", Name: "list_cards"},
						{ID: "2", Name: "query_plan"},
					},
				},
			},
			expect: func(result scorer.ScoreResult, err error) {
				s.NoError(err)
				s.InDelta(1.0, result.Score, 0.001)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			sc := NewExpectedToolScorer(scenario.args.expectedTool)
			result, err := sc.Score(s.ctx, scenario.args.sample)
			scenario.expect(result, err)
		})
	}
}

func (s *MecontrolaScorersSuite) TestExpectedToolScorer_Kind() {
	sc := NewExpectedToolScorer("get_card")
	s.Equal(scorer.ScorerKindCodeBased, sc.Kind())
	s.Equal("expected-tool:get_card", sc.ID())
}

func (s *MecontrolaScorersSuite) TestFinancialToolCallAccuracyScorer_Covers22Tools() {
	s.Len(mecontrolaFinancialTools, 22)
	for _, toolName := range mecontrolaFinancialTools {
		s.Run("deve reconhecer "+toolName, func() {
			sc := NewFinancialToolCallAccuracyScorer()
			result, err := sc.Score(s.ctx, scorer.RunSample{
				ToolCalls: []scorer.ToolCallRecord{{ID: "1", Name: toolName}},
			})
			s.NoError(err)
			s.InDelta(1.0, result.Score, 0.001)
		})
	}
}

func (s *MecontrolaScorersSuite) TestBuildMeControlaScorers_ReturnsAllEntries() {
	type dependencies struct{}

	scenarios := []struct {
		name         string
		dependencies dependencies
		expect       func(entries []scorer.ScorerEntry)
	}{
		{
			name:         "deve retornar os 3 scorers atuais mais os intrinsecos comportamentais e os de tom",
			dependencies: dependencies{},
			expect: func(entries []scorer.ScorerEntry) {
				s.Len(entries, 15)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			_ = scenario.dependencies
			entries := BuildMeControlaScorers(s.providerMock)
			scenario.expect(entries)
		})
	}
}
