package guards

import (
	"context"
	"strings"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
)

type registrationFailureWithoutToolGuard struct{}

func NewRegistrationFailureWithoutToolGuard() PostGuard {
	return &registrationFailureWithoutToolGuard{}
}

func (g *registrationFailureWithoutToolGuard) Name() string {
	return "registration_failure_without_tool"
}

func (g *registrationFailureWithoutToolGuard) Inspect(_ context.Context, _ agent.Request, out agent.Result) GuardDecision {
	if len(out.ToolCalls) > 0 {
		return GuardDecision{}
	}
	if strings.TrimSpace(out.Content) != successWithoutToolFallbackMessage {
		return GuardDecision{}
	}
	if out.ToolOutcome == agent.ToolOutcomeUsecaseError && out.FailureReason != "" {
		return GuardDecision{}
	}
	forced := out
	forced.ToolOutcome = agent.ToolOutcomeUsecaseError
	forced.FailureReason = "guard registration_failure_without_tool: LLM reportou falha de registro sem nenhuma chamada de ferramenta"
	return GuardDecision{Handled: true, Result: forced}
}
