package guards

import (
	"context"
	"strings"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
)

const expiredWithoutToolReask = "Pode me contar de novo o que você quer fazer? Eu continuo a partir daqui. 🙂"

var expiredWithoutToolMarkers = []string{
	"o registro expirou",
}

type expiredWithoutToolGuard struct{}

func NewExpiredWithoutToolGuard() PostGuard {
	return &expiredWithoutToolGuard{}
}

func (g *expiredWithoutToolGuard) Name() string {
	return "expired_without_tool"
}

func (g *expiredWithoutToolGuard) Inspect(_ context.Context, _ agent.Request, out agent.Result) GuardDecision {
	if len(out.ToolCalls) > 0 {
		return GuardDecision{}
	}
	if !containsExpiredWithoutToolMarker(out.Content) {
		return GuardDecision{}
	}
	forced := out
	forced.Content = expiredWithoutToolReask
	forced.ToolOutcome = agent.ToolOutcomeClarify
	return GuardDecision{Handled: true, Result: forced}
}

func containsExpiredWithoutToolMarker(content string) bool {
	lower := strings.ToLower(content)
	for _, marker := range expiredWithoutToolMarkers {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}
