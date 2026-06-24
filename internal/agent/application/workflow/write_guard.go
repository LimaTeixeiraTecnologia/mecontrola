package workflow

import (
	"context"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/tools"
)

type GuardDecision int

const (
	GuardProceed GuardDecision = iota + 1
	GuardShortCircuit
)

type SettleFunc func(ctx context.Context, executed bool)

type GuardSteps struct {
	Authorize func(ctx context.Context, in tools.ToolInput) (tools.ToolResult, bool)
	Replay    func(ctx context.Context, in tools.ToolInput) (tools.ToolResult, bool)
	Policy    func(ctx context.Context, in tools.ToolInput) (tools.ToolResult, bool)
	Audit     func(ctx context.Context, in tools.ToolInput) (tools.ToolResult, SettleFunc, bool)
}

type WriteGuard struct {
	steps GuardSteps
}

func NewWriteGuard(steps GuardSteps) *WriteGuard {
	return &WriteGuard{steps: steps}
}

func (g *WriteGuard) Apply(ctx context.Context, in tools.ToolInput) (GuardDecision, tools.ToolResult, SettleFunc) {
	if g.steps.Authorize != nil {
		if blocked, denied := g.steps.Authorize(ctx, in); denied {
			return GuardShortCircuit, blocked, nil
		}
	}
	if g.steps.Replay != nil {
		if replay, replayed := g.steps.Replay(ctx, in); replayed {
			return GuardShortCircuit, replay, nil
		}
	}
	if g.steps.Policy != nil {
		if blocked, stop := g.steps.Policy(ctx, in); stop {
			return GuardShortCircuit, blocked, nil
		}
	}
	if g.steps.Audit != nil {
		if blocked, settle, stop := g.steps.Audit(ctx, in); stop {
			if settle != nil {
				settle(ctx, false)
			}
			return GuardShortCircuit, blocked, nil
		} else if settle != nil {
			return GuardProceed, tools.ToolResult{}, settle
		}
	}
	return GuardProceed, tools.ToolResult{}, nil
}
