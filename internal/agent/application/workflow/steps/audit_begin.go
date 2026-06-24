package steps

import (
	"context"

	"github.com/google/uuid"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/tools"
	platform "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

type AuditSettleFunc func(ctx context.Context, executed bool)

type AuditBeginResult struct {
	Conflicted bool
	Failed     bool
	Settle     AuditSettleFunc
	DecisionID uuid.UUID
}

type AuditBeginFunc func(ctx context.Context, state ExpenseState) AuditBeginResult

type OnSettleRegistered func(runID uuid.UUID, settle AuditSettleFunc)

type auditBeginStep struct {
	begin       AuditBeginFunc
	onSettle    OnSettleRegistered
	replayReply string
	failReply   string
}

func NewAuditBegin(begin AuditBeginFunc, onSettle OnSettleRegistered, replayReply, failReply string) platform.Step[ExpenseState] {
	return &auditBeginStep{begin: begin, onSettle: onSettle, replayReply: replayReply, failReply: failReply}
}

func (s *auditBeginStep) ID() string { return "audit_begin" }

func (s *auditBeginStep) Execute(ctx context.Context, state ExpenseState) (platform.StepOutput[ExpenseState], error) {
	if state.IsDone() {
		return platform.StepOutput[ExpenseState]{State: state, Status: platform.StepStatusCompleted}, nil
	}
	result := s.begin(ctx, state)
	if result.Conflicted {
		state.Outcome = tools.OutcomeReplay
		state.Reply = s.replayReply
		state.ShortCircuit = true
		return platform.StepOutput[ExpenseState]{State: state, Status: platform.StepStatusCompleted}, nil
	}
	if result.Failed {
		state.Outcome = tools.OutcomeUsecaseError
		state.Reply = s.failReply
		state.ShortCircuit = true
		return platform.StepOutput[ExpenseState]{State: state, Status: platform.StepStatusCompleted}, nil
	}
	if result.Settle != nil && s.onSettle != nil {
		s.onSettle(result.DecisionID, result.Settle)
	}
	state.DecisionID = result.DecisionID
	return platform.StepOutput[ExpenseState]{State: state, Status: platform.StepStatusCompleted}, nil
}
