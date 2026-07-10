package scorers

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/scorer"
)

type WritePersistenceAccuracySuite struct {
	suite.Suite
	ctx context.Context
}

func TestWritePersistenceAccuracySuite(t *testing.T) {
	suite.Run(t, new(WritePersistenceAccuracySuite))
}

func (s *WritePersistenceAccuracySuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *WritePersistenceAccuracySuite) TestScore() {
	type args struct {
		sample scorer.RunSample
	}

	scenarios := []struct {
		name   string
		args   args
		expect func(result scorer.ScoreResult, err error)
	}{
		{
			name: "routed conta como efeito legitimo e pontua 1",
			args: args{sample: scorer.RunSample{
				ToolCalls: []scorer.ToolCallRecord{{Name: "register_expense", Outcome: "routed"}},
			}},
			expect: func(result scorer.ScoreResult, err error) {
				s.NoError(err)
				s.InDelta(1.0, result.Score, 0.001)
			},
		},
		{
			name: "reconciled conta como efeito legitimo e pontua 1",
			args: args{sample: scorer.RunSample{
				ToolCalls: []scorer.ToolCallRecord{{Name: "register_income", Outcome: "reconciled"}},
			}},
			expect: func(result scorer.ScoreResult, err error) {
				s.NoError(err)
				s.InDelta(1.0, result.Score, 0.001)
			},
		},
		{
			name: "usecaseError reprova com 0",
			args: args{sample: scorer.RunSample{
				ToolCalls: []scorer.ToolCallRecord{{Name: "register_expense", Outcome: "usecaseError"}},
			}},
			expect: func(result scorer.ScoreResult, err error) {
				s.NoError(err)
				s.InDelta(0.0, result.Score, 0.001)
			},
		},
		{
			name: "missingResolver reprova com 0",
			args: args{sample: scorer.RunSample{
				ToolCalls: []scorer.ToolCallRecord{{Name: "adjust_allocation", Outcome: "missingResolver"}},
			}},
			expect: func(result scorer.ScoreResult, err error) {
				s.NoError(err)
				s.InDelta(0.0, result.Score, 0.001)
			},
		},
		{
			name: "truncated reprova com 0",
			args: args{sample: scorer.RunSample{
				ToolCalls: []scorer.ToolCallRecord{{Name: "delete_entry", Outcome: "truncated"}},
			}},
			expect: func(result scorer.ScoreResult, err error) {
				s.NoError(err)
				s.InDelta(0.0, result.Score, 0.001)
			},
		},
		{
			name: "vazio-com-marcador (write-tool sem outcome) reprova com 0",
			args: args{sample: scorer.RunSample{
				Output:    "Registrei sua despesa.",
				ToolCalls: []scorer.ToolCallRecord{{Name: "register_expense", Outcome: ""}},
			}},
			expect: func(result scorer.ScoreResult, err error) {
				s.NoError(err)
				s.InDelta(0.0, result.Score, 0.001)
			},
		},
		{
			name: "clarify e neutro fica fora do denominador e pontua 1",
			args: args{sample: scorer.RunSample{
				ToolCalls: []scorer.ToolCallRecord{{Name: "register_expense", Outcome: "clarify"}},
			}},
			expect: func(result scorer.ScoreResult, err error) {
				s.NoError(err)
				s.InDelta(1.0, result.Score, 0.001)
			},
		},
		{
			name: "replay e neutro fica fora do denominador e pontua 1",
			args: args{sample: scorer.RunSample{
				ToolCalls: []scorer.ToolCallRecord{{Name: "register_expense", Outcome: "replay"}},
			}},
			expect: func(result scorer.ScoreResult, err error) {
				s.NoError(err)
				s.InDelta(1.0, result.Score, 0.001)
			},
		},
		{
			name: "misto: write legitima e write sem efeito reprova com 0 (falha-segura)",
			args: args{sample: scorer.RunSample{
				ToolCalls: []scorer.ToolCallRecord{
					{Name: "register_expense", Outcome: "routed"},
					{Name: "register_income", Outcome: "usecaseError"},
				},
			}},
			expect: func(result scorer.ScoreResult, err error) {
				s.NoError(err)
				s.InDelta(0.0, result.Score, 0.001)
			},
		},
		{
			name: "sem write-tool (apenas leitura) e neutro e pontua 1",
			args: args{sample: scorer.RunSample{
				ToolCalls: []scorer.ToolCallRecord{{Name: "query_month", Outcome: ""}},
			}},
			expect: func(result scorer.ScoreResult, err error) {
				s.NoError(err)
				s.InDelta(1.0, result.Score, 0.001)
			},
		},
		{
			name: "sample sem tool-calls e neutro e pontua 1",
			args: args{sample: scorer.RunSample{Output: "Como posso ajudar?"}},
			expect: func(result scorer.ScoreResult, err error) {
				s.NoError(err)
				s.InDelta(1.0, result.Score, 0.001)
			},
		},
		{
			name: "duas writes efetivadas pontuam 1",
			args: args{sample: scorer.RunSample{
				ToolCalls: []scorer.ToolCallRecord{
					{Name: "register_expense", Outcome: "routed"},
					{Name: "register_income", Outcome: "reconciled"},
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
			sc := NewWritePersistenceAccuracyScorer()
			result, err := sc.Score(s.ctx, scenario.args.sample)
			scenario.expect(result, err)
		})
	}
}

func (s *WritePersistenceAccuracySuite) TestKindAndID() {
	sc := NewWritePersistenceAccuracyScorer()
	s.Equal("write_persistence_accuracy", sc.ID())
	s.Equal(scorer.ScorerKindCodeBased, sc.Kind())
}
