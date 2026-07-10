package postdeploy

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type GateSuite struct {
	suite.Suite
}

func TestGateSuite(t *testing.T) {
	suite.Run(t, new(GateSuite))
}

func (s *GateSuite) TestMeetsMinimumSample() {
	type args struct {
		aggregate RunAggregate
	}

	scenarios := []struct {
		name   string
		args   args
		expect func(ok bool)
	}{
		{
			name: "deve aprovar amostra quando N>=100 runs",
			args: args{aggregate: RunAggregate{TotalRuns: 100, WindowStart: time.Now(), WindowEnd: time.Now()}},
			expect: func(ok bool) {
				s.True(ok)
			},
		},
		{
			name: "deve aprovar amostra quando janela>=14 dias mesmo com poucos runs",
			args: args{aggregate: RunAggregate{TotalRuns: 10, WindowStart: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), WindowEnd: time.Date(2026, 1, 16, 0, 0, 0, 0, time.UTC)}},
			expect: func(ok bool) {
				s.True(ok)
			},
		},
		{
			name: "deve reprovar amostra quando N<100 e janela<14 dias",
			args: args{aggregate: RunAggregate{TotalRuns: 23, WindowStart: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), WindowEnd: time.Date(2026, 1, 8, 0, 0, 0, 0, time.UTC)}},
			expect: func(ok bool) {
				s.False(ok)
			},
		},
		{
			name: "deve reprovar janela invertida (fim antes do início) como zero dias",
			args: args{aggregate: RunAggregate{TotalRuns: 5, WindowStart: time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC), WindowEnd: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}},
			expect: func(ok bool) {
				s.False(ok)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			ok := scenario.args.aggregate.MeetsMinimumSample()
			scenario.expect(ok)
		})
	}
}

func (s *GateSuite) TestFailureRate() {
	type args struct {
		aggregate RunAggregate
	}

	scenarios := []struct {
		name   string
		args   args
		expect func(rate float64)
	}{
		{
			name: "deve calcular taxa de falha igual a baseline",
			args: args{aggregate: RunAggregate{TotalRuns: 23, FailedRuns: 4}},
			expect: func(rate float64) {
				s.InDelta(BaselineFailureRate, rate, 0.0001)
			},
		},
		{
			name: "deve retornar zero quando não há runs",
			args: args{aggregate: RunAggregate{TotalRuns: 0, FailedRuns: 0}},
			expect: func(rate float64) {
				s.Equal(0.0, rate)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			rate := scenario.args.aggregate.FailureRate()
			scenario.expect(rate)
		})
	}
}

func (s *GateSuite) TestRedefinedToolCallAccuracy() {
	type args struct {
		aggregate RunAggregate
		hits      int
	}

	scenarios := []struct {
		name   string
		args   args
		expect func(ratio float64)
	}{
		{
			name: "deve excluir clarify/replay do denominador (redefinição RF-42)",
			args: args{aggregate: RunAggregate{TotalRuns: 23, ExpectedToolRuns: 15}, hits: 12},
			expect: func(ratio float64) {
				s.InDelta(0.8, ratio, 0.0001)
			},
		},
		{
			name: "deve retornar zero quando nenhum run esperava tool",
			args: args{aggregate: RunAggregate{TotalRuns: 5, ExpectedToolRuns: 0}, hits: 0},
			expect: func(ratio float64) {
				s.Equal(0.0, ratio)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			ratio := scenario.args.aggregate.RedefinedToolCallAccuracy(scenario.args.hits)
			scenario.expect(ratio)
		})
	}
}

