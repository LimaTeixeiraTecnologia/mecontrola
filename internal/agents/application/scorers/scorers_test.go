package scorers

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
	llmmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/scorer"
)

type ScorersSuite struct {
	suite.Suite
	ctx          context.Context
	providerMock *llmmocks.Provider
}

func TestScorersSuite(t *testing.T) {
	suite.Run(t, new(ScorersSuite))
}

func (s *ScorersSuite) SetupTest() {
	s.ctx = context.Background()
	s.providerMock = llmmocks.NewProvider(s.T())
}

func (s *ScorersSuite) TestToolCallAccuracyScorer_HitAll() {
	type args struct {
		sample scorer.RunSample
	}
	type dependencies struct{}

	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(result scorer.ScoreResult, err error)
	}{
		{
			name: "deve retornar score 1.0 quando tool get-weather é chamada",
			args: args{sample: scorer.RunSample{
				ToolCalls: []scorer.ToolCallRecord{{ID: "1", Name: "get-weather"}},
			}},
			dependencies: dependencies{},
			expect: func(result scorer.ScoreResult, err error) {
				s.NoError(err)
				s.InDelta(1.0, result.Score, 0.001)
			},
		},
		{
			name:         "deve retornar score 0.0 quando tool não é chamada",
			args:         args{sample: scorer.RunSample{ToolCalls: nil}},
			dependencies: dependencies{},
			expect: func(result scorer.ScoreResult, err error) {
				s.NoError(err)
				s.InDelta(0.0, result.Score, 0.001)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			sc := NewToolCallAccuracyScorer()
			result, err := sc.Score(s.ctx, scenario.args.sample)
			scenario.expect(result, err)
		})
	}
}

func (s *ScorersSuite) TestCompletenessScorer_AllFieldsPresent() {
	type args struct {
		sample scorer.RunSample
	}

	validOutput := map[string]any{
		"temperature": 25.0,
		"feelsLike":   23.0,
		"humidity":    60.0,
		"windSpeed":   10.0,
		"windGust":    15.0,
		"conditions":  "Clear sky",
		"location":    "London",
	}
	validJSON, _ := json.Marshal(validOutput)

	scenarios := []struct {
		name   string
		args   args
		expect func(result scorer.ScoreResult, err error)
	}{
		{
			name: "deve retornar score 1.0 quando todos os campos estão presentes",
			args: args{sample: scorer.RunSample{Output: string(validJSON)}},
			expect: func(result scorer.ScoreResult, err error) {
				s.NoError(err)
				s.InDelta(1.0, result.Score, 0.001)
			},
		},
		{
			name: "deve retornar score parcial quando campos estão faltando",
			args: args{sample: scorer.RunSample{Output: `{"temperature":25.0}`}},
			expect: func(result scorer.ScoreResult, err error) {
				s.NoError(err)
				s.Less(result.Score, 1.0)
			},
		},
		{
			name: "deve retornar score 0.0 quando output não é JSON",
			args: args{sample: scorer.RunSample{Output: "not json"}},
			expect: func(result scorer.ScoreResult, err error) {
				s.NoError(err)
				s.InDelta(0.0, result.Score, 0.001)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			sc := NewCompletenessScorer()
			result, err := sc.Score(s.ctx, scenario.args.sample)
			scenario.expect(result, err)
		})
	}
}

func (s *ScorersSuite) TestTranslationScorer_ValidContract() {
	type args struct {
		sample scorer.RunSample
	}
	type dependencies struct {
		providerMock *llmmocks.Provider
	}

	validJudgeResponse := `{"score":0.9,"reason":"translation is accurate"}`

	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(result scorer.ScoreResult, err error)
	}{
		{
			name: "deve retornar score quando judge responde com contrato válido",
			args: args{sample: scorer.RunSample{
				Input:  "clima em São Paulo?",
				Output: "Weather in São Paulo: 25°C",
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
			args: args{sample: scorer.RunSample{Input: "clima", Output: "weather"}},
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
		{
			name: "deve retornar erro quando contrato não é satisfeito (score fora de range)",
			args: args{sample: scorer.RunSample{Input: "clima", Output: "weather"}},
			dependencies: dependencies{
				providerMock: func() *llmmocks.Provider {
					s.providerMock.EXPECT().
						Complete(mock.Anything, mock.AnythingOfType("llm.Request")).
						Return(llm.Response{RawJSON: []byte(`{"score":2.0,"reason":"bad"}`)}, nil).
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
			sc := NewTranslationScorer(scenario.dependencies.providerMock)
			result, err := sc.Score(s.ctx, scenario.args.sample)
			scenario.expect(result, err)
		})
	}
}

func (s *ScorersSuite) TestBuildWeatherScorers_ReturnsThreeEntries() {
	entries := BuildWeatherScorers(s.providerMock)
	s.Len(entries, 3)
}
