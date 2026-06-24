package steps

import (
	"context"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/tools"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/confirmation"
	platform "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

type ConfirmAuthorizeFunc func(ctx context.Context, state confirmation.ConfirmState) bool

type confirmAuthorizeStep struct {
	authorize ConfirmAuthorizeFunc
	denyReply string
}

func NewConfirmAuthorize(authorize ConfirmAuthorizeFunc, denyReply string) platform.Step[confirmation.ConfirmState] {
	return &confirmAuthorizeStep{authorize: authorize, denyReply: denyReply}
}

func (s *confirmAuthorizeStep) ID() string { return "authorize" }

func (s *confirmAuthorizeStep) Execute(ctx context.Context, state confirmation.ConfirmState) (platform.StepOutput[confirmation.ConfirmState], error) {
	if state.IsDone() {
		return platform.StepOutput[confirmation.ConfirmState]{State: state, Status: platform.StepStatusCompleted}, nil
	}
	if s.authorize(ctx, state) {
		return platform.StepOutput[confirmation.ConfirmState]{State: state, Status: platform.StepStatusCompleted}, nil
	}
	state.Outcome = int(tools.OutcomeAuthzDenied)
	state.Reply = s.denyReply
	state.ShortCircuit = true
	return platform.StepOutput[confirmation.ConfirmState]{State: state, Status: platform.StepStatusCompleted}, nil
}

type ConfirmReplayFunc func(ctx context.Context, state confirmation.ConfirmState) (reply string, found bool)

type confirmReplayStep struct {
	replay ConfirmReplayFunc
}

func NewConfirmReplay(replay ConfirmReplayFunc) platform.Step[confirmation.ConfirmState] {
	return &confirmReplayStep{replay: replay}
}

func (s *confirmReplayStep) ID() string { return "replay" }

func (s *confirmReplayStep) Execute(ctx context.Context, state confirmation.ConfirmState) (platform.StepOutput[confirmation.ConfirmState], error) {
	if state.IsDone() {
		return platform.StepOutput[confirmation.ConfirmState]{State: state, Status: platform.StepStatusCompleted}, nil
	}
	reply, found := s.replay(ctx, state)
	if !found {
		return platform.StepOutput[confirmation.ConfirmState]{State: state, Status: platform.StepStatusCompleted}, nil
	}
	state.Outcome = int(tools.OutcomeReplay)
	state.Reply = reply
	state.ShortCircuit = true
	return platform.StepOutput[confirmation.ConfirmState]{State: state, Status: platform.StepStatusCompleted}, nil
}

type ConfirmPolicyFunc func(ctx context.Context, state confirmation.ConfirmState) (blocked bool, reply string)

type confirmPolicyStep struct {
	policy ConfirmPolicyFunc
}

func NewConfirmPolicy(policy ConfirmPolicyFunc) platform.Step[confirmation.ConfirmState] {
	return &confirmPolicyStep{policy: policy}
}

func (s *confirmPolicyStep) ID() string { return "policy" }

func (s *confirmPolicyStep) Execute(ctx context.Context, state confirmation.ConfirmState) (platform.StepOutput[confirmation.ConfirmState], error) {
	if state.IsDone() {
		return platform.StepOutput[confirmation.ConfirmState]{State: state, Status: platform.StepStatusCompleted}, nil
	}
	blocked, reply := s.policy(ctx, state)
	if !blocked {
		return platform.StepOutput[confirmation.ConfirmState]{State: state, Status: platform.StepStatusCompleted}, nil
	}
	state.Outcome = int(tools.OutcomePolicyBlocked)
	state.Reply = reply
	state.ShortCircuit = true
	return platform.StepOutput[confirmation.ConfirmState]{State: state, Status: platform.StepStatusCompleted}, nil
}

type ConfirmAuditSettleFunc func(ctx context.Context, executed bool)

type ConfirmAuditBeginResult struct {
	Conflicted bool
	Failed     bool
	Settle     ConfirmAuditSettleFunc
	DecisionID uuid.UUID
}

type ConfirmAuditBeginFunc func(ctx context.Context, state confirmation.ConfirmState) ConfirmAuditBeginResult

type ConfirmOnSettleRegistered func(runID uuid.UUID, settle ConfirmAuditSettleFunc)

type confirmAuditBeginStep struct {
	begin       ConfirmAuditBeginFunc
	onSettle    ConfirmOnSettleRegistered
	replayReply string
	failReply   string
}

func NewConfirmAuditBegin(begin ConfirmAuditBeginFunc, onSettle ConfirmOnSettleRegistered, replayReply, failReply string) platform.Step[confirmation.ConfirmState] {
	return &confirmAuditBeginStep{begin: begin, onSettle: onSettle, replayReply: replayReply, failReply: failReply}
}

func (s *confirmAuditBeginStep) ID() string { return "audit_begin" }

func (s *confirmAuditBeginStep) Execute(ctx context.Context, state confirmation.ConfirmState) (platform.StepOutput[confirmation.ConfirmState], error) {
	if state.IsDone() {
		return platform.StepOutput[confirmation.ConfirmState]{State: state, Status: platform.StepStatusCompleted}, nil
	}
	result := s.begin(ctx, state)
	if result.Conflicted {
		state.Outcome = int(tools.OutcomeReplay)
		state.Reply = s.replayReply
		state.ShortCircuit = true
		return platform.StepOutput[confirmation.ConfirmState]{State: state, Status: platform.StepStatusCompleted}, nil
	}
	if result.Failed {
		state.Outcome = int(tools.OutcomeUsecaseError)
		state.Reply = s.failReply
		state.ShortCircuit = true
		return platform.StepOutput[confirmation.ConfirmState]{State: state, Status: platform.StepStatusCompleted}, nil
	}
	if result.Settle != nil && s.onSettle != nil {
		s.onSettle(result.DecisionID, result.Settle)
	}
	state.DecisionID = result.DecisionID.String()
	return platform.StepOutput[confirmation.ConfirmState]{State: state, Status: platform.StepStatusCompleted}, nil
}

type ConfirmFormatFunc func(state confirmation.ConfirmState) string

type confirmFormatStep struct {
	format ConfirmFormatFunc
}

func NewConfirmFormat(format ConfirmFormatFunc) platform.Step[confirmation.ConfirmState] {
	return &confirmFormatStep{format: format}
}

func (s *confirmFormatStep) ID() string { return "format" }

func (s *confirmFormatStep) Execute(_ context.Context, state confirmation.ConfirmState) (platform.StepOutput[confirmation.ConfirmState], error) {
	if state.IsDone() {
		return platform.StepOutput[confirmation.ConfirmState]{State: state, Status: platform.StepStatusCompleted}, nil
	}
	if state.Outcome != int(tools.OutcomeRouted) {
		return platform.StepOutput[confirmation.ConfirmState]{State: state, Status: platform.StepStatusCompleted}, nil
	}
	state.Reply = s.format(state)
	return platform.StepOutput[confirmation.ConfirmState]{State: state, Status: platform.StepStatusCompleted}, nil
}