func (s *GateSuite) TestEvaluateGate() {
	type args struct {
		runs         RunAggregate
		toolCallHits int
		scorers      map[string]ScorerAggregate
		ops          OperationalCounters
	}

	sufficientSample := RunAggregate{
		AgentID:          "mecontrola-agent",
		TotalRuns:        120,
		SucceededRuns:    115,
		FailedRuns:       4,
		ExpectedToolRuns: 100,
		TruncatedRuns:    0,
	}

	scenarios := []struct {
		name   string
		args   args
		expect func(v GateVerdict)
	}{
		{
			name: "deve promover quando amostra suficiente, sem regressão e métricas acima da margem",
			args: args{
				runs:         sufficientSample,
				toolCallHits: 90,
				scorers: map[string]ScorerAggregate{
					ScorerIDCompleteness:   {ScorerID: ScorerIDCompleteness, MeanScore: RequiredCompleteness + 0.01},
					ScorerIDCategorization: {ScorerID: ScorerIDCategorization, MeanScore: RequiredCategorization + 0.01},
				},
				ops: OperationalCounters{},
			},
			expect: func(v GateVerdict) {
				s.True(v.Promote)
				s.True(v.SampleSufficient)
				s.True(v.FailureRatePassed)
				s.True(v.NoRegressionOperational)
				s.Empty(v.Reasons)
			},
		},
		{
			name: "não deve promover quando amostra insuficiente mesmo com métricas boas",
			args: args{
				runs:         RunAggregate{AgentID: "mecontrola-agent", TotalRuns: 23, FailedRuns: 4, ExpectedToolRuns: 15},
				toolCallHits: 15,
				scorers: map[string]ScorerAggregate{
					ScorerIDCompleteness:   {ScorerID: ScorerIDCompleteness, MeanScore: RequiredCompleteness + 0.1},
					ScorerIDCategorization: {ScorerID: ScorerIDCategorization, MeanScore: RequiredCategorization + 0.1},
				},
				ops: OperationalCounters{},
			},
			expect: func(v GateVerdict) {
				s.False(v.Promote)
				s.False(v.SampleSufficient)
				s.NotEmpty(v.Reasons)
			},
		},
		{
			name: "não deve promover quando taxa de falha excede a baseline",
			args: args{
				runs:         RunAggregate{AgentID: "mecontrola-agent", TotalRuns: 120, FailedRuns: 40, ExpectedToolRuns: 100},
				toolCallHits: 90,
				scorers: map[string]ScorerAggregate{
					ScorerIDCompleteness:   {ScorerID: ScorerIDCompleteness, MeanScore: RequiredCompleteness + 0.1},
					ScorerIDCategorization: {ScorerID: ScorerIDCategorization, MeanScore: RequiredCategorization + 0.1},
				},
				ops: OperationalCounters{},
			},
			expect: func(v GateVerdict) {
				s.False(v.Promote)
				s.False(v.FailureRatePassed)
			},
		},
		{
			name: "não deve promover quando tool-call-accuracy redefinida abaixo da margem",
			args: args{
				runs:         sufficientSample,
				toolCallHits: 10,
				scorers: map[string]ScorerAggregate{
					ScorerIDCompleteness:   {ScorerID: ScorerIDCompleteness, MeanScore: RequiredCompleteness + 0.1},
					ScorerIDCategorization: {ScorerID: ScorerIDCategorization, MeanScore: RequiredCategorization + 0.1},
				},
				ops: OperationalCounters{},
			},
			expect: func(v GateVerdict) {
				s.False(v.Promote)
				s.NotEmpty(v.Reasons)
			},
		},
		{
			name: "não deve promover quando houve truncamento no período",
			args: args{
				runs: RunAggregate{
					AgentID: "mecontrola-agent", TotalRuns: 120, FailedRuns: 4,
					ExpectedToolRuns: 100, TruncatedRuns: 3,
				},
				toolCallHits: 90,
				scorers: map[string]ScorerAggregate{
					ScorerIDCompleteness:   {ScorerID: ScorerIDCompleteness, MeanScore: RequiredCompleteness + 0.1},
					ScorerIDCategorization: {ScorerID: ScorerIDCategorization, MeanScore: RequiredCategorization + 0.1},
				},
				ops: OperationalCounters{},
			},
			expect: func(v GateVerdict) {
				s.False(v.Promote)
				s.False(v.NoRegressionOperational)
			},
		},
		{
			name: "não deve promover quando há erros de RunStore.Update ou MessageStore.Append",
			args: args{
				runs:         sufficientSample,
				toolCallHits: 90,
				scorers: map[string]ScorerAggregate{
					ScorerIDCompleteness:   {ScorerID: ScorerIDCompleteness, MeanScore: RequiredCompleteness + 0.1},
					ScorerIDCategorization: {ScorerID: ScorerIDCategorization, MeanScore: RequiredCategorization + 0.1},
				},
				ops: OperationalCounters{RunUpdateErrors: 1},
			},
			expect: func(v GateVerdict) {
				s.False(v.Promote)
				s.False(v.NoRegressionOperational)
			},
		},
		{
			name: "não deve promover quando há violação de escrita duplicada",
			args: args{
				runs:         sufficientSample,
				toolCallHits: 90,
				scorers: map[string]ScorerAggregate{
					ScorerIDCompleteness:   {ScorerID: ScorerIDCompleteness, MeanScore: RequiredCompleteness + 0.1},
					ScorerIDCategorization: {ScorerID: ScorerIDCategorization, MeanScore: RequiredCategorization + 0.1},
				},
				ops: OperationalCounters{DuplicateWriteViolations: 1},
			},
			expect: func(v GateVerdict) {
				s.False(v.Promote)
				s.False(v.NoRegressionOperational)
			},
		},
		{
			name: "não deve promover quando completeness está ausente na amostra (evidência incompleta)",
			args: args{
				runs:         sufficientSample,
				toolCallHits: 90,
				scorers: map[string]ScorerAggregate{
					ScorerIDCategorization: {ScorerID: ScorerIDCategorization, MeanScore: RequiredCategorization + 0.1},
				},
				ops: OperationalCounters{},
			},
			expect: func(v GateVerdict) {
				s.False(v.Promote)
				s.NotEmpty(v.Reasons)
			},
		},
		{
			name: "não deve promover quando categorization está ausente na amostra (evidência incompleta)",
			args: args{
				runs:         sufficientSample,
				toolCallHits: 90,
				scorers: map[string]ScorerAggregate{
					ScorerIDCompleteness: {ScorerID: ScorerIDCompleteness, MeanScore: RequiredCompleteness + 0.1},
				},
				ops: OperationalCounters{},
			},
			expect: func(v GateVerdict) {
				s.False(v.Promote)
				s.NotEmpty(v.Reasons)
			},
		},
		{
			name: "não deve promover quando nenhum scorer de continuidade está presente (mapa vazio)",
			args: args{
				runs:         sufficientSample,
				toolCallHits: 90,
				scorers:      map[string]ScorerAggregate{},
				ops:          OperationalCounters{},
			},
			expect: func(v GateVerdict) {
				s.False(v.Promote)
				s.Len(v.Reasons, 2)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			verdict := EvaluateGate(scenario.args.runs, scenario.args.toolCallHits, scenario.args.scorers, scenario.args.ops)
			scenario.expect(verdict)
		})
	}
}
