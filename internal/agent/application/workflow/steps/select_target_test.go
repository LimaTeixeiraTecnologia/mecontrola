package steps

import (
	"context"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/tools"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/confirmation"
	platform "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

type SelectTargetSuite struct {
	suite.Suite
	ctx context.Context
}

func TestSelectTargetSuite(t *testing.T) {
	suite.Run(t, new(SelectTargetSuite))
}

func (s *SelectTargetSuite) SetupTest() {
	s.ctx = context.Background()
}

func byRefState() confirmation.ConfirmState {
	return confirmation.ConfirmState{
		OperationKind: confirmation.OperationDeleteByRef,
		UserID:        "550e8400-e29b-41d4-a716-446655440000",
		Channel:       "whatsapp",
		SearchQuery:   "mercado",
	}
}

func candidates(n int) []confirmation.TargetCandidate {
	out := make([]confirmation.TargetCandidate, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, confirmation.TargetCandidate{
			TxID:        "tx-" + string(rune('a'+i)),
			Version:     int64(i + 1),
			Description: "Mercado",
			AmountCents: int64((i + 1) * 1000),
			OccurredAt:  "24/06",
		})
	}
	return out
}

func (s *SelectTargetSuite) TestNonByRefOperation_NoOp() {
	step := NewSelectTarget()
	state := byRefState()
	state.OperationKind = confirmation.OperationDeleteLast

	out, err := step.Execute(s.ctx, state)

	s.NoError(err)
	s.Equal(platform.StepStatusCompleted, out.Status)
	s.False(out.State.ShortCircuit)
	s.Empty(out.State.TargetTransactionID)
}

func (s *SelectTargetSuite) TestZeroCandidates_ShortCircuit() {
	step := NewSelectTarget()
	state := byRefState()
	state.Candidates = nil

	out, err := step.Execute(s.ctx, state)

	s.NoError(err)
	s.Equal(platform.StepStatusCompleted, out.Status)
	s.True(out.State.ShortCircuit)
	s.Equal(int(tools.OutcomeRouted), out.State.Outcome)
	s.Contains(out.State.Reply, "mercado")
}

func (s *SelectTargetSuite) TestSingleCandidate_AutoSelect() {
	step := NewSelectTarget()
	state := byRefState()
	state.Candidates = candidates(1)

	out, err := step.Execute(s.ctx, state)

	s.NoError(err)
	s.Equal(platform.StepStatusCompleted, out.Status)
	s.False(out.State.ShortCircuit)
	s.Equal("tx-a", out.State.TargetTransactionID)
	s.Equal(int64(1), out.State.TargetTransactionVersion)
	s.Equal(int64(1000), out.State.TargetAmountCents)
	s.Equal(confirmation.AwaitingNone, out.State.AwaitingApproval)
}

func (s *SelectTargetSuite) TestMultipleCandidates_SuspendSelect() {
	step := NewSelectTarget()
	state := byRefState()
	state.Candidates = candidates(3)

	out, err := step.Execute(s.ctx, state)

	s.NoError(err)
	s.Equal(platform.StepStatusSuspended, out.Status)
	s.Equal(confirmation.AwaitingSelect, out.State.AwaitingApproval)
	s.Require().NotNil(out.Suspend)
	s.Contains(out.Suspend.Prompt, "1) R$ 10,00 — Mercado (24/06)")
	s.Contains(out.Suspend.Prompt, "3) R$ 30,00 — Mercado (24/06)")
	s.Contains(out.Suspend.Prompt, "Responda com o número.")
}

func (s *SelectTargetSuite) TestResume_ValidIndex() {
	step := NewSelectTarget()
	state := byRefState()
	state.Candidates = candidates(3)
	state.AwaitingApproval = confirmation.AwaitingSelect
	state.ResumeText = "2"

	out, err := step.Execute(s.ctx, state)

	s.NoError(err)
	s.Equal(platform.StepStatusCompleted, out.Status)
	s.Equal("tx-b", out.State.TargetTransactionID)
	s.Equal(int64(2), out.State.TargetTransactionVersion)
	s.Equal(int64(2000), out.State.TargetAmountCents)
	s.Equal(confirmation.AwaitingNone, out.State.AwaitingApproval)
	s.False(out.State.ShortCircuit)
}

func (s *SelectTargetSuite) TestResume_InvalidIndex_RepromptOnce() {
	step := NewSelectTarget()
	state := byRefState()
	state.Candidates = candidates(3)
	state.AwaitingApproval = confirmation.AwaitingSelect
	state.ResumeText = "9"

	out, err := step.Execute(s.ctx, state)

	s.NoError(err)
	s.Equal(platform.StepStatusSuspended, out.Status)
	s.Equal(1, out.State.SelectRepromptCount)
	s.Equal(confirmation.AwaitingSelect, out.State.AwaitingApproval)
}

