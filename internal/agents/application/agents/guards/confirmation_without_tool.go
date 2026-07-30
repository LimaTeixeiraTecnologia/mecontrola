package guards

import (
	"context"
	"strings"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
)

var confirmationWithoutToolMarkers = []string{
	"posso registrar?",
	"posso atualizar?",
	"você confirma esta operação?",
	"responda *sim* para confirmar",
	"responda apenas sim ou não",
	"responda apenas *sim* ou *não*",
	"operação cancelada conforme solicitado",
	"a operação foi cancelada",
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
	forced := out
	forced.Content = successWithoutToolFallbackMessage
	forced.ToolOutcome = agent.ToolOutcomeUsecaseError
	forced.FailureReason = "guard confirmation_without_tool: pedido de confirmação fabricado pelo LLM sem tool call"
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
