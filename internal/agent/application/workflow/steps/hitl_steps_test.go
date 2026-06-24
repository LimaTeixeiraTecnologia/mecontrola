package steps

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/tools"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/confirmation"
	platform "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

type HITLStepsSuite struct {
	suite.Suite
	ctx context.Context
}

func TestHITLStepsSuite(t *testing.T) {
	suite.Run(t, new(HITLStepsSuite))
}

func (s *HITLStepsSuite) SetupTest() {
	s.ctx = context.Background()
}

func baseConfirmState() confirmation.ConfirmState {
	return confirmation.ConfirmState{
		OperationKind: confirmation.OperationDeleteLast,
		UserID:        "550e8400-e29b-41d4-a716-446655440000",
		Channel:       "whatsapp",
		PromptText:    "Confirma?",
	}
}

func (s *HITLStepsSuite) TestConfirmGate_FirstSuspend() {
	step := NewConfirmGate(time.Minute)
	state := baseConfirmState()

	out, err := step.Execute(s.ctx, state)

	s.NoError(err)
	s.Equal(platform.StepStatusSuspended, out.Status)
	s.Equal(confirmation.AwaitingConfirm, out.State.AwaitingApproval)
	s.False(out.State.SuspendedAt.IsZero())
	s.Equal("Confirma?", out.Suspend.Prompt)
}

func (s *HITLStepsSuite) TestConfirmGate_Confirm() {
	step := NewConfirmGate(time.Minute)
	state := baseConfirmState()
	state.AwaitingApproval = confirmation.AwaitingConfirm
	state.ResumeText = "sim"

	out, err := step.Execute(s.ctx, state)

	s.NoError(err)
	s.Equal(platform.StepStatusCompleted, out.Status)
	s.Equal(confirmation.AwaitingNone, out.State.AwaitingApproval)
	s.False(out.State.ShortCircuit)
}

func (s *HITLStepsSuite) TestConfirmGate_Cancel() {
	step := NewConfirmGate(time.Minute)
	state := baseConfirmState()
	state.AwaitingApproval = confirmation.AwaitingConfirm
	state.ResumeText = "não"

	out, err := step.Execute(s.ctx, state)

	s.NoError(err)
	s.Equal(platform.StepStatusCompleted, out.Status)
	s.True(out.State.ShortCircuit)
	s.Equal(confirmCancelledText, out.State.Reply)
	s.Equal(int(tools.OutcomeRouted), out.State.Outcome)
}

func (s *HITLStepsSuite) TestConfirmGate_AmbiguousOnce() {
	step := NewConfirmGate(time.Minute)
	state := baseConfirmState()
	state.AwaitingApproval = confirmation.AwaitingConfirm
	state.ResumeText = "talvez"

	out, err := step.Execute(s.ctx, state)

	s.NoError(err)
	s.Equal(platform.StepStatusSuspended, out.Status)
	s.Equal(1, out.State.RepromptCount)
}

func (s *HITLStepsSuite) TestConfirmGate_AmbiguousTwice_Cancel() {
	step := NewConfirmGate(time.Minute)
	state := baseConfirmState()
	state.AwaitingApproval = confirmation.AwaitingConfirm
	state.RepromptCount = 1
	state.ResumeText = "talvez"

	out, err := step.Execute(s.ctx, state)

	s.NoError(err)
	s.Equal(platform.StepStatusCompleted, out.Status)
	s.True(out.State.ShortCircuit)
	s.Equal(confirmCancelledAmbiguousText, out.State.Reply)
}

func (s *HITLStepsSuite) TestConfirmGate_Expired() {
	step := NewConfirmGate(time.Millisecond)
	state := baseConfirmState()
	state.AwaitingApproval = confirmation.AwaitingConfirm
	state.SuspendedAt = time.Now().UTC().Add(-time.Hour)
	state.ResumeText = "sim"

	out, err := step.Execute(s.ctx, state)

	s.NoError(err)
	s.Equal(platform.StepStatusCompleted, out.Status)
	s.True(out.State.Expired)
	s.True(out.State.ShortCircuit)
	s.Equal(confirmExpiredText, out.State.Reply)
}

func (s *HITLStepsSuite) TestConfirmAuthorize_Allows() {
	step := NewConfirmAuthorize(func(_ context.Context, _ confirmation.ConfirmState) bool { return true }, "negado")
	state := baseConfirmState()

	out, err := step.Execute(s.ctx, state)

	s.NoError(err)
	s.Equal(platform.StepStatusCompleted, out.Status)
	s.False(out.State.ShortCircuit)
}

func (s *HITLStepsSuite) TestConfirmAuthorize_Denies() {
	step := NewConfirmAuthorize(func(_ context.Context, _ confirmation.ConfirmState) bool { return false }, "negado")
	state := baseConfirmState()

	out, err := step.Execute(s.ctx, state)

	s.NoError(err)
	s.Equal(platform.StepStatusCompleted, out.Status)
	s.True(out.State.ShortCircuit)
	s.Equal("negado", out.State.Reply)
	s.Equal(int(tools.OutcomeAuthzDenied), out.State.Outcome)
}

func (s *HITLStepsSuite) TestConfirmReplay_Found() {
	step := NewConfirmReplay(func(_ context.Context, _ confirmation.ConfirmState) (string, bool) {
		return "já processado", true
	})
	state := baseConfirmState()

	out, err := step.Execute(s.ctx, state)

	s.NoError(err)
	s.Equal(platform.StepStatusCompleted, out.Status)
	s.True(out.State.ShortCircuit)
	s.Equal("já processado", out.State.Reply)
	s.Equal(int(tools.OutcomeReplay), out.State.Outcome)
}

func (s *HITLStepsSuite) TestConfirmAuditBegin_PersistsDecisionID() {
	step := NewConfirmAuditBegin(func(_ context.Context, _ confirmation.ConfirmState) ConfirmAuditBeginResult {
		return ConfirmAuditBeginResult{DecisionID: [16]byte{1}}
	}, nil, "replay", "falha")
	state := baseConfirmState()

	out, err := step.Execute(s.ctx, state)

	s.NoError(err)
	s.Equal(platform.StepStatusCompleted, out.Status)
	s.Equal("01000000-0000-0000-0000-000000000000", out.State.DecisionID)
}

func (s *HITLStepsSuite) TestConfirmFormat_Routed() {
	step := NewConfirmFormat(func(_ confirmation.ConfirmState) string { return "ok" })
	state := baseConfirmState()
	state.Outcome = int(tools.OutcomeRouted)

	out, err := step.Execute(s.ctx, state)

	s.NoError(err)
	s.Equal("ok", out.State.Reply)
}
