package agents

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/llm"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/scorer"
)

type fakeScorerRunner struct {
	observed []observedCall
}

type observedCall struct {
	runID  uuid.UUID
	sample scorer.RunSample
}

func (f *fakeScorerRunner) Observe(_ context.Context, runID uuid.UUID, s scorer.RunSample) {
	f.observed = append(f.observed, observedCall{runID: runID, sample: s})
}

func (f *fakeScorerRunner) Shutdown(_ context.Context) {}

type ScoringHooksSuite struct {
	suite.Suite
	ctx    context.Context
	runner *fakeScorerRunner
	hooks  *ScoringHooks
}

func TestScoringHooksSuite(t *testing.T) {
	suite.Run(t, new(ScoringHooksSuite))
}

func (s *ScoringHooksSuite) SetupTest() {
	s.ctx = context.Background()
	s.runner = &fakeScorerRunner{}
	s.hooks = NewScoringHooks(s.runner)
}

func (s *ScoringHooksSuite) TestObservesRunWithToolCalls() {
	runID := uuid.New()
	ctx := agent.WithRunID(s.ctx, runID)

	req := agent.Request{Messages: []llm.Message{
		{Role: "system", Content: "instructions"},
		{Role: "user", Content: "gastei R$ 50 no mercado"},
	}}

	ctx = s.hooks.BeforeExecute(ctx, "mecontrola-agent", req)
	s.hooks.AfterTool(ctx, "mecontrola-agent", "register_expense", []byte(`{}`), nil)
	s.hooks.AfterExecute(ctx, "mecontrola-agent", agent.Result{Content: "Registrei R$ 50,00."}, nil)

	s.Require().Len(s.runner.observed, 1)
	call := s.runner.observed[0]
	s.Equal(runID, call.runID)
	s.Equal("gastei R$ 50 no mercado", call.sample.Input)
	s.Equal("Registrei R$ 50,00.", call.sample.Output)
	s.Require().Len(call.sample.ToolCalls, 1)
	s.Equal("register_expense", call.sample.ToolCalls[0].Name)
}

func (s *ScoringHooksSuite) TestSkipsOnExecutionError() {
	ctx := agent.WithRunID(s.ctx, uuid.New())
	ctx = s.hooks.BeforeExecute(ctx, "mecontrola-agent", agent.Request{})
	s.hooks.AfterExecute(ctx, "mecontrola-agent", agent.Result{}, errors.New("boom"))
	s.Empty(s.runner.observed)
}

func (s *ScoringHooksSuite) TestSkipsWhenRunIDMissing() {
	ctx := s.hooks.BeforeExecute(s.ctx, "mecontrola-agent", agent.Request{
		Messages: []llm.Message{{Role: "user", Content: "hi"}},
	})
	s.hooks.AfterExecute(ctx, "mecontrola-agent", agent.Result{Content: "hello"}, nil)
	s.Empty(s.runner.observed)
}

func (s *ScoringHooksSuite) TestSkipsToolCallOnError() {
	ctx := agent.WithRunID(s.ctx, uuid.New())
	ctx = s.hooks.BeforeExecute(ctx, "mecontrola-agent", agent.Request{
		Messages: []llm.Message{{Role: "user", Content: "hi"}},
	})
	s.hooks.AfterTool(ctx, "mecontrola-agent", "register_expense", nil, errors.New("tool failed"))
	s.hooks.AfterExecute(ctx, "mecontrola-agent", agent.Result{Content: "ok"}, nil)

	s.Require().Len(s.runner.observed, 1)
	s.Empty(s.runner.observed[0].sample.ToolCalls)
}
