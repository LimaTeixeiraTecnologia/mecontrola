package guards

import (
	"context"
	"strings"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
)

var confirmationWithoutToolMarkers = []string{
	"posso registrar?",
	"posso atualizar?",
}

type confirmationWithoutToolGuard struct{}

func NewConfirmationWithoutToolGuard() PostGuard {
	return &confirmationWithoutToolGuard{}
}

func (g *confirmationWithoutToolGuard) Name() string {
	return "confirmation_without_tool"
}

func (g *confirmationWithoutToolGuard) Inspect(_ context.Context, _ agent.Request, out agent.Result) GuardDecision {
	if ExpectedVerbatimText(out.ToolCalls) != "" {
		return GuardDecision{}
	}
	if !containsConfirmationMarker(out.Content) {
		return GuardDecision{}
	}
	if len(out.ToolCalls) > 0 {
		return GuardDecision{}
	}
	forced := out
	forced.Content = successWithoutToolFallbackMessage
	forced.ToolOutcome = agent.ToolOutcomeUsecaseError
	return GuardDecision{Handled: true, Result: forced}
}

func containsConfirmationMarker(content string) bool {
	lower := strings.ToLower(content)
	for _, marker := range confirmationWithoutToolMarkers {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}
