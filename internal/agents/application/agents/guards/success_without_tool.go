package guards

import (
	"context"
	"strings"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/agent"
)

const successWithoutToolFallbackMessage = "Não consegui registrar. Tente novamente em breve."

var successWithoutToolMarkers = []string{
	"registrei",
	"salvei",
	"atualizei",
	"removi",
	"exclu",
	"cadastrei",
	"criei",
}

var successWithoutToolWriteTools = map[string]struct{}{
	"register_expense":  {},
	"register_income":   {},
	"create_recurrence": {},
	"update_recurrence": {},
	"delete_recurrence": {},
	"adjust_allocation": {},
	"edit_entry":        {},
	"delete_entry":      {},
	"update_card":       {},
	"create_card":       {},
	"create_budget":     {},
}

type successWithoutToolGuard struct{}

func NewSuccessWithoutToolGuard() PostGuard {
	return &successWithoutToolGuard{}
}

func (g *successWithoutToolGuard) Name() string {
	return "success_without_tool"
}

func (g *successWithoutToolGuard) Inspect(_ context.Context, _ agent.Request, out agent.Result) GuardDecision {
	if ExpectedVerbatimText(out.ToolCalls) != "" {
		return GuardDecision{}
	}
	if !containsSuccessMarker(out.Content) {
		return GuardDecision{}
	}
	if hasSuccessfulWriteTool(out.ToolCalls) {
		return GuardDecision{}
	}
	forced := out
	forced.Content = successWithoutToolFallbackMessage
	forced.ToolOutcome = agent.ToolOutcomeUsecaseError
	return GuardDecision{Handled: true, Result: forced}
}

func containsSuccessMarker(content string) bool {
	lower := strings.ToLower(content)
	for _, marker := range successWithoutToolMarkers {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func hasSuccessfulWriteTool(calls []agent.ToolCallRecord) bool {
	for _, call := range calls {
		if call.Outcome != agent.ToolCallOutcomeSuccess {
			continue
		}
		if _, ok := successWithoutToolWriteTools[call.Tool]; ok {
			return true
		}
	}
	return false
}
