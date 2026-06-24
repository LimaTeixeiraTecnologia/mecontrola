package steps

import (
	"context"
	"strings"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/tools"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/confirmation"
	platform "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/workflow"
)

const defaultConfirmTTL = 10 * time.Minute

type confirmGateStep struct {
	ttl  time.Duration
	o11y observability.Observability
}

func NewConfirmGate(ttl time.Duration) platform.Step[confirmation.ConfirmState] {
	return NewConfirmGateWithObservability(ttl, nil)
}

func NewConfirmGateWithObservability(ttl time.Duration, o11y observability.Observability) platform.Step[confirmation.ConfirmState] {
	if ttl <= 0 {
		ttl = defaultConfirmTTL
	}
	return &confirmGateStep{ttl: ttl, o11y: o11y}
}

func (s *confirmGateStep) ID() string { return "confirm_gate" }

func (s *confirmGateStep) Execute(ctx context.Context, state confirmation.ConfirmState) (platform.StepOutput[confirmation.ConfirmState], error) {
	if state.IsDone() {
		return platform.StepOutput[confirmation.ConfirmState]{State: state, Status: platform.StepStatusCompleted}, nil
	}

	if state.AwaitingApproval == confirmation.AwaitingNone {
		state.AwaitingApproval = confirmation.AwaitingConfirm
		state.SuspendedAt = time.Now().UTC()
		s.log(ctx, "agent.hitl.suspended",
			observability.String("operation", state.OperationKind.String()),
		)
		return platform.StepOutput[confirmation.ConfirmState]{
			State:  state,
			Status: platform.StepStatusSuspended,
			Suspend: &platform.Suspension{
				Reason: platform.SuspendAwaitingInput,
				Prompt: state.PromptText,
			},
		}, nil
	}

	now := time.Now().UTC()
	if !state.SuspendedAt.IsZero() && now.Sub(state.SuspendedAt) > s.ttl {
		state.Expired = true
		state.ShortCircuit = true
		state.Outcome = int(tools.OutcomeRouted)
		state.Reply = confirmExpiredText
		state.AwaitingApproval = confirmation.AwaitingNone
		s.log(ctx, "agent.hitl.expired",
			observability.String("operation", state.OperationKind.String()),
		)
		return platform.StepOutput[confirmation.ConfirmState]{State: state, Status: platform.StepStatusCompleted}, nil
	}

	resumeText := strings.TrimSpace(state.ResumeText)

	if matchesExpenseConfirmation(resumeText) {
		state.AwaitingApproval = confirmation.AwaitingNone
		s.log(ctx, "agent.hitl.confirmed",
			observability.String("operation", state.OperationKind.String()),
		)
		return platform.StepOutput[confirmation.ConfirmState]{State: state, Status: platform.StepStatusCompleted}, nil
	}

	if matchesExpenseCancellation(resumeText) {
		state.ShortCircuit = true
		state.Outcome = int(tools.OutcomeRouted)
		state.Reply = confirmCancelledText
		state.AwaitingApproval = confirmation.AwaitingNone
		s.log(ctx, "agent.hitl.cancelled",
			observability.String("operation", state.OperationKind.String()),
		)
		return platform.StepOutput[confirmation.ConfirmState]{State: state, Status: platform.StepStatusCompleted}, nil
	}

	if state.RepromptCount >= 1 {
		state.ShortCircuit = true
		state.Outcome = int(tools.OutcomeRouted)
		state.Reply = confirmCancelledAmbiguousText
		state.AwaitingApproval = confirmation.AwaitingNone
		s.log(ctx, "agent.hitl.cancelled",
			observability.String("operation", state.OperationKind.String()),
			observability.String("reason", "ambiguous"),
		)
		return platform.StepOutput[confirmation.ConfirmState]{State: state, Status: platform.StepStatusCompleted}, nil
	}

	state.RepromptCount++
	state.SuspendedAt = time.Now().UTC()
	s.log(ctx, "agent.hitl.reprompt",
		observability.String("operation", state.OperationKind.String()),
	)
	return platform.StepOutput[confirmation.ConfirmState]{
		State:  state,
		Status: platform.StepStatusSuspended,
		Suspend: &platform.Suspension{
			Reason: platform.SuspendAwaitingInput,
			Prompt: state.PromptText,
		},
	}, nil
}

func (s *confirmGateStep) log(ctx context.Context, msg string, attrs ...observability.Field) {
	if s.o11y == nil {
		return
	}
	s.o11y.Logger().Info(ctx, msg, attrs...)
}

const confirmExpiredText = "O tempo de confirmação expirou. A operação foi cancelada."
const confirmCancelledText = "Ok, operação cancelada. Nada foi alterado."
const confirmCancelledAmbiguousText = "Não entendi sua resposta. Operação cancelada por segurança."
