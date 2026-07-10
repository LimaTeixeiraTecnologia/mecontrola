package agents

import (
	"context"
	"errors"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/scorer"
	scorermocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/scorer/mocks"
)

type ScoringHooksSuite struct {
	suite.Suite
	ctx    context.Context
	obs    observability.Observability
	runner *scorermocks.ScorerRunner
}

func TestScoringHooksSuite(t *testing.T) {
	suite.Run(t, new(ScoringHooksSuite))
}

func (s *ScoringHooksSuite) SetupTest() {
	s.ctx = context.Background()
	s.obs = fake.NewProvider()
	s.runner = scorermocks.NewScorerRunner(s.T())
}

func (s *ScoringHooksSuite) TestObserve() {
	type args struct {
		withRunID  bool
		userMsg    string
		toolCalled bool
		toolErr    error
		output     string
		execErr    error
	}
	type dependencies struct {
		runner *scorermocks.ScorerRunner
	}

	runID := uuid.New()

	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
	}{
		{
			name: "deve observar run com tool calls",
			args: args{withRunID: true, userMsg: "gastei R$ 50 no mercado", toolCalled: true, output: "Registrei R$ 50,00."},
			dependencies: dependencies{
				runner: func() *scorermocks.ScorerRunner {
					s.runner.EXPECT().
						Observe(mock.Anything, mock.Anything, mock.Anything).
						Run(func(_ context.Context, gotRunID uuid.UUID, sample scorer.RunSample) {
							s.Equal(runID, gotRunID)
							s.Equal("gastei R$ 50 no mercado", sample.Input)
							s.Equal("Registrei R$ 50,00.", sample.Output)
							s.Require().Len(sample.ToolCalls, 1)
							s.Equal("register_expense", sample.ToolCalls[0].Name)
						}).
						Return().
						Once()
					return s.runner
				}(),
			},
		},
		{
			name:         "deve pular quando execucao retorna erro",
			args:         args{withRunID: true, execErr: errors.New("boom")},
			dependencies: dependencies{runner: s.runner},
		},
		{
			name:         "deve pular quando run_id esta ausente",
			args:         args{userMsg: "hi", output: "hello"},
			dependencies: dependencies{runner: s.runner},
		},
		{
			name: "deve pular tool call em erro mas observar run",
			args: args{withRunID: true, userMsg: "hi", toolCalled: true, toolErr: errors.New("tool failed"), output: "ok"},
			dependencies: dependencies{
				runner: func() *scorermocks.ScorerRunner {
					s.runner.EXPECT().
						Observe(mock.Anything, mock.Anything, mock.Anything).
						Run(func(_ context.Context, gotRunID uuid.UUID, sample scorer.RunSample) {
							s.Equal(runID, gotRunID)
							s.Empty(sample.ToolCalls)
						}).
						Return().
						Once()
					return s.runner
				}(),
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			hooks := NewScoringHooks(scenario.dependencies.runner, s.obs)

			ctx := s.ctx
			if scenario.args.withRunID {
				ctx = agent.WithRunID(ctx, runID)
			}

			ctx = hooks.BeforeExecute(ctx, "mecontrola-agent", agent.Request{Messages: []llm.Message{
				{Role: "system", Content: "instructions"},
				{Role: "user", Content: scenario.args.userMsg},
			}})

			if scenario.args.toolCalled {
				hooks.AfterTool(ctx, "mecontrola-agent", "register_expense", []byte(`{}`), []byte(`{}`), scenario.args.toolErr)
			}

			hooks.AfterExecute(ctx, "mecontrola-agent", agent.Result{Content: scenario.args.output}, scenario.args.execErr)
		})
	}
}

func (s *ScoringHooksSuite) TestAfterTool_CapturesArgs() {
	type args struct {
		argsJSON []byte
	}
	type dependencies struct {
		runner *scorermocks.ScorerRunner
	}

	runID := uuid.New()

	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(sample scorer.RunSample)
	}{
		{
			name: "deve popular Args a partir do argsJSON valido",
			args: args{argsJSON: []byte(`{"amountCents":5000,"description":"mercado"}`)},
			dependencies: dependencies{
				runner: func() *scorermocks.ScorerRunner {
					s.runner.EXPECT().
						Observe(mock.Anything, mock.Anything, mock.Anything).
						Run(func(_ context.Context, _ uuid.UUID, sample scorer.RunSample) {
							s.Require().Len(sample.ToolCalls, 1)
							s.Equal(float64(5000), sample.ToolCalls[0].Args["amountCents"])
							s.Equal("mercado", sample.ToolCalls[0].Args["description"])
						}).
						Return().
						Once()
					return s.runner
				}(),
			},
			expect: func(sample scorer.RunSample) {},
		},
		{
			name: "deve manter Args nulo quando argsJSON e invalido",
			args: args{argsJSON: []byte(`not-json`)},
			dependencies: dependencies{
				runner: func() *scorermocks.ScorerRunner {
					s.runner.EXPECT().
						Observe(mock.Anything, mock.Anything, mock.Anything).
						Run(func(_ context.Context, _ uuid.UUID, sample scorer.RunSample) {
							s.Require().Len(sample.ToolCalls, 1)
							s.Nil(sample.ToolCalls[0].Args)
						}).
						Return().
						Once()
					return s.runner
				}(),
			},
			expect: func(sample scorer.RunSample) {},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			hooks := NewScoringHooks(scenario.dependencies.runner, s.obs)

			ctx := agent.WithRunID(s.ctx, runID)
			ctx = hooks.BeforeExecute(ctx, "mecontrola-agent", agent.Request{Messages: []llm.Message{
				{Role: "user", Content: "gastei R$ 50 no mercado"},
			}})

			hooks.AfterTool(ctx, "mecontrola-agent", "register_expense", scenario.args.argsJSON, []byte(`{}`), nil)
			hooks.AfterExecute(ctx, "mecontrola-agent", agent.Result{Content: "ok"}, nil)
		})
	}
}

func (s *ScoringHooksSuite) TestAfterExecute_RF28_RecordsSkipWhenRunIDMissing() {
	hooks := NewScoringHooks(s.runner, s.obs)

	ctx := hooks.BeforeExecute(s.ctx, "mecontrola-agent", agent.Request{Messages: []llm.Message{
		{Role: "user", Content: "hi"},
	}})

	s.NotPanics(func() {
		hooks.AfterExecute(ctx, "mecontrola-agent", agent.Result{Content: "hello"}, nil)
	})
}

func (s *ScoringHooksSuite) TestAfterExecute_RF28_RecordsSkipWhenRunnerNil() {
	hooks := NewScoringHooks(nil, s.obs)

	ctx := agent.WithRunID(s.ctx, uuid.New())
	ctx = hooks.BeforeExecute(ctx, "mecontrola-agent", agent.Request{Messages: []llm.Message{
		{Role: "user", Content: "hi"},
	}})

	s.NotPanics(func() {
		hooks.AfterExecute(ctx, "mecontrola-agent", agent.Result{Content: "hello"}, nil)
	})
}