func (s *SelectTargetSuite) TestResume_InvalidIndex_SecondTimeCancels() {
	step := NewSelectTarget()
	state := byRefState()
	state.Candidates = candidates(3)
	state.AwaitingApproval = confirmation.AwaitingSelect
	state.ResumeText = "abc"
	state.SelectRepromptCount = 1

	out, err := step.Execute(s.ctx, state)

	s.NoError(err)
	s.Equal(platform.StepStatusCompleted, out.Status)
	s.True(out.State.ShortCircuit)
	s.Equal(confirmation.AwaitingNone, out.State.AwaitingApproval)
	s.Equal(int(tools.OutcomeRouted), out.State.Outcome)
}

func (s *SelectTargetSuite) TestResume_ZeroIndexRejected() {
	step := NewSelectTarget()
	state := byRefState()
	state.Candidates = candidates(2)
	state.AwaitingApproval = confirmation.AwaitingSelect
	state.ResumeText = "0"

	out, err := step.Execute(s.ctx, state)

	s.NoError(err)
	s.Equal(platform.StepStatusSuspended, out.Status)
	s.Equal(1, out.State.SelectRepromptCount)
}

func (s *SelectTargetSuite) TestResume_IndexWithExtraWords() {
	step := NewSelectTarget()
	state := byRefState()
	state.Candidates = candidates(3)
	state.AwaitingApproval = confirmation.AwaitingSelect
	state.ResumeText = "1 por favor"

	out, err := step.Execute(s.ctx, state)

	s.NoError(err)
	s.Equal(platform.StepStatusCompleted, out.Status)
	s.Equal("tx-a", out.State.TargetTransactionID)
}

type SelectTargetMetricSuite struct {
	suite.Suite
	ctx      context.Context
	provider *fake.Provider
}

func TestSelectTargetMetricSuite(t *testing.T) {
	suite.Run(t, new(SelectTargetMetricSuite))
}

func (s *SelectTargetMetricSuite) SetupTest() {
	s.ctx = context.Background()
	s.provider = fake.NewProvider()
}

func (s *SelectTargetMetricSuite) assertOutcome(expected string) {
	metrics := s.provider.Metrics().(*fake.FakeMetrics)
	counter := metrics.GetCounter("agent_target_select_total")
	s.Require().NotNil(counter)
	values := counter.GetValues()
	s.Require().Len(values, 1)
	s.Equal(int64(1), values[0].Value)
	s.Require().Len(values[0].Fields, 1)
	s.Equal("outcome", values[0].Fields[0].Key)
	s.Equal(expected, values[0].Fields[0].StringValue())
}

func (s *SelectTargetMetricSuite) TestNone() {
	step := NewSelectTargetWithObservability(s.provider)
	state := byRefState()
	state.Candidates = nil

	_, err := step.Execute(s.ctx, state)

	s.NoError(err)
	s.assertOutcome("none")
}

func (s *SelectTargetMetricSuite) TestFoundAutoSelect() {
	step := NewSelectTargetWithObservability(s.provider)
	state := byRefState()
	state.Candidates = candidates(1)

	_, err := step.Execute(s.ctx, state)

	s.NoError(err)
	s.assertOutcome("found")
}

func (s *SelectTargetMetricSuite) TestMulti() {
	step := NewSelectTargetWithObservability(s.provider)
	state := byRefState()
	state.Candidates = candidates(3)

	_, err := step.Execute(s.ctx, state)

	s.NoError(err)
	s.assertOutcome("multi")
}

func (s *SelectTargetMetricSuite) TestFoundResume() {
	step := NewSelectTargetWithObservability(s.provider)
	state := byRefState()
	state.Candidates = candidates(3)
	state.AwaitingApproval = confirmation.AwaitingSelect
	state.ResumeText = "2"

	_, err := step.Execute(s.ctx, state)

	s.NoError(err)
	s.assertOutcome("found")
}

func (s *SelectTargetMetricSuite) TestReprompt() {
	step := NewSelectTargetWithObservability(s.provider)
	state := byRefState()
	state.Candidates = candidates(3)
	state.AwaitingApproval = confirmation.AwaitingSelect
	state.ResumeText = "9"

	_, err := step.Execute(s.ctx, state)

	s.NoError(err)
	s.assertOutcome("reprompt")
}

func (s *SelectTargetMetricSuite) TestCancel() {
	step := NewSelectTargetWithObservability(s.provider)
	state := byRefState()
	state.Candidates = candidates(3)
	state.AwaitingApproval = confirmation.AwaitingSelect
	state.ResumeText = "abc"
	state.SelectRepromptCount = 1

	_, err := step.Execute(s.ctx, state)

	s.NoError(err)
	s.assertOutcome("cancel")
}

func (s *SelectTargetMetricSuite) TestNilObservability_NoPanic() {
	step := NewSelectTargetWithObservability(nil)
	state := byRefState()
	state.Candidates = candidates(2)

	out, err := step.Execute(s.ctx, state)

	s.NoError(err)
	s.Equal(platform.StepStatusSuspended, out.Status)
}
